package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type DeliveryStatusUpdate struct {
	MessageID string
	Status    string
	UpdatedAt time.Time
}

type WhatsAppClient struct {
	Config         WhatsAppClientConfig
	deliveryStatus map[string]DeliveryStatusUpdate
}

func NewWhatsAppClient(config WhatsAppClientConfig) *WhatsAppClient {
	return &WhatsAppClient{
		Config:         config,
		deliveryStatus: map[string]DeliveryStatusUpdate{},
	}
}

func WhatsAppInboundTypes() []string {
	return []string{"text", "image", "document", "audio", "video", "location", "contacts", "interactive", "reaction"}
}

func WhatsAppOutboundTypes() []string {
	return []string{"text", "image", "document", "audio", "video", "template", "interactive", "reaction"}
}

func RequiresWhatsAppTemplate(within24hSession bool, outboundType string) bool {
	if within24hSession {
		return false
	}
	return strings.ToLower(strings.TrimSpace(outboundType)) != "template"
}

func ValidateWhatsAppWebhookSignature(appSecret []byte, rawBody []byte, signatureHeader string) bool {
	signatureHeader = strings.TrimSpace(strings.ToLower(signatureHeader))
	signatureHeader = strings.TrimPrefix(signatureHeader, "sha256=")
	mac := hmac.New(sha256.New, appSecret)
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signatureHeader), []byte(expected))
}

func WhatsAppSendRetryPolicy(statusCode int, retryAfter time.Duration) (retry bool, delay time.Duration) {
	switch {
	case statusCode == 429:
		if retryAfter > 0 {
			return true, retryAfter
		}
		return true, 1 * time.Second
	case statusCode >= 500:
		return true, 1 * time.Second
	default:
		return false, 0
	}
}

func (c *WhatsAppClient) RecordDeliveryStatus(messageID, status string, updatedAt time.Time) {
	c.deliveryStatus[messageID] = DeliveryStatusUpdate{
		MessageID: messageID,
		Status:    strings.ToLower(strings.TrimSpace(status)),
		UpdatedAt: updatedAt.UTC(),
	}
}

func (c *WhatsAppClient) DeliveryStatus(messageID string) (DeliveryStatusUpdate, bool) {
	status, ok := c.deliveryStatus[messageID]
	return status, ok
}

func (c *WhatsAppClient) ValidateOutbound(outboundType string, within24hSession bool, templateName string) error {
	if RequiresWhatsAppTemplate(within24hSession, outboundType) && strings.TrimSpace(templateName) == "" {
		return fmt.Errorf("whatsapp template required outside 24h session")
	}
	return nil
}
