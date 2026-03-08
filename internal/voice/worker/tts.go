package worker

import (
	"fmt"
	"sync"
)

// TTSRequest holds the parameters for a text-to-speech request.
type TTSRequest struct {
	Text     string
	Voice    string
	Language string
}

// TTSResult holds the result of a text-to-speech synthesis.
type TTSResult struct {
	AudioData  []byte
	DurationMs int64
	Provider   string
}

// TTSProvider is the interface for text-to-speech backends.
type TTSProvider interface {
	Synthesize(req TTSRequest) (*TTSResult, error)
}

// TTSService manages text-to-speech with primary and fallback providers.
type TTSService struct {
	mu                sync.Mutex
	primary           TTSProvider
	fallback          TTSProvider
	consecutiveErrors int
	usingFallback     bool
	errorThreshold    int
}

// NewTTSService creates a new TTSService with failover logic.
func NewTTSService(primary, fallback TTSProvider) *TTSService {
	return &TTSService{
		primary:        primary,
		fallback:       fallback,
		errorThreshold: 3,
	}
}

// Synthesize converts text to speech, switching to fallback after consecutive errors.
func (s *TTSService) Synthesize(req TTSRequest) (*TTSResult, error) {
	if req.Text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}

	s.mu.Lock()
	provider := s.primary
	if s.usingFallback && s.fallback != nil {
		provider = s.fallback
	}
	s.mu.Unlock()

	result, err := provider.Synthesize(req)
	if err != nil {
		s.mu.Lock()
		s.consecutiveErrors++
		if s.consecutiveErrors >= s.errorThreshold && s.fallback != nil && !s.usingFallback {
			s.usingFallback = true
			s.consecutiveErrors = 0
			s.mu.Unlock()

			// Retry with fallback.
			return s.fallback.Synthesize(req)
		}
		s.mu.Unlock()
		return nil, fmt.Errorf("tts synthesize: %w", err)
	}

	s.mu.Lock()
	s.consecutiveErrors = 0
	s.mu.Unlock()

	return result, nil
}

// IsUsingFallback returns whether the service has switched to the fallback provider.
func (s *TTSService) IsUsingFallback() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usingFallback
}

// ResetToPrimary switches back to the primary provider.
func (s *TTSService) ResetToPrimary() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usingFallback = false
	s.consecutiveErrors = 0
}
