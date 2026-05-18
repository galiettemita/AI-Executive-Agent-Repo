package admin

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ToolMTTRLog represents a tool incident with resolution timing.
type ToolMTTRLog struct {
	ID                   string     `json:"id"`
	WorkspaceID          string     `json:"workspace_id"`
	ToolID               string     `json:"tool_id"`
	IncidentStart        time.Time  `json:"incident_start"`
	IncidentEnd          *time.Time `json:"incident_end,omitempty"`
	ResolutionDurationMs int64      `json:"resolution_duration_ms"`
	CreatedAt            time.Time  `json:"created_at"`
}

// ToolMTTRService tracks mean time to recovery for tools.
type ToolMTTRService struct {
	mu        sync.Mutex
	incidents map[string]*ToolMTTRLog // key: ID
	// openIndex maps workspaceID:toolID to the latest open incident ID
	openIndex map[string]string
	now       func() time.Time
}

// NewToolMTTRService creates a new ToolMTTRService.
func NewToolMTTRService() *ToolMTTRService {
	return &ToolMTTRService{
		incidents: map[string]*ToolMTTRLog{},
		openIndex: map[string]string{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func mttrKey(workspaceID, toolID string) string {
	return workspaceID + ":" + toolID
}

// RecordIncident records the start of a tool incident.
func (s *ToolMTTRService) RecordIncident(workspaceID, toolID string) (ToolMTTRLog, error) {
	if workspaceID == "" {
		return ToolMTTRLog{}, fmt.Errorf("workspace_id is required")
	}
	if toolID == "" {
		return ToolMTTRLog{}, fmt.Errorf("tool_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := mttrKey(workspaceID, toolID)
	if existingID, ok := s.openIndex[key]; ok {
		if inc := s.incidents[existingID]; inc != nil && inc.IncidentEnd == nil {
			return ToolMTTRLog{}, fmt.Errorf("tool %s already has an open incident", toolID)
		}
	}

	log := &ToolMTTRLog{
		ID:            uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:   workspaceID,
		ToolID:        toolID,
		IncidentStart: s.now(),
		CreatedAt:     s.now(),
	}
	s.incidents[log.ID] = log
	s.openIndex[key] = log.ID
	return *log, nil
}

// ResolveIncident marks an open incident as resolved.
func (s *ToolMTTRService) ResolveIncident(workspaceID, toolID string) (ToolMTTRLog, error) {
	if workspaceID == "" {
		return ToolMTTRLog{}, fmt.Errorf("workspace_id is required")
	}
	if toolID == "" {
		return ToolMTTRLog{}, fmt.Errorf("tool_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := mttrKey(workspaceID, toolID)
	incID, ok := s.openIndex[key]
	if !ok {
		return ToolMTTRLog{}, fmt.Errorf("no open incident for tool %s", toolID)
	}
	inc := s.incidents[incID]
	if inc.IncidentEnd != nil {
		return ToolMTTRLog{}, fmt.Errorf("incident already resolved for tool %s", toolID)
	}

	now := s.now()
	inc.IncidentEnd = &now
	inc.ResolutionDurationMs = now.Sub(inc.IncidentStart).Milliseconds()
	delete(s.openIndex, key)
	return *inc, nil
}

// GetMTTR returns the mean time to recovery in milliseconds for a tool.
func (s *ToolMTTRService) GetMTTR(workspaceID, toolID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalMs int64
	var count int
	for _, inc := range s.incidents {
		if inc.WorkspaceID == workspaceID && inc.ToolID == toolID && inc.IncidentEnd != nil {
			totalMs += inc.ResolutionDurationMs
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return totalMs / int64(count)
}

// GetToolMTTRHistory returns all incidents for a tool, sorted by start time.
func (s *ToolMTTRService) GetToolMTTRHistory(workspaceID, toolID string) []ToolMTTRLog {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []ToolMTTRLog
	for _, inc := range s.incidents {
		if inc.WorkspaceID == workspaceID && inc.ToolID == toolID {
			result = append(result, *inc)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].IncidentStart.Before(result[j].IncidentStart)
	})
	return result
}
