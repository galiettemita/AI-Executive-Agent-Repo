package worker

import (
	"fmt"
	"sync"
)

// STTResult holds the result of a speech-to-text transcription.
type STTResult struct {
	Text       string
	Confidence float64
	Language   string
	DurationMs int64
}

// STTProvider is the interface for speech-to-text backends.
type STTProvider interface {
	Transcribe(audioData []byte) (*STTResult, error)
}

// STTService manages speech-to-text with primary and fallback providers.
type STTService struct {
	mu                sync.Mutex
	primary           STTProvider
	fallback          STTProvider
	consecutiveErrors int
	usingFallback     bool
	errorThreshold    int
}

// NewSTTService creates a new STTService with failover logic.
func NewSTTService(primary, fallback STTProvider) *STTService {
	return &STTService{
		primary:        primary,
		fallback:       fallback,
		errorThreshold: 3,
	}
}

// Transcribe performs speech-to-text, switching to fallback after consecutive errors.
func (s *STTService) Transcribe(audioData []byte) (*STTResult, error) {
	if len(audioData) == 0 {
		return nil, fmt.Errorf("audio data must not be empty")
	}

	s.mu.Lock()
	provider := s.primary
	if s.usingFallback && s.fallback != nil {
		provider = s.fallback
	}
	s.mu.Unlock()

	result, err := provider.Transcribe(audioData)
	if err != nil {
		s.mu.Lock()
		s.consecutiveErrors++
		if s.consecutiveErrors >= s.errorThreshold && s.fallback != nil && !s.usingFallback {
			s.usingFallback = true
			s.consecutiveErrors = 0
			s.mu.Unlock()

			// Retry with fallback.
			return s.fallback.Transcribe(audioData)
		}
		s.mu.Unlock()
		return nil, fmt.Errorf("stt transcribe: %w", err)
	}

	s.mu.Lock()
	s.consecutiveErrors = 0
	s.mu.Unlock()

	return result, nil
}

// IsUsingFallback returns whether the service has switched to the fallback provider.
func (s *STTService) IsUsingFallback() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usingFallback
}

// ResetToPrimary switches back to the primary provider.
func (s *STTService) ResetToTPrimary() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usingFallback = false
	s.consecutiveErrors = 0
}
