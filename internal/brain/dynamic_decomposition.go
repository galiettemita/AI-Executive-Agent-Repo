package brain

import (
	"strings"
	"sync"
)

// ComplexitySignals captures features of a request that affect decomposition.
type ComplexitySignals struct {
	WordCount              int  `json:"word_count"`
	EntityCount            int  `json:"entity_count"`
	IntentCount            int  `json:"intent_count"`
	DomainCount            int  `json:"domain_count"`
	HasTemporalConstraints bool `json:"has_temporal_constraints"`
	HasDependencies        bool `json:"has_dependencies"`
}

// DynamicDecompositionService determines task decomposition limits based on
// request complexity.
type DynamicDecompositionService struct {
	mu               sync.Mutex
	baseMaxTasks     int
	baseMaxDepth     int
	entityKeywords   []string
	temporalKeywords []string
	dependencyKeywords []string
}

// NewDynamicDecompositionService creates a new decomposition service.
func NewDynamicDecompositionService() *DynamicDecompositionService {
	return &DynamicDecompositionService{
		baseMaxTasks: 5,
		baseMaxDepth: 3,
		entityKeywords: []string{
			"@", "email", "calendar", "document", "meeting", "project",
			"client", "team", "report", "invoice", "contract",
		},
		temporalKeywords: []string{
			"before", "after", "by", "deadline", "until", "tomorrow",
			"next week", "asap", "within", "schedule",
		},
		dependencyKeywords: []string{
			"then", "after that", "once", "first", "before doing",
			"depends on", "followed by", "next",
		},
	}
}

// EstimateComplexity analyses a request string and returns complexity signals.
func (s *DynamicDecompositionService) EstimateComplexity(request string) ComplexitySignals {
	s.mu.Lock()
	defer s.mu.Unlock()

	lower := strings.ToLower(request)
	words := strings.Fields(request)

	signals := ComplexitySignals{
		WordCount: len(words),
	}

	// Count entities.
	for _, kw := range s.entityKeywords {
		if strings.Contains(lower, kw) {
			signals.EntityCount++
		}
	}

	// Detect temporal constraints.
	for _, kw := range s.temporalKeywords {
		if strings.Contains(lower, kw) {
			signals.HasTemporalConstraints = true
			break
		}
	}

	// Detect dependencies.
	for _, kw := range s.dependencyKeywords {
		if strings.Contains(lower, kw) {
			signals.HasDependencies = true
			break
		}
	}

	// Estimate intent count from sentence structure.
	sentences := splitSentences(request)
	intentVerbs := []string{
		"send", "create", "update", "delete", "find", "search",
		"schedule", "summarize", "analyze", "draft", "review",
		"check", "remind", "book", "cancel", "forward", "reply",
	}
	intentSet := map[string]bool{}
	for _, sentence := range sentences {
		sentLower := strings.ToLower(sentence)
		for _, verb := range intentVerbs {
			if strings.Contains(sentLower, verb) {
				intentSet[verb] = true
			}
		}
	}
	signals.IntentCount = len(intentSet)
	if signals.IntentCount == 0 {
		signals.IntentCount = 1
	}

	// Estimate domain count from entity diversity.
	domainMap := map[string]bool{}
	domainKeywords := map[string]string{
		"email": "communication", "calendar": "scheduling", "meeting": "scheduling",
		"document": "documents", "report": "documents", "invoice": "finance",
		"contract": "legal", "client": "crm", "project": "project_management",
	}
	for _, kw := range s.entityKeywords {
		if strings.Contains(lower, kw) {
			if domain, ok := domainKeywords[kw]; ok {
				domainMap[domain] = true
			}
		}
	}
	signals.DomainCount = len(domainMap)
	if signals.DomainCount == 0 {
		signals.DomainCount = 1
	}

	return signals
}

// splitSentences splits text into sentences on common delimiters.
func splitSentences(text string) []string {
	// Replace common sentence endings with a delimiter.
	for _, sep := range []string{". ", "! ", "? ", "\n"} {
		text = strings.ReplaceAll(text, sep, "|||")
	}
	parts := strings.Split(text, "|||")
	var out []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ComputeLimits determines the max tasks and max depth for decomposition
// based on complexity signals.
func (s *DynamicDecompositionService) ComputeLimits(signals ComplexitySignals) (maxTasks, maxDepth int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	maxTasks = s.baseMaxTasks
	maxDepth = s.baseMaxDepth

	// Scale tasks by intent count.
	if signals.IntentCount > 1 {
		maxTasks += signals.IntentCount - 1
	}

	// Scale by domain diversity.
	if signals.DomainCount > 1 {
		maxTasks += signals.DomainCount - 1
	}

	// Word count scaling.
	if signals.WordCount > 50 {
		maxTasks += 2
	} else if signals.WordCount > 20 {
		maxTasks++
	}

	// Dependencies increase depth.
	if signals.HasDependencies {
		maxDepth++
	}

	// Temporal constraints add depth for scheduling phases.
	if signals.HasTemporalConstraints {
		maxDepth++
	}

	// Cap limits.
	if maxTasks > 20 {
		maxTasks = 20
	}
	if maxDepth > 8 {
		maxDepth = 8
	}

	return maxTasks, maxDepth
}
