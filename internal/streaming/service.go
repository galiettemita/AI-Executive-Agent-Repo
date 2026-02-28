package streaming

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxFirstByteSLAMillis = 500
	minFirstByteSLAMillis = 100
	defaultChunkSizeBytes = 2048
	maxChunkSizeBytes     = 8192
	maxPlansPerWorkspace  = 50
)

type Config struct {
	WorkspaceID           string `json:"workspace_id"`
	AckEnabled            bool   `json:"ack_enabled"`
	TypingIndicator       bool   `json:"typing_indicator"`
	FirstByteSLAMillis    int    `json:"first_byte_sla_ms"`
	ChunkSizeBytes        int    `json:"chunk_size_bytes"`
	ProgressiveDisclosure bool   `json:"progressive_disclosure"`
}

type DeliveryPlan struct {
	WorkspaceID           string    `json:"workspace_id"`
	IngressTurnID         string    `json:"ingress_turn_id"`
	AckEnabled            bool      `json:"ack_enabled"`
	AckMessage            string    `json:"ack_message"`
	TypingIndicator       bool      `json:"typing_indicator"`
	ProgressiveDisclosure bool      `json:"progressive_disclosure"`
	FirstByteMillis       int       `json:"first_byte_ms"`
	FirstByteSLAMillis    int       `json:"first_byte_sla_ms"`
	SLABreached           bool      `json:"sla_breached"`
	Chunks                []string  `json:"chunks"`
	Events                []string  `json:"events"`
	CreatedAt             time.Time `json:"created_at"`
}

type Stats struct {
	WorkspaceID             string `json:"workspace_id"`
	PlansTotal              int    `json:"plans_total"`
	AckSentTotal            int    `json:"ack_sent_total"`
	FirstByteSamples        int    `json:"first_byte_samples"`
	FirstByteSLABreachTotal int    `json:"first_byte_sla_breach_total"`
	LastFirstByteMillis     int    `json:"last_first_byte_ms"`
}

type Service struct {
	mu      sync.RWMutex
	configs map[string]Config
	plans   map[string][]DeliveryPlan
	stats   map[string]Stats
	now     func() time.Time
}

func NewService() *Service {
	return &Service{
		configs: map[string]Config{},
		plans:   map[string][]DeliveryPlan{},
		stats:   map[string]Stats{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) DefaultConfig(workspaceID string) Config {
	if workspaceID == "" {
		workspaceID = "default"
	}
	return Config{
		WorkspaceID:           workspaceID,
		AckEnabled:            true,
		TypingIndicator:       true,
		FirstByteSLAMillis:    maxFirstByteSLAMillis,
		ChunkSizeBytes:        defaultChunkSizeBytes,
		ProgressiveDisclosure: true,
	}
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	cfg, ok := s.configs[workspaceID]
	return cfg, ok
}

func (s *Service) UpsertConfig(workspaceID string, cfg Config) Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	defaults := s.DefaultConfig(workspaceID)
	cfg.WorkspaceID = workspaceID
	if cfg.FirstByteSLAMillis == 0 {
		cfg.FirstByteSLAMillis = defaults.FirstByteSLAMillis
	}
	if cfg.FirstByteSLAMillis < minFirstByteSLAMillis {
		cfg.FirstByteSLAMillis = minFirstByteSLAMillis
	}
	if cfg.FirstByteSLAMillis > maxFirstByteSLAMillis {
		cfg.FirstByteSLAMillis = maxFirstByteSLAMillis
	}
	if cfg.ChunkSizeBytes == 0 {
		cfg.ChunkSizeBytes = defaults.ChunkSizeBytes
	}
	if cfg.ChunkSizeBytes < 1 {
		cfg.ChunkSizeBytes = 1
	}
	if cfg.ChunkSizeBytes > maxChunkSizeBytes {
		cfg.ChunkSizeBytes = maxChunkSizeBytes
	}
	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) PrepareDeliveryPlan(workspaceID, ingressTurnID, payload string, firstByteMillis int) DeliveryPlan {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	cfg, ok := s.configs[workspaceID]
	if !ok {
		cfg = s.DefaultConfig(workspaceID)
		s.configs[workspaceID] = cfg
	}
	if firstByteMillis <= 0 {
		firstByteMillis = 120
	}

	plan := DeliveryPlan{
		WorkspaceID:           workspaceID,
		IngressTurnID:         normalizeIngressTurnID(ingressTurnID),
		AckEnabled:            cfg.AckEnabled,
		TypingIndicator:       cfg.TypingIndicator,
		ProgressiveDisclosure: cfg.ProgressiveDisclosure,
		FirstByteMillis:       firstByteMillis,
		FirstByteSLAMillis:    cfg.FirstByteSLAMillis,
		SLABreached:           firstByteMillis > cfg.FirstByteSLAMillis,
		Chunks:                splitPayload(payload, cfg.ChunkSizeBytes, cfg.ProgressiveDisclosure),
		Events:                []string{},
		CreatedAt:             s.now(),
	}

	if plan.AckEnabled {
		plan.AckMessage = "Processing your request..."
		plan.Events = append(plan.Events, "BREVIO.streaming.ack_sent.v1")
	}
	if plan.TypingIndicator {
		plan.Events = append(plan.Events, "BREVIO.streaming.typing_indicator.v1")
	}
	plan.Events = append(plan.Events, "BREVIO.streaming.first_byte.v1")

	s.plans[workspaceID] = append([]DeliveryPlan{copyPlan(plan)}, s.plans[workspaceID]...)
	if len(s.plans[workspaceID]) > maxPlansPerWorkspace {
		s.plans[workspaceID] = s.plans[workspaceID][:maxPlansPerWorkspace]
	}

	stats := s.stats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.PlansTotal++
	stats.FirstByteSamples++
	stats.LastFirstByteMillis = firstByteMillis
	if plan.AckEnabled {
		stats.AckSentTotal++
	}
	if plan.SLABreached {
		stats.FirstByteSLABreachTotal++
	}
	s.stats[workspaceID] = stats

	return copyPlan(plan)
}

func (s *Service) ListRecentPlans(workspaceID string) []DeliveryPlan {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	plans := s.plans[workspaceID]
	out := make([]DeliveryPlan, len(plans))
	for i := range plans {
		out[i] = copyPlan(plans[i])
	}
	return out
}

func (s *Service) GetStats(workspaceID string) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	stats, ok := s.stats[workspaceID]
	if !ok {
		return Stats{WorkspaceID: workspaceID}
	}
	return stats
}

func splitPayload(payload string, chunkSize int, progressiveDisclosure bool) []string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return []string{""}
	}
	if !progressiveDisclosure || chunkSize <= 0 || len(trimmed) <= chunkSize {
		return []string{trimmed}
	}
	chunks := make([]string, 0, (len(trimmed)/chunkSize)+1)
	for start := 0; start < len(trimmed); start += chunkSize {
		end := start + chunkSize
		if end > len(trimmed) {
			end = len(trimmed)
		}
		chunks = append(chunks, trimmed[start:end])
	}
	return chunks
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeIngressTurnID(ingressTurnID string) string {
	if strings.TrimSpace(ingressTurnID) == "" {
		return "turn_default"
	}
	return ingressTurnID
}

func copyPlan(plan DeliveryPlan) DeliveryPlan {
	out := plan
	out.Chunks = append([]string(nil), plan.Chunks...)
	out.Events = append([]string(nil), plan.Events...)
	sort.Strings(out.Events)
	return out
}
