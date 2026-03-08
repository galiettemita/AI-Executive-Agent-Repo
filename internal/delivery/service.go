package delivery

import (
	"fmt"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/determinism"
)

// DeliveryRequest represents a message delivery request.
type DeliveryRequest struct {
	ID          string
	WorkspaceID string
	ChannelID   string
	RecipientID string
	Content     string
	Status      string // pending, delivered, failed, retrying
	Attempts    int
	CreatedAt   time.Time
	DeliveredAt *time.Time
}

// DeliveryService manages message delivery with retry logic.
type DeliveryService struct {
	mu         sync.Mutex
	deliveries map[string]*DeliveryRequest
	maxRetries int
}

// NewDeliveryService creates a new DeliveryService.
func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		deliveries: make(map[string]*DeliveryRequest),
		maxRetries: 3,
	}
}

// Send creates a new delivery request.
func (s *DeliveryService) Send(workspaceID, channelID, recipientID, content string) (*DeliveryRequest, error) {
	if content == "" {
		return nil, fmt.Errorf("content must not be empty")
	}
	if recipientID == "" {
		return nil, fmt.Errorf("recipientID must not be empty")
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate delivery id: %w", err)
	}

	dr := &DeliveryRequest{
		ID:          id.String(),
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
		RecipientID: recipientID,
		Content:     content,
		Status:      "pending",
		Attempts:    1,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.deliveries[dr.ID] = dr
	s.mu.Unlock()

	return dr, nil
}

// GetStatus retrieves the delivery status.
func (s *DeliveryService) GetStatus(deliveryID string) (*DeliveryRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dr, ok := s.deliveries[deliveryID]
	if !ok {
		return nil, fmt.Errorf("delivery %s not found", deliveryID)
	}
	return dr, nil
}

// Retry retries a failed delivery.
func (s *DeliveryService) Retry(deliveryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dr, ok := s.deliveries[deliveryID]
	if !ok {
		return fmt.Errorf("delivery %s not found", deliveryID)
	}
	if dr.Status == "delivered" {
		return fmt.Errorf("delivery %s already delivered", deliveryID)
	}
	if dr.Attempts >= s.maxRetries {
		dr.Status = "failed"
		return fmt.Errorf("delivery %s exceeded max retries (%d)", deliveryID, s.maxRetries)
	}

	dr.Attempts++
	dr.Status = "retrying"
	return nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (s *DeliveryService) MarkDelivered(deliveryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dr, ok := s.deliveries[deliveryID]
	if !ok {
		return fmt.Errorf("delivery %s not found", deliveryID)
	}

	now := time.Now()
	dr.Status = "delivered"
	dr.DeliveredAt = &now
	return nil
}

// MarkFailed marks a delivery as failed.
func (s *DeliveryService) MarkFailed(deliveryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dr, ok := s.deliveries[deliveryID]
	if !ok {
		return fmt.Errorf("delivery %s not found", deliveryID)
	}

	dr.Status = "failed"
	return nil
}

// ListPending returns all pending deliveries for a workspace.
func (s *DeliveryService) ListPending(workspaceID string) []DeliveryRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []DeliveryRequest
	for _, dr := range s.deliveries {
		if dr.WorkspaceID == workspaceID && (dr.Status == "pending" || dr.Status == "retrying") {
			result = append(result, *dr)
		}
	}
	return result
}
