package sessions

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Substitution records a single pronoun/reference resolution.
type Substitution struct {
	Original   string `json:"original"`
	Resolved   string `json:"resolved"`
	EntityType string `json:"entity_type"`
}

// ResolvedUtterance is the result of coreference resolution on user input.
type ResolvedUtterance struct {
	OriginalText  string         `json:"original_text"`
	ResolvedText  string         `json:"resolved_text"`
	Substitutions []Substitution `json:"substitutions"`
	Confidence    float64        `json:"confidence"`
}

// IntentContinuation holds context carried forward from a previous turn.
type IntentContinuation struct {
	PreviousIntent string            `json:"previous_intent"`
	Entities       map[string]string `json:"entities"`
}

// CoreferenceResolver resolves pronouns and follow-up references by looking
// up entities from recent session history.
type CoreferenceResolver struct {
	mu          sync.RWMutex
	sessionSvc  *Service
	continuations map[string]IntentContinuation // sessionID -> last continuation
}

// NewCoreferenceResolver creates a resolver backed by the given session service.
func NewCoreferenceResolver(sessionSvc *Service) *CoreferenceResolver {
	return &CoreferenceResolver{
		sessionSvc:    sessionSvc,
		continuations: make(map[string]IntentContinuation),
	}
}

// pronouns we attempt to resolve.
var pronounMap = map[string][]string{
	"it":    {"object", "topic", "item"},
	"they":  {"person", "group", "team"},
	"them":  {"person", "group", "team"},
	"this":  {"object", "topic", "item"},
	"that":  {"object", "topic", "item"},
	"these": {"object", "topic", "item"},
	"those": {"object", "topic", "item"},
	"he":    {"person"},
	"she":   {"person"},
	"his":   {"person"},
	"her":   {"person"},
}

// followUpPrefixes signal that the user is continuing the prior intent.
var followUpPrefixes = []string{
	"also",
	"and then",
	"what about",
	"how about",
	"same for",
	"do the same",
	"what else",
	"anything else",
	"and also",
	"plus",
	"additionally",
}

// Resolve attempts to replace pronouns in utterance with concrete entity
// values from the session context.
func (r *CoreferenceResolver) Resolve(_ context.Context, sessionID string, utterance string, entities map[string]string) (*ResolvedUtterance, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// Merge current entities with session entities.
	sessionEntities := r.sessionSvc.GetEntities(sessionID)
	entityMap := make(map[string]string)
	for _, e := range sessionEntities {
		entityMap[e.Key] = e.Value
	}
	for k, v := range entities {
		entityMap[k] = v
	}

	result := &ResolvedUtterance{
		OriginalText:  utterance,
		ResolvedText:  utterance,
		Substitutions: []Substitution{},
		Confidence:    1.0,
	}

	words := strings.Fields(utterance)
	resolved := make([]string, len(words))
	copy(resolved, words)

	for i, word := range words {
		lower := strings.ToLower(strings.Trim(word, ".,!?;:"))
		entityTypes, isPronoun := pronounMap[lower]
		if !isPronoun {
			continue
		}

		// Find best matching entity.
		replacement, entityType := findBestEntity(entityMap, entityTypes)
		if replacement == "" {
			continue
		}

		// Preserve punctuation.
		suffix := ""
		if len(word) > 0 {
			last := word[len(word)-1]
			if last == '.' || last == ',' || last == '!' || last == '?' || last == ';' || last == ':' {
				suffix = string(last)
			}
		}

		resolved[i] = replacement + suffix
		result.Substitutions = append(result.Substitutions, Substitution{
			Original:   lower,
			Resolved:   replacement,
			EntityType: entityType,
		})
		result.Confidence -= 0.1 // each substitution reduces confidence slightly
	}

	if len(result.Substitutions) > 0 {
		result.ResolvedText = strings.Join(resolved, " ")
	}
	if result.Confidence < 0.1 {
		result.Confidence = 0.1
	}

	return result, nil
}

// IsFollowUp returns true if the text appears to be a follow-up utterance.
func IsFollowUp(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, prefix := range followUpPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// ResolveFollowUp carries forward the previous intent context and resolves
// the follow-up utterance.
func (r *CoreferenceResolver) ResolveFollowUp(ctx context.Context, sessionID string, text string) (*ResolvedUtterance, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	r.mu.RLock()
	continuation, hasContinuation := r.continuations[sessionID]
	r.mu.RUnlock()

	entities := make(map[string]string)
	if hasContinuation {
		for k, v := range continuation.Entities {
			entities[k] = v
		}
	}

	resolved, err := r.Resolve(ctx, sessionID, text, entities)
	if err != nil {
		return nil, err
	}

	// If this is a follow-up, append context from previous intent.
	if hasContinuation && IsFollowUp(text) {
		resolved.Confidence -= 0.05
		if resolved.Confidence < 0.1 {
			resolved.Confidence = 0.1
		}
	}

	return resolved, nil
}

// SetContinuation stores the intent continuation for a session (called after
// each turn so the next turn can reference prior context).
func (r *CoreferenceResolver) SetContinuation(sessionID string, intent string, entities map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.continuations[sessionID] = IntentContinuation{
		PreviousIntent: intent,
		Entities:       entities,
	}
}

// findBestEntity looks for an entity whose key matches one of the desired
// entity types.
func findBestEntity(entityMap map[string]string, desiredTypes []string) (string, string) {
	// First try exact key match.
	for _, t := range desiredTypes {
		if v, ok := entityMap[t]; ok && v != "" {
			return v, t
		}
	}
	// Fall back to any available entity (prefer shorter keys as they tend to
	// be more specific).
	for k, v := range entityMap {
		if v != "" {
			return v, k
		}
	}
	return "", ""
}
