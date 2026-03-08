package cognition

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BehaviorSignal represents an observed user behavior signal.
type BehaviorSignal struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	SignalType  string    `json:"signal_type"` // click, dwell, skip, edit, undo, retry
	Context     string    `json:"context"`
	Value       float64   `json:"value"`
	Timestamp   time.Time `json:"timestamp"`
}

// InferredPreference represents a preference inferred from behavior signals.
type InferredPreference struct {
	Context         string   `json:"context"`
	PreferredAction string   `json:"preferred_action"`
	Confidence      float64  `json:"confidence"`
	SignalCount     int      `json:"signal_count"`
	Evidence        []string `json:"evidence"`
}

var validSignalTypes = map[string]struct{}{
	"click": {},
	"dwell": {},
	"skip":  {},
	"edit":  {},
	"undo":  {},
	"retry": {},
}

// signalValence maps signal types to their sentiment valence.
var signalValence = map[string]float64{
	"click": 1.0,  // positive
	"dwell": 0.5,  // interest
	"skip":  -1.0, // negative
	"edit":  -0.3, // correction
	"undo":  -0.8, // rejection
	"retry": -0.5, // dissatisfaction
}

// ImplicitLearningService learns user preferences from behavior signals.
type ImplicitLearningService struct {
	mu      sync.Mutex
	signals []BehaviorSignal
}

// NewImplicitLearningService creates a new ImplicitLearningService.
func NewImplicitLearningService() *ImplicitLearningService {
	return &ImplicitLearningService{
		signals: []BehaviorSignal{},
	}
}

// RecordSignal stores a behavior signal.
func (s *ImplicitLearningService) RecordSignal(signal BehaviorSignal) error {
	if strings.TrimSpace(signal.WorkspaceID) == "" || strings.TrimSpace(signal.UserID) == "" {
		return fmt.Errorf("workspace_id and user_id are required")
	}
	if _, ok := validSignalTypes[signal.SignalType]; !ok {
		return fmt.Errorf("invalid signal type: %s", signal.SignalType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if signal.ID == "" {
		signal.ID = uuid.Must(uuid.NewV7()).String()
	}
	if signal.Timestamp.IsZero() {
		signal.Timestamp = time.Now().UTC()
	}

	s.signals = append(s.signals, signal)
	return nil
}

// InferPreference analyzes signal patterns to infer a preference for a given context.
func (s *ImplicitLearningService) InferPreference(workspaceID, userID, context string) (*InferredPreference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.inferPreferenceLocked(workspaceID, userID, context)
}

func (s *ImplicitLearningService) inferPreferenceLocked(workspaceID, userID, context string) (*InferredPreference, error) {
	contextLower := strings.ToLower(context)
	var relevant []BehaviorSignal
	for _, sig := range s.signals {
		if sig.WorkspaceID == workspaceID && sig.UserID == userID {
			if strings.Contains(strings.ToLower(sig.Context), contextLower) ||
				strings.Contains(contextLower, strings.ToLower(sig.Context)) {
				relevant = append(relevant, sig)
			}
		}
	}

	if len(relevant) == 0 {
		return nil, fmt.Errorf("no signals found for context: %s", context)
	}

	totalValence := 0.0
	var evidence []string
	for _, sig := range relevant {
		totalValence += signalValence[sig.SignalType]
		evidence = append(evidence, fmt.Sprintf("%s:%s", sig.SignalType, sig.Context))
	}

	preferredAction := "continue"
	if totalValence > 0 {
		preferredAction = "repeat"
	} else if totalValence < 0 {
		preferredAction = "change"
	}

	confidence := float64(len(relevant)) / (float64(len(relevant)) + 5.0)

	return &InferredPreference{
		Context:         context,
		PreferredAction: preferredAction,
		Confidence:      confidence,
		SignalCount:     len(relevant),
		Evidence:        evidence,
	}, nil
}

// GetPreferences returns all inferred preferences for a user.
func (s *ImplicitLearningService) GetPreferences(workspaceID, userID string) []InferredPreference {
	s.mu.Lock()
	defer s.mu.Unlock()

	contexts := make(map[string]struct{})
	for _, sig := range s.signals {
		if sig.WorkspaceID == workspaceID && sig.UserID == userID {
			contexts[sig.Context] = struct{}{}
		}
	}

	var prefs []InferredPreference
	for ctx := range contexts {
		p, err := s.inferPreferenceLocked(workspaceID, userID, ctx)
		if err == nil {
			prefs = append(prefs, *p)
		}
	}

	sort.Slice(prefs, func(i, j int) bool {
		return prefs[i].Confidence > prefs[j].Confidence
	})

	return prefs
}
