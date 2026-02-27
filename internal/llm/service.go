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
	WorkspaceID     string
	PromptKey       string
	Input           string
	Tier            string
	ModelID         string
	ProviderID      string
	MaxOutputTokens int
}

type Response struct {
	RequestHash string
	PlanJSON    string
	FromReplay  bool
}

type Service struct {
	mu              sync.Mutex
	prompts         map[string][]PromptVersion
	replay          map[string]string
	replayHitCount  int
	activePrompt    map[string]int
	maxTokensByTier map[string]int
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
	s.prompts[prompt.PromptKey] = append(s.prompts[prompt.PromptKey], prompt)
	sort.Slice(s.prompts[prompt.PromptKey], func(i, j int) bool {
		return s.prompts[prompt.PromptKey][i].VersionInt < s.prompts[prompt.PromptKey][j].VersionInt
	})
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

func requestHash(req Request) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		req.WorkspaceID,
		req.PromptKey,
		req.Input,
		req.Tier,
		req.ModelID,
		req.ProviderID,
	}, "::")))
	return hex.EncodeToString(h[:])
}

func deterministicPlanJSON(hash string, maxTokens int) string {
	payload := map[string]any{
		"request_hash": hash,
		"temperature":  0,
		"top_p":        1,
		"max_tokens":   maxTokens,
		"actions":      []string{"analyze", "route", "respond"},
	}
	blob, _ := json.Marshal(payload)
	return string(blob)
}

func (s *Service) Generate(req Request) Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := requestHash(req)
	if cached, ok := s.replay[hash]; ok {
		s.replayHitCount++
		return Response{RequestHash: hash, PlanJSON: cached, FromReplay: true}
	}

	maxTokens := req.MaxOutputTokens
	if maxTokens == 0 {
		maxTokens = s.maxTokensByTier[req.Tier]
	}
	plan := deterministicPlanJSON(hash, maxTokens)
	s.replay[hash] = plan
	return Response{RequestHash: hash, PlanJSON: plan, FromReplay: false}
}

func (s *Service) ReplayHitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replayHitCount
}
