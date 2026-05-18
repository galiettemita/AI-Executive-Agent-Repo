package brain

import "sync"

// UncertaintyLevel represents the quantified uncertainty of a response.
type UncertaintyLevel struct {
	Score           float64
	Label           string
	ShouldQualify   bool
	QualifierPhrase string
}

// UncertaintyService quantifies confidence into uncertainty levels.
type UncertaintyService struct {
	mu sync.Mutex
}

// NewUncertaintyService creates a new UncertaintyService.
func NewUncertaintyService() *UncertaintyService {
	return &UncertaintyService{}
}

// Quantify maps a confidence score to an UncertaintyLevel.
func (us *UncertaintyService) Quantify(confidence float64) UncertaintyLevel {
	us.mu.Lock()
	defer us.mu.Unlock()

	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	switch {
	case confidence >= 0.9:
		return UncertaintyLevel{
			Score:           confidence,
			Label:           "high_confidence",
			ShouldQualify:   false,
			QualifierPhrase: "",
		}
	case confidence >= 0.7:
		return UncertaintyLevel{
			Score:           confidence,
			Label:           "moderate",
			ShouldQualify:   true,
			QualifierPhrase: "I believe...",
		}
	case confidence >= 0.5:
		return UncertaintyLevel{
			Score:           confidence,
			Label:           "low",
			ShouldQualify:   true,
			QualifierPhrase: "I'm not certain but...",
		}
	default:
		return UncertaintyLevel{
			Score:           confidence,
			Label:           "very_low",
			ShouldQualify:   true,
			QualifierPhrase: "You may want to verify...",
		}
	}
}
