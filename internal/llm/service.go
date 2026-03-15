package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/cache"
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
	replay          map[string]string // in-memory fallback when Redis is unavailable
	replayHitCount  int
	activePrompt    map[string]int
	maxTokensByTier map[string]int
	lastFailover    string

	// Intelligence layer: real LLM client for provider-backed inference.
	intelligence *IntelligenceService

	// Redis-backed replay cache. When non-nil, takes precedence over the
	// in-memory replay map for cross-instance idempotency.
	redisCache *cache.RedisClient
}

type TierModelSelection struct {
	Tier            string
	PrimaryModel    string
	FallbackModel   string
	MaxOutputTokens int
}

func NewService() *Service {
	return &Service{
		prompts:      map[string][]PromptVersion{},
		replay:       map[string]string{},
		activePrompt: map[string]int{},
		maxTokensByTier: map[string]int{
			"T0": 512,
			"T1": 1024,
			"T2": 4096,
			"T3": 8192,
		},
	}
}

func ResolveTierModel(tier string) TierModelSelection {
	mapping := DefaultTierModelMapping()
	normalized := normalizeTier(tier)
	if selected, ok := mapping[normalized]; ok {
		return selected
	}
	return mapping["T1"]
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

	// Check replay cache: Redis first, then in-memory fallback.
	if s.redisCache != nil {
		if cached, ok, err := s.redisCache.ReplayGet(context.Background(), hash); err == nil && ok {
			s.replayHitCount++
			return Response{
				RequestHash:    hash,
				PlanJSON:       cached,
				FromReplay:     true,
				ProviderID:     providerID,
				FailoverReason: failoverReason,
			}
		} else if err != nil {
			log.Printf("[LLM] Redis replay GET failed, falling back to in-memory: %v", err)
		}
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

	// Store in both Redis and in-memory for durability.
	if s.redisCache != nil {
		if err := s.redisCache.ReplaySet(context.Background(), hash, plan); err != nil {
			log.Printf("[LLM] Redis replay SET failed: %v", err)
		}
	}
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

// SetRedisCache injects a Redis client for cross-instance replay caching.
// When set, the replay cache is checked in Redis before the in-memory map.
func (s *Service) SetRedisCache(rc *cache.RedisClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redisCache = rc
}

// RedisCache returns the configured Redis client, or nil if not set.
func (s *Service) RedisCache() *cache.RedisClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.redisCache
}

// SetIntelligence injects a real IntelligenceService for provider-backed inference.
func (s *Service) SetIntelligence(intel *IntelligenceService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intelligence = intel
}

// Intelligence returns the configured IntelligenceService, or nil if not set.
func (s *Service) Intelligence() *IntelligenceService {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intelligence
}

// ClassifyIntent delegates to the real IntelligenceService for LLM-backed classification.
// Returns an error if intelligence is not configured.
func (s *Service) ClassifyIntent(ctx context.Context, payload, workspaceID string) (*IntentClassification, *Usage, error) {
	intel := s.Intelligence()
	if intel == nil {
		return nil, nil, fmt.Errorf("llm: intelligence service not configured")
	}
	return intel.ClassifyIntent(ctx, payload, workspaceID)
}

// GeneratePlan delegates to the real IntelligenceService for LLM-backed plan generation.
func (s *Service) GeneratePlan(ctx context.Context, intent string, confidence float64, payload, memoryContext, ragContext string) (*GeneratedPlan, *Usage, error) {
	intel := s.Intelligence()
	if intel == nil {
		return nil, nil, fmt.Errorf("llm: intelligence service not configured")
	}
	return intel.GeneratePlan(ctx, intent, confidence, payload, memoryContext, ragContext)
}

// SynthesizeResponse delegates to the real IntelligenceService for LLM-backed synthesis.
func (s *Service) SynthesizeResponse(ctx context.Context, payload, toolResults string) (*SynthesizedResponse, *Usage, error) {
	intel := s.Intelligence()
	if intel == nil {
		return nil, nil, fmt.Errorf("llm: intelligence service not configured")
	}
	return intel.SynthesizeResponse(ctx, payload, toolResults)
}

// StreamSynthesizeResponse implements streaming synthesis, delegating to IntelligenceService.
func (s *Service) StreamSynthesizeResponse(
	ctx context.Context, payload, toolResults string, out chan<- StreamChunk,
) {
	intel := s.Intelligence()
	if intel == nil {
		out <- StreamChunk{Error: fmt.Errorf("llm: intelligence not configured")}
		close(out)
		return
	}
	intel.StreamSynthesizeResponse(ctx, payload, toolResults, out)
}

// VerifyExecution delegates to the real IntelligenceService for LLM-backed verification.
func (s *Service) VerifyExecution(ctx context.Context, input VerifyInput) (*VerifyResult, *Usage, error) {
	intel := s.Intelligence()
	if intel == nil {
		return nil, nil, fmt.Errorf("llm: intelligence service not configured")
	}
	return intel.VerifyExecution(ctx, input)
}

// SummarizeText implements brain.Summarizer. Delegates to IntelligenceService.
func (s *Service) SummarizeText(
	ctx context.Context,
	conversationText string,
	maxOutputTokens int,
) (string, error) {
	intel := s.Intelligence()
	if intel == nil {
		return "", fmt.Errorf("llm: intelligence not configured for summarization")
	}
	return intel.SummarizeText(ctx, conversationText, maxOutputTokens)
}
