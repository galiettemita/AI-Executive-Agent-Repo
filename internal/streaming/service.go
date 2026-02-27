package streaming

import "sync"

type Config struct {
	WorkspaceID           string `json:"workspace_id"`
	AckEnabled            bool   `json:"ack_enabled"`
	TypingIndicator       bool   `json:"typing_indicator"`
	FirstByteSLAMillis    int    `json:"first_byte_sla_ms"`
	ChunkSizeBytes        int    `json:"chunk_size_bytes"`
	ProgressiveDisclosure bool   `json:"progressive_disclosure"`
}

type Service struct {
	mu      sync.RWMutex
	configs map[string]Config
}

func NewService() *Service {
	return &Service{
		configs: map[string]Config{},
	}
}

func (s *Service) DefaultConfig(workspaceID string) Config {
	return Config{
		WorkspaceID:           workspaceID,
		AckEnabled:            true,
		TypingIndicator:       true,
		FirstByteSLAMillis:    500,
		ChunkSizeBytes:        2048,
		ProgressiveDisclosure: true,
	}
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	if cfg.ChunkSizeBytes == 0 {
		cfg.ChunkSizeBytes = defaults.ChunkSizeBytes
	}
	s.configs[workspaceID] = cfg
	return cfg
}
