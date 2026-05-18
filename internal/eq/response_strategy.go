package eq

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EQResponseStrategy defines how the agent should modulate its response based
// on detected emotional state, communication style and time of day.
type EQResponseStrategy struct {
	ID             string  `json:"id"`
	DetectedState  string  `json:"detected_state"`   // e.g. "frustrated", "positive", "neutral"
	CommStyle      string  `json:"comm_style"`        // e.g. "formal", "casual", "balanced"
	TimeBucket     string  `json:"time_bucket"`       // e.g. "morning", "afternoon", "evening"
	LengthModifier float64 `json:"length_modifier"`   // multiplier on response length
	FormalityLevel int     `json:"formality_level"`   // 1-5
	OfferHelp      bool    `json:"offer_help"`
	ToneDirective  string  `json:"tone_directive"`    // e.g. "empathetic", "direct", "encouraging"
	CheckInAfter   int     `json:"check_in_after"`    // minutes after which to check in
}

// StrategyResult is the output of applying a strategy to a detected state.
type StrategyResult struct {
	ToneDirective  string  `json:"tone_directive"`
	LengthModifier float64 `json:"length_modifier"`
	FormalityLevel int     `json:"formality_level"`
	OfferHelp      bool    `json:"offer_help"`
	CheckInAfter   int     `json:"check_in_after"`
}

// EQStrategyService manages EQ response strategies.
type EQStrategyService struct {
	mu         sync.Mutex
	strategies []EQResponseStrategy
	now        func() time.Time
}

// NewEQStrategyService creates a new EQStrategyService.
func NewEQStrategyService() *EQStrategyService {
	return &EQStrategyService{
		strategies: []EQResponseStrategy{},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// AddStrategy adds a new response strategy.
func (s *EQStrategyService) AddStrategy(strategy EQResponseStrategy) (EQResponseStrategy, error) {
	if strategy.DetectedState == "" {
		return EQResponseStrategy{}, fmt.Errorf("detected_state is required")
	}
	if strategy.CommStyle == "" {
		return EQResponseStrategy{}, fmt.Errorf("comm_style is required")
	}
	if strategy.LengthModifier <= 0 {
		strategy.LengthModifier = 1.0
	}
	if strategy.FormalityLevel < 1 || strategy.FormalityLevel > 5 {
		return EQResponseStrategy{}, fmt.Errorf("formality_level must be between 1 and 5")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	strategy.ID = uuid.Must(uuid.NewV7()).String()
	s.strategies = append(s.strategies, strategy)
	return strategy, nil
}

// GetStrategy finds the best matching strategy for the given parameters.
func (s *EQStrategyService) GetStrategy(detectedState, commStyle, timeBucket string) (*EQResponseStrategy, error) {
	if detectedState == "" {
		return nil, fmt.Errorf("detected_state is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var best *EQResponseStrategy
	bestScore := -1

	for i := range s.strategies {
		st := &s.strategies[i]
		score := 0
		if st.DetectedState != detectedState {
			continue
		}
		score++
		if st.CommStyle == commStyle {
			score++
		}
		if st.TimeBucket == timeBucket {
			score++
		}
		if score > bestScore {
			bestScore = score
			cp := *st
			best = &cp
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no strategy found for state %q", detectedState)
	}
	return best, nil
}

// ListStrategies returns all registered strategies.
func (s *EQStrategyService) ListStrategies() []EQResponseStrategy {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]EQResponseStrategy, len(s.strategies))
	copy(out, s.strategies)
	return out
}

// ApplyStrategy selects and applies a strategy for the given detected state and
// communication style. It returns a StrategyResult with tone/length modifiers.
func (s *EQStrategyService) ApplyStrategy(detectedState, commStyle string) (*StrategyResult, error) {
	if detectedState == "" {
		return nil, fmt.Errorf("detected_state is required")
	}

	// Determine time bucket from current hour.
	hour := s.now().Hour()
	timeBucket := "morning"
	if hour >= 12 && hour < 17 {
		timeBucket = "afternoon"
	} else if hour >= 17 {
		timeBucket = "evening"
	}

	st, err := s.GetStrategy(detectedState, commStyle, timeBucket)
	if err != nil {
		// Return sensible defaults when no strategy is found.
		return &StrategyResult{
			ToneDirective:  "neutral",
			LengthModifier: 1.0,
			FormalityLevel: 3,
			OfferHelp:      false,
			CheckInAfter:   0,
		}, nil
	}

	return &StrategyResult{
		ToneDirective:  st.ToneDirective,
		LengthModifier: st.LengthModifier,
		FormalityLevel: st.FormalityLevel,
		OfferHelp:      st.OfferHelp,
		CheckInAfter:   st.CheckInAfter,
	}, nil
}
