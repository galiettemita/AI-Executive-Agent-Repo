package capture

import (
	"sort"
	"sync"
)

type DailyCapture struct {
	WorkspaceID string `json:"workspace_id"`
	Date        string `json:"date"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
}

type Service struct {
	mu       sync.RWMutex
	captures map[string]map[string]DailyCapture
}

func NewService() *Service {
	return &Service{
		captures: map[string]map[string]DailyCapture{},
	}
}

func (s *Service) Add(capture DailyCapture) DailyCapture {
	s.mu.Lock()
	defer s.mu.Unlock()
	if capture.WorkspaceID == "" {
		capture.WorkspaceID = "default"
	}
	if capture.Status == "" {
		capture.Status = "completed"
	}
	if _, ok := s.captures[capture.WorkspaceID]; !ok {
		s.captures[capture.WorkspaceID] = map[string]DailyCapture{}
	}
	s.captures[capture.WorkspaceID][capture.Date] = capture
	return capture
}

func (s *Service) List(workspaceID string) []DailyCapture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DailyCapture, 0, len(s.captures[workspaceID]))
	for _, capture := range s.captures[workspaceID] {
		out = append(out, capture)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Date < out[j].Date
	})
	return out
}

func (s *Service) Get(workspaceID, date string) (DailyCapture, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	capture, ok := s.captures[workspaceID][date]
	return capture, ok
}
