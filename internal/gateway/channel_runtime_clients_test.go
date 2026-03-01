package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestWhatsAppRuntimeClientHelpers(t *testing.T) {
	t.Parallel()

	if !RequiresWhatsAppTemplate(false, "text") || RequiresWhatsAppTemplate(true, "text") {
		t.Fatal("unexpected template requirement behavior")
	}
	if retry, delay := WhatsAppSendRetryPolicy(429, 5*time.Second); !retry || delay != 5*time.Second {
		t.Fatalf("unexpected 429 retry behavior: retry=%v delay=%s", retry, delay)
	}

	rawBody := []byte(`{"hello":"world"}`)
	secret := []byte("app-secret")
	mac := hmac.New(sha256.New, secret)
	mac.Write(rawBody)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !ValidateWhatsAppWebhookSignature(secret, rawBody, signature) {
		t.Fatal("expected valid whatsapp webhook signature")
	}

	client := NewWhatsAppClient(NewWhatsAppClientConfig("v21.0", "123"))
	if err := client.ValidateOutbound("text", false, ""); err == nil {
		t.Fatal("expected outbound template validation error outside session")
	}
	client.RecordDeliveryStatus("msg_1", "delivered", time.Now().UTC())
	if status, ok := client.DeliveryStatus("msg_1"); !ok || status.Status != "delivered" {
		t.Fatalf("unexpected status lookup: ok=%v status=%+v", ok, status)
	}
}

func TestIMessageRuntimeClientHelpers(t *testing.T) {
	t.Parallel()

	if !IMessageSendRetryPolicy(500) || IMessageSendRetryPolicy(400) {
		t.Fatal("unexpected imessage retry behavior")
	}

	rawBody := []byte(`{"hi":"there"}`)
	secret := []byte("imessage-secret")
	mac := hmac.New(sha256.New, secret)
	mac.Write(rawBody)
	signature := hex.EncodeToString(mac.Sum(nil))
	if !ValidateIMessageWebhookSignature(secret, rawBody, signature) {
		t.Fatal("expected valid imessage webhook signature")
	}

	client := NewIMessageClient(NewIMessageClientConfig("https://msp.example.com/v1", "biz"))
	client.RecordDeliveryStatus("msg_2", "read", time.Now().UTC())
	if status, ok := client.DeliveryStatus("msg_2"); !ok || status.Status != "read" {
		t.Fatalf("unexpected status lookup: ok=%v status=%+v", ok, status)
	}
}
