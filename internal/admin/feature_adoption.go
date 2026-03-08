package admin

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FeatureAdoptionEvent records a user's interaction with a feature.
type FeatureAdoptionEvent struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	FeatureID   string    `json:"feature_id"`
	Action      string    `json:"action"` // activated, used, deactivated
	CreatedAt   time.Time `json:"created_at"`
}

// FeatureAdoptionStats holds aggregate adoption stats for a feature.
type FeatureAdoptionStats struct {
	FeatureID   string  `json:"feature_id"`
	TotalUsers  int     `json:"total_users"`
	TotalEvents int     `json:"total_events"`
	AdoptionPct float64 `json:"adoption_pct"`
}

// FeatureAdoptionService tracks feature adoption across workspaces and users.
type FeatureAdoptionService struct {
	mu              sync.Mutex
	events          []FeatureAdoptionEvent
	totalUsers      int // used for adoption percentage calculation
	now             func() time.Time
}

// NewFeatureAdoptionService creates a new FeatureAdoptionService.
func NewFeatureAdoptionService(totalUsers int) *FeatureAdoptionService {
	if totalUsers <= 0 {
		totalUsers = 1
	}
	return &FeatureAdoptionService{
		events:     []FeatureAdoptionEvent{},
		totalUsers: totalUsers,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// TrackAdoption records a feature adoption event.
func (s *FeatureAdoptionService) TrackAdoption(evt FeatureAdoptionEvent) (FeatureAdoptionEvent, error) {
	if evt.WorkspaceID == "" {
		return FeatureAdoptionEvent{}, fmt.Errorf("workspace_id is required")
	}
	if evt.UserID == "" {
		return FeatureAdoptionEvent{}, fmt.Errorf("user_id is required")
	}
	if evt.FeatureID == "" {
		return FeatureAdoptionEvent{}, fmt.Errorf("feature_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	evt.ID = uuid.Must(uuid.NewV7()).String()
	if evt.Action == "" {
		evt.Action = "used"
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = s.now()
	}
	s.events = append(s.events, evt)
	return evt, nil
}

// GetAdoptionStats returns aggregate adoption stats across all features.
func (s *FeatureAdoptionService) GetAdoptionStats() []FeatureAdoptionStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	// featureID -> set of userIDs
	featureUsers := map[string]map[string]struct{}{}
	featureEvents := map[string]int{}

	for _, e := range s.events {
		if featureUsers[e.FeatureID] == nil {
			featureUsers[e.FeatureID] = map[string]struct{}{}
		}
		featureUsers[e.FeatureID][e.UserID] = struct{}{}
		featureEvents[e.FeatureID]++
	}

	var result []FeatureAdoptionStats
	for fid, users := range featureUsers {
		result = append(result, FeatureAdoptionStats{
			FeatureID:   fid,
			TotalUsers:  len(users),
			TotalEvents: featureEvents[fid],
			AdoptionPct: float64(len(users)) / float64(s.totalUsers) * 100,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FeatureID < result[j].FeatureID
	})
	return result
}

// GetFeatureAdoption returns adoption stats for a specific feature.
func (s *FeatureAdoptionService) GetFeatureAdoption(featureID string) FeatureAdoptionStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := map[string]struct{}{}
	eventCount := 0
	for _, e := range s.events {
		if e.FeatureID == featureID {
			users[e.UserID] = struct{}{}
			eventCount++
		}
	}

	return FeatureAdoptionStats{
		FeatureID:   featureID,
		TotalUsers:  len(users),
		TotalEvents: eventCount,
		AdoptionPct: float64(len(users)) / float64(s.totalUsers) * 100,
	}
}
