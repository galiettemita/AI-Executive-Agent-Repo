package cognition

import (
	"sort"
	"sync"
	"time"
)

// WeightedBehaviorSignal represents a single implicit user behavior signal
// with a string-typed value, used by the ImplicitPreferenceService.
type WeightedBehaviorSignal struct {
	WorkspaceID string
	UserID      string
	SignalType  string // click, dwell, dismiss, accept, edit, undo
	Context     string
	Value       string
	Timestamp   time.Time
}

// WeightedPreference represents a preference inferred from aggregated weighted signals.
type WeightedPreference struct {
	WorkspaceID string
	UserID      string
	Category    string
	Preference  string
	Confidence  float64
	SignalCount int
}

// ImplicitPreferenceService learns user preferences from behavioral signals
// using weighted signal aggregation.
type ImplicitPreferenceService struct {
	mu      sync.RWMutex
	signals map[string][]WeightedBehaviorSignal // key: workspaceID:userID
}

// NewImplicitPreferenceService creates a new ImplicitPreferenceService.
func NewImplicitPreferenceService() *ImplicitPreferenceService {
	return &ImplicitPreferenceService{
		signals: make(map[string][]WeightedBehaviorSignal),
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

// RecordWeightedSignal stores a behavioral signal.
func (s *ImplicitPreferenceService) RecordWeightedSignal(signal WeightedBehaviorSignal) error {
	if signal.Timestamp.IsZero() {
		signal.Timestamp = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := signal.WorkspaceID + ":" + signal.UserID
	s.signals[key] = append(s.signals[key], signal)
	return nil
}

// InferWeightedPreferences aggregates signals into inferred preferences for a user.
func (s *ImplicitPreferenceService) InferWeightedPreferences(workspaceID, userID string) []WeightedPreference {
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

	var prefs []WeightedPreference
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

		prefs = append(prefs, WeightedPreference{
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

// GetWeightedPreference retrieves the strongest preference for a given category.
func (s *ImplicitPreferenceService) GetWeightedPreference(workspaceID, userID, category string) (*WeightedPreference, error) {
	prefs := s.InferWeightedPreferences(workspaceID, userID)
	for _, p := range prefs {
		if p.Category == category {
			return &p, nil
		}
	}
	return nil, nil
}
