package compliance

import (
	"fmt"
	"sort"
	"sync"
)

type Framework struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Key         string `json:"key"`
	Status      string `json:"status"`
	VersionInt  int    `json:"version_int"`
}

type Evidence struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	FrameworkID string `json:"framework_id"`
	EventType   string `json:"event_type"`
	ArtifactURI string `json:"artifact_uri"`
	SHA256      string `json:"sha256"`
}

type DSRRequest struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	UserID       string `json:"user_id"`
	RequestType  string `json:"request_type"`
	Status       string `json:"status"`
	DeadlineDate string `json:"deadline_date"`
	CreatedAt    string `json:"created_at"`
}

type Service struct {
	mu         sync.RWMutex
	nextID     int
	frameworks map[string]Framework
	evidence   []Evidence
	dsr        map[string]DSRRequest
}

func NewService() *Service {
	return &Service{
		nextID:     1,
		frameworks: map[string]Framework{},
		evidence:   []Evidence{},
		dsr:        map[string]DSRRequest{},
	}
}

func (s *Service) UpsertFramework(framework Framework) Framework {
	s.mu.Lock()
	defer s.mu.Unlock()
	if framework.ID == "" {
		framework.ID = fmt.Sprintf("framework_%06d", s.nextID)
		s.nextID++
	}
	if framework.WorkspaceID == "" {
		framework.WorkspaceID = "default"
	}
	if framework.Key == "" {
		framework.Key = "soc2"
	}
	if framework.Status == "" {
		framework.Status = "active"
	}
	if framework.VersionInt == 0 {
		framework.VersionInt = 1
	}
	s.frameworks[framework.ID] = framework
	return framework
}

func (s *Service) ListFrameworks(workspaceID string) []Framework {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Framework, 0, len(s.frameworks))
	for _, framework := range s.frameworks {
		if workspaceID != "" && framework.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, framework)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) AddEvidence(evidence Evidence) Evidence {
	s.mu.Lock()
	defer s.mu.Unlock()
	evidence.ID = fmt.Sprintf("evidence_%06d", s.nextID)
	s.nextID++
	if evidence.WorkspaceID == "" {
		evidence.WorkspaceID = "default"
	}
	if evidence.EventType == "" {
		evidence.EventType = "BREVIO.compliance.evidence_collected.v1"
	}
	if evidence.SHA256 == "" {
		evidence.SHA256 = "sha256:placeholder"
	}
	s.evidence = append(s.evidence, evidence)
	return evidence
}

func (s *Service) ListEvidence(workspaceID string) []Evidence {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Evidence, 0, len(s.evidence))
	for _, evidence := range s.evidence {
		if workspaceID != "" && evidence.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, evidence)
	}
	return out
}

func (s *Service) CreateDSR(request DSRRequest) DSRRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	request.ID = fmt.Sprintf("dsr_%06d", s.nextID)
	s.nextID++
	if request.WorkspaceID == "" {
		request.WorkspaceID = "default"
	}
	if request.RequestType == "" {
		request.RequestType = "access"
	}
	if request.Status == "" {
		request.Status = "received"
	}
	if request.DeadlineDate == "" {
		request.DeadlineDate = "2026-03-31"
	}
	request.CreatedAt = fmt.Sprintf("2026-02-27T00:%02d:00Z", s.nextID%60)
	s.dsr[request.ID] = request
	return request
}

func (s *Service) GetDSR(id string) (DSRRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	request, ok := s.dsr[id]
	return request, ok
}

func (s *Service) UpdateDSR(id string, update DSRRequest) (DSRRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.dsr[id]
	if !ok {
		return DSRRequest{}, false
	}
	if update.Status != "" {
		current.Status = update.Status
	}
	if update.DeadlineDate != "" {
		current.DeadlineDate = update.DeadlineDate
	}
	if update.RequestType != "" {
		current.RequestType = update.RequestType
	}
	s.dsr[id] = current
	return current, true
}

func (s *Service) ListDSR(workspaceID string) []DSRRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DSRRequest, 0, len(s.dsr))
	for _, request := range s.dsr {
		if workspaceID != "" && request.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, request)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}
