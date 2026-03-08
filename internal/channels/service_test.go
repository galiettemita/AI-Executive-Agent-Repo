package channels

import (
	"strings"
	"testing"
)

func TestRegisterChannel(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()
	ch, err := svc.RegisterChannel("ws1", "email", map[string]string{"smtp_host": "mail.example.com"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ch.ID == "" {
		t.Fatal("expected non-empty channel ID")
	}
	if ch.Type != "email" {
		t.Fatalf("expected email type, got %s", ch.Type)
	}
	if !ch.Enabled {
		t.Fatal("expected channel to be enabled by default")
	}
	if ch.Config["smtp_host"] != "mail.example.com" {
		t.Fatal("expected config to be set")
	}
}

func TestRegisterChannelValidation(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()

	_, err := svc.RegisterChannel("", "email", nil)
	if err == nil {
		t.Fatal("expected error for empty workspaceID")
	}

	_, err = svc.RegisterChannel("ws1", "", nil)
	if err == nil {
		t.Fatal("expected error for empty channel type")
	}
}

func TestGetChannel(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()
	ch, _ := svc.RegisterChannel("ws1", "sms", nil)

	got, err := svc.GetChannel(ch.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != ch.ID {
		t.Fatalf("expected %s, got %s", ch.ID, got.ID)
	}

	_, err = svc.GetChannel("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestEnableDisableChannel(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()
	ch, _ := svc.RegisterChannel("ws1", "slack", nil)

	err := svc.DisableChannel(ch.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, _ := svc.GetChannel(ch.ID)
	if got.Enabled {
		t.Fatal("expected channel to be disabled")
	}

	err = svc.EnableChannel(ch.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, _ = svc.GetChannel(ch.ID)
	if !got.Enabled {
		t.Fatal("expected channel to be enabled")
	}

	err = svc.DisableChannel("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestRouteMessage(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()
	ch, _ := svc.RegisterChannel("ws1", "email", nil)

	resp, err := svc.RouteMessage(ch.ID, "Hello World")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(resp, "email sent") {
		t.Fatalf("expected email sent response, got %s", resp)
	}

	// Disabled channel.
	svc.DisableChannel(ch.ID)
	_, err = svc.RouteMessage(ch.ID, "Hello")
	if err == nil {
		t.Fatal("expected error for disabled channel")
	}

	// Non-existent channel.
	_, err = svc.RouteMessage("nonexistent", "Hello")
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestListChannels(t *testing.T) {
	t.Parallel()

	svc := NewChannelService()
	svc.RegisterChannel("ws1", "email", nil)
	svc.RegisterChannel("ws1", "sms", nil)
	svc.RegisterChannel("ws2", "slack", nil)

	ws1 := svc.ListChannels("ws1")
	if len(ws1) != 2 {
		t.Fatalf("expected 2 channels for ws1, got %d", len(ws1))
	}

	ws2 := svc.ListChannels("ws2")
	if len(ws2) != 1 {
		t.Fatalf("expected 1 channel for ws2, got %d", len(ws2))
	}
}
