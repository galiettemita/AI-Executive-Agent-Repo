package call

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// WebhookEvent represents an incoming VAPI webhook event.
type WebhookEvent struct {
	EventType string         `json:"type"`
	CallID    string         `json:"call_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// WebhookHandler validates and processes VAPI webhook events.
type WebhookHandler struct {
	hmacSecret  string
	callService *CallService
}

// NewWebhookHandler creates a webhook handler with the given HMAC secret.
func NewWebhookHandler(hmacSecret string, callService *CallService) *WebhookHandler {
	return &WebhookHandler{
		hmacSecret:  hmacSecret,
		callService: callService,
	}
}

// HandleWebhook validates the HMAC-SHA256 signature and processes the event.
func (wh *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Vapi-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Webhook-Signature")
	}

	if !wh.ValidateSignature(body, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	switch event.EventType {
	case "call.started":
		// Call has started ringing or is in progress.
		if wh.callService != nil {
			wh.callService.handleCallStarted(event.CallID)
		}
	case "call.ended":
		transcript, _ := event.Data["transcript"].(string)
		if wh.callService != nil {
			_ = wh.callService.HandleCallCompleted(event.CallID, transcript)
		}
	case "transcript.update":
		// Partial transcript update; stored for real-time streaming.
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// ValidateSignature verifies the HMAC-SHA256 signature of the webhook body.
func (wh *WebhookHandler) ValidateSignature(body []byte, signature string) bool {
	if wh.hmacSecret == "" || signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(wh.hmacSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
