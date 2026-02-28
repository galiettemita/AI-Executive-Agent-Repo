package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type PromptVersion struct {
	PromptKey        string
	VersionInt       int
	Body             string
	ParentVersionInt int
	ShadowEvalPassed bool
	CreatedAt        time.Time
}

type Request struct {
	WorkspaceID      string
	PromptKey        string
	Input            string
	Tier             string
	ModelID          string
	ProviderID       string
	MaxOutputTokens  int
	Temperature      float64
	TopP             float64
	PresencePenalty  float64
	FrequencyPenalty float64
}

type Response struct {
	RequestHash    string
	PlanJSON       string
	FromReplay     bool
	ProviderID     string
	FailoverReason string
}

type Service struct {
	mu              sync.Mutex
	prompts         map[string][]PromptVersion
	replay          map[string]string
	replayHitCount  int
	activePrompt    map[string]int
	maxTokensByTier map[string]int
	lastFailover    string
}

func NewService() *Service {
	return &Service{
		prompts:      map[string][]PromptVersion{},
		replay:       map[string]string{},
		activePrompt: map[string]int{},
		maxTokensByTier: map[string]int{
			"T0": 256,
			"T1": 512,
			"T2": 1024,
			"T3": 2048,
		},
	}
}

func (s *Service) RegisterPrompt(prompt PromptVersion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.prompts[prompt.PromptKey]
	updated := false
	for i := range existing {
		if existing[i].VersionInt == prompt.VersionInt {
			existing[i] = prompt
			updated = true
			break
		}
	}
	if !updated {
		existing = append(existing, prompt)
	}
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].VersionInt < existing[j].VersionInt
	})
	s.prompts[prompt.PromptKey] = existing
	if _, exists := s.activePrompt[prompt.PromptKey]; !exists {
		s.activePrompt[prompt.PromptKey] = prompt.VersionInt
	}
}

func (s *Service) PromotePrompt(promptKey string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.prompts[promptKey] {
		if v.VersionInt == version {
			if !v.ShadowEvalPassed {
				return fmt.Errorf("prompt version %d failed shadow eval", version)
			}
			s.activePrompt[promptKey] = version
			return nil
		}
	}
	return fmt.Errorf("prompt version not found")
}

func (s *Service) RollbackPrompt(promptKey string, targetVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.prompts[promptKey] {
		if v.VersionInt == targetVersion {
			s.activePrompt[promptKey] = targetVersion
			return nil
		}
	}
	return fmt.Errorf("prompt version not found")
}

func (s *Service) ActivePromptVersion(promptKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activePrompt[promptKey]
}

func normalizeTier(tier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	switch normalized {
	case "T0", "T1", "T2", "T3":
		return normalized
	default:
		return "T1"
	}
}

func requestHash(req Request, activePromptVersion, maxTokens int) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		req.WorkspaceID,
		req.PromptKey,
		fmt.Sprintf("prompt_version=%d", activePromptVersion),
		req.Input,
		normalizeTier(req.Tier),
		req.ModelID,
		req.ProviderID,
		fmt.Sprintf("max_tokens=%d", maxTokens),
		"temperature=0",
		"top_p=1",
		"presence_penalty=0",
		"frequency_penalty=0",
	}, "::")))
	return hex.EncodeToString(h[:])
}

func deterministicPlanJSON(hash string, maxTokens int, providerID string) string {
	payload := map[string]any{
		"request_hash":      hash,
		"temperature":       0,
		"top_p":             1,
		"presence_penalty":  0,
		"frequency_penalty": 0,
		"max_tokens":        maxTokens,
		"provider_id":       providerID,
		"actions":           []string{"analyze", "route", "respond"},
	}
	blob, _ := json.Marshal(payload)
	return string(blob)
}

func (s *Service) resolveMaxTokens(tier string, requested int) int {
	normalizedTier := normalizeTier(tier)
	limit := s.maxTokensByTier[normalizedTier]
	if requested <= 0 {
		return limit
	}
	if requested > limit {
		return limit
	}
	return requested
}

func (s *Service) Generate(req Request) Response {
	return s.GenerateWithFallback(req, "", false, false)
}

func (s *Service) GenerateWithFallback(req Request, fallbackProviderID string, primaryFailed bool, outputCommitted bool) Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeVersion := s.activePrompt[req.PromptKey]
	if activeVersion == 0 {
		activeVersion = 1
	}
	maxTokens := s.resolveMaxTokens(req.Tier, req.MaxOutputTokens)
	hash := requestHash(req, activeVersion, maxTokens)

	providerID := req.ProviderID
	failoverReason := ""
	if primaryFailed && !outputCommitted && strings.TrimSpace(fallbackProviderID) != "" {
		providerID = fallbackProviderID
		failoverReason = "primary_provider_failure_no_output_committed"
		s.lastFailover = failoverReason
	}

	if cached, ok := s.replay[hash]; ok {
		s.replayHitCount++
		return Response{
			RequestHash:    hash,
			PlanJSON:       cached,
			FromReplay:     true,
			ProviderID:     providerID,
			FailoverReason: failoverReason,
		}
	}

	plan := deterministicPlanJSON(hash, maxTokens, providerID)
	s.replay[hash] = plan
	return Response{
		RequestHash:    hash,
		PlanJSON:       plan,
		FromReplay:     false,
		ProviderID:     providerID,
		FailoverReason: failoverReason,
	}
}

func (s *Service) ReplayHitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replayHitCount
}

func (s *Service) LastFailoverReason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastFailover
}
