package channels

import (
	"fmt"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/determinism"
)

// Channel represents a communication channel (email, SMS, Slack, etc.).
type Channel struct {
	ID          string
	WorkspaceID string
	Type        string // email, sms, slack, webhook, voice
	Config      map[string]string
	Enabled     bool
	CreatedAt   time.Time
}

// ChannelService manages communication channels.
type ChannelService struct {
	mu       sync.Mutex
	channels map[string]*Channel
}

// NewChannelService creates a new ChannelService.
func NewChannelService() *ChannelService {
	return &ChannelService{
		channels: make(map[string]*Channel),
	}
}

// RegisterChannel creates a new channel.
func (s *ChannelService) RegisterChannel(workspaceID, channelType string, config map[string]string) (*Channel, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID must not be empty")
	}
	if channelType == "" {
		return nil, fmt.Errorf("channel type must not be empty")
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate channel id: %w", err)
	}

	if config == nil {
		config = make(map[string]string)
	}

	ch := &Channel{
		ID:          id.String(),
		WorkspaceID: workspaceID,
		Type:        channelType,
		Config:      config,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.channels[ch.ID] = ch
	s.mu.Unlock()

	return ch, nil
}

// GetChannel retrieves a channel by ID.
func (s *ChannelService) GetChannel(channelID string) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.channels[channelID]
	if !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	return ch, nil
}

// ListChannels returns all channels for a workspace.
func (s *ChannelService) ListChannels(workspaceID string) []Channel {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Channel
	for _, ch := range s.channels {
		if ch.WorkspaceID == workspaceID {
			result = append(result, *ch)
		}
	}
	return result
}

// EnableChannel enables a channel.
func (s *ChannelService) EnableChannel(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.channels[channelID]
	if !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}
	ch.Enabled = true
	return nil
}

// DisableChannel disables a channel.
func (s *ChannelService) DisableChannel(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.channels[channelID]
	if !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}
	ch.Enabled = false
	return nil
}

// RouteMessage sends a message through a specific channel.
func (s *ChannelService) RouteMessage(channelID, message string) (string, error) {
	s.mu.Lock()
	ch, ok := s.channels[channelID]
	if !ok {
		s.mu.Unlock()
		return "", fmt.Errorf("channel %s not found", channelID)
	}
	if !ch.Enabled {
		s.mu.Unlock()
		return "", fmt.Errorf("channel %s is disabled", channelID)
	}
	channelType := ch.Type
	s.mu.Unlock()

	// Simulate routing based on channel type.
	switch channelType {
	case "email":
		return fmt.Sprintf("email sent: %s", message), nil
	case "sms":
		return fmt.Sprintf("sms sent: %s", message), nil
	case "slack":
		return fmt.Sprintf("slack message sent: %s", message), nil
	case "webhook":
		return fmt.Sprintf("webhook delivered: %s", message), nil
	default:
		return fmt.Sprintf("message routed via %s: %s", channelType, message), nil
	}
}
