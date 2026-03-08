package eq

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CommunicationProfile defines the communication style preferences for a workspace.
type CommunicationProfile struct {
	WorkspaceID string  `json:"workspace_id"`
	Formality   string  `json:"formality"`  // casual, balanced, formal
	Verbosity   string  `json:"verbosity"`  // concise, balanced, detailed
	EmojiUse    bool    `json:"emoji_use"`
	Humor       bool    `json:"humor"`
	Directness  float64 `json:"directness"` // 0.0 to 1.0
}

// EmotionalState captures detected emotion at a point in time.
type EmotionalState struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	Valence         float64   `json:"valence"`          // -1 to 1
	Arousal         float64   `json:"arousal"`           // 0 to 1
	DetectedEmotion string    `json:"detected_emotion"`
	Confidence      float64   `json:"confidence"`
	Timestamp       time.Time `json:"timestamp"`
}

// EQService provides emotional intelligence and behavioral calibration.
type EQService struct {
	mu       sync.Mutex
	profiles map[string]CommunicationProfile
	history  map[string][]EmotionalState
	now      func() time.Time
}

// NewEQService creates a new EQService.
func NewEQService() *EQService {
	return &EQService{
		profiles: map[string]CommunicationProfile{},
		history:  map[string][]EmotionalState{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// DefaultProfile returns a balanced default communication profile.
func DefaultProfile() CommunicationProfile {
	return CommunicationProfile{
		Formality:  "balanced",
		Verbosity:  "balanced",
		EmojiUse:   false,
		Humor:      false,
		Directness: 0.5,
	}
}

// SetProfile sets the communication profile for a workspace.
func (s *EQService) SetProfile(workspaceID string, profile CommunicationProfile) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}

	valid := map[string]bool{"casual": true, "balanced": true, "formal": true}
	if !valid[profile.Formality] {
		return fmt.Errorf("invalid formality: %s", profile.Formality)
	}
	validV := map[string]bool{"concise": true, "balanced": true, "detailed": true}
	if !validV[profile.Verbosity] {
		return fmt.Errorf("invalid verbosity: %s", profile.Verbosity)
	}
	if profile.Directness < 0 || profile.Directness > 1 {
		return fmt.Errorf("directness must be between 0 and 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	profile.WorkspaceID = workspaceID
	s.profiles[workspaceID] = profile
	return nil
}

// GetProfile returns the communication profile for a workspace.
func (s *EQService) GetProfile(workspaceID string) (*CommunicationProfile, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.profiles[workspaceID]
	if !ok {
		return nil, fmt.Errorf("profile not found for workspace: %s", workspaceID)
	}
	return &p, nil
}

// DetectEmotion performs keyword-based sentiment detection on text.
func (s *EQService) DetectEmotion(text string) (*EmotionalState, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text is required")
	}

	lower := strings.ToLower(text)
	emotion := "neutral"
	valence := 0.0
	arousal := 0.3
	confidence := 0.6

	positiveWords := []string{"happy", "great", "excellent", "love", "amazing", "wonderful", "fantastic", "good", "thank"}
	negativeWords := []string{"angry", "sad", "frustrated", "upset", "terrible", "awful", "hate", "bad", "annoyed"}
	urgentWords := []string{"urgent", "asap", "immediately", "critical", "emergency", "help"}

	posCount := 0
	negCount := 0
	urgCount := 0

	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			posCount++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			negCount++
		}
	}
	for _, w := range urgentWords {
		if strings.Contains(lower, w) {
			urgCount++
		}
	}

	if posCount > negCount {
		emotion = "positive"
		valence = 0.7
		arousal = 0.5
		confidence = 0.75
	} else if negCount > posCount {
		emotion = "negative"
		valence = -0.7
		arousal = 0.6
		confidence = 0.75
	}

	if urgCount > 0 {
		arousal = 0.9
		if emotion == "neutral" {
			emotion = "urgent"
		}
		confidence = 0.8
	}

	state := &EmotionalState{
		ID:              uuid.Must(uuid.NewV7()).String(),
		Valence:         valence,
		Arousal:         arousal,
		DetectedEmotion: emotion,
		Confidence:      confidence,
		Timestamp:       s.now(),
	}
	return state, nil
}

// LogEmotionalState records an emotional state in history.
func (s *EQService) LogEmotionalState(state EmotionalState) error {
	if state.WorkspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if state.ID == "" {
		state.ID = uuid.Must(uuid.NewV7()).String()
	}
	if state.Timestamp.IsZero() {
		state.Timestamp = s.now()
	}
	s.history[state.WorkspaceID] = append(s.history[state.WorkspaceID], state)
	return nil
}

// GetEmotionalHistory returns the most recent emotional states for a workspace.
func (s *EQService) GetEmotionalHistory(workspaceID string, limit int) []EmotionalState {
	s.mu.Lock()
	defer s.mu.Unlock()

	h := s.history[workspaceID]
	if limit <= 0 || limit > len(h) {
		limit = len(h)
	}

	start := len(h) - limit
	out := make([]EmotionalState, limit)
	copy(out, h[start:])
	return out
}

// AdaptResponse adjusts a response based on the workspace communication profile.
func (s *EQService) AdaptResponse(workspaceID, response string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}
	if strings.TrimSpace(response) == "" {
		return "", fmt.Errorf("response is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profiles[workspaceID]
	if !ok {
		return response, nil // return unchanged if no profile
	}

	adapted := response

	switch profile.Formality {
	case "formal":
		adapted = ensureFormalClosing(adapted)
	case "casual":
		adapted = ensureCasualTone(adapted)
	}

	switch profile.Verbosity {
	case "concise":
		if len(adapted) > 200 {
			adapted = adapted[:200] + "..."
		}
	case "detailed":
		adapted = adapted + "\n\nPlease let me know if you need further details."
	}

	return adapted, nil
}

func ensureFormalClosing(s string) string {
	if !strings.HasSuffix(strings.TrimSpace(s), ".") {
		s = strings.TrimSpace(s) + "."
	}
	return s
}

func ensureCasualTone(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "Dear") {
		s = strings.Replace(s, "Dear", "Hey", 1)
	}
	return s
}
