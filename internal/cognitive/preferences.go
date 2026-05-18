package cognitive

import (
	"sort"
	"sync"
	"time"
)

// BehaviorSignal represents a single implicit user behavior signal.
type BehaviorSignal struct {
	WorkspaceID string
	UserID      string
	SignalType  string // click, dwell, dismiss, accept, edit, undo
	Context     string
	Value       string
	Timestamp   time.Time
}

// InferredPreference represents a preference inferred from aggregated signals.
type InferredPreference struct {
	WorkspaceID string
	UserID      string
	Category    string
	Preference  string
	Confidence  float64
	SignalCount int
}

// ImplicitPreferenceService learns user preferences from behavioral signals.
type ImplicitPreferenceService struct {
	mu      sync.RWMutex
	signals map[string][]BehaviorSignal // key: workspaceID:userID
}

// NewImplicitPreferenceService creates a new ImplicitPreferenceService.
func NewImplicitPreferenceService() *ImplicitPreferenceService {
	return &ImplicitPreferenceService{
		signals: make(map[string][]BehaviorSignal),
	}
}

// signalWeight maps signal types to their preference weight.
var signalWeight = map[string]float64{
	"click":   0.3,
	"dwell":   0.2,
	"dismiss": -0.4,
	"accept":  0.5,
	"edit":    0.1,
	"undo":    -0.3,
}

// RecordSignal stores a behavioral signal.
func (s *ImplicitPreferenceService) RecordSignal(signal BehaviorSignal) error {
	if signal.Timestamp.IsZero() {
		signal.Timestamp = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := signal.WorkspaceID + ":" + signal.UserID
	s.signals[key] = append(s.signals[key], signal)
	return nil
}

// InferPreferences aggregates signals into inferred preferences for a user.
func (s *ImplicitPreferenceService) InferPreferences(workspaceID, userID string) []InferredPreference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := workspaceID + ":" + userID
	signals := s.signals[key]
	if len(signals) == 0 {
		return nil
	}

	// Group signals by context (category) and value (preference).
	type prefKey struct {
		category   string
		preference string
	}
	agg := make(map[prefKey]*struct {
		weightSum float64
		count     int
	})

	for _, sig := range signals {
		pk := prefKey{category: sig.Context, preference: sig.Value}
		entry, ok := agg[pk]
		if !ok {
			entry = &struct {
				weightSum float64
				count     int
			}{}
			agg[pk] = entry
		}
		w := signalWeight[sig.SignalType]
		entry.weightSum += w
		entry.count++
	}

	var prefs []InferredPreference
	for pk, entry := range agg {
		// Confidence is normalized weight sum.
		confidence := entry.weightSum / float64(entry.count)
		// Normalize to 0-1 range.
		confidence = (confidence + 0.5) / 1.0
		if confidence < 0 {
			confidence = 0
		}
		if confidence > 1 {
			confidence = 1
		}

		prefs = append(prefs, InferredPreference{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Category:    pk.category,
			Preference:  pk.preference,
			Confidence:  confidence,
			SignalCount: entry.count,
		})
	}

	// Sort by confidence descending.
	sort.Slice(prefs, func(i, j int) bool {
		return prefs[i].Confidence > prefs[j].Confidence
	})

	return prefs
}

// GetPreference retrieves the strongest preference for a given category.
func (s *ImplicitPreferenceService) GetPreference(workspaceID, userID, category string) (*InferredPreference, error) {
	prefs := s.InferPreferences(workspaceID, userID)
	for _, p := range prefs {
		if p.Category == category {
			return &p, nil
		}
	}
	return nil, nil
}
