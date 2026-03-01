package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type IMessageClient struct {
	Config         IMessageClientConfig
	deliveryStatus map[string]DeliveryStatusUpdate
}

func NewIMessageClient(config IMessageClientConfig) *IMessageClient {
	return &IMessageClient{
		Config:         config,
		deliveryStatus: map[string]DeliveryStatusUpdate{},
	}
}

func IMessageInboundTypes() []string {
	return []string{"text", "image", "document", "audio", "richLink", "listPicker"}
}

func IMessageOutboundTypes() []string {
	return []string{"text", "image", "richLink", "listPicker", "timePicker", "quickReply"}
}

func ValidateIMessageWebhookSignature(secret []byte, rawBody []byte, signatureHeader string) bool {
	signatureHeader = strings.TrimSpace(strings.ToLower(signatureHeader))
	mac := hmac.New(sha256.New, secret)
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signatureHeader), []byte(expected))
}

func IMessageSendRetryPolicy(statusCode int) bool {
	return statusCode == 429 || statusCode >= 500
}

func (c *IMessageClient) RecordDeliveryStatus(messageID, status string, updatedAt time.Time) {
	c.deliveryStatus[messageID] = DeliveryStatusUpdate{
		MessageID: messageID,
		Status:    strings.ToLower(strings.TrimSpace(status)),
		UpdatedAt: updatedAt.UTC(),
	}
}

func (c *IMessageClient) DeliveryStatus(messageID string) (DeliveryStatusUpdate, bool) {
	status, ok := c.deliveryStatus[messageID]
	return status, ok
}
