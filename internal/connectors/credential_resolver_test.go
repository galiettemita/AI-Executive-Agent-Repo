package connectors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractConnectorKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		toolKey string
		want    string
	}{
		{"google_calendar.create_event", "google_calendar"},
		{"gmail.send_email", "gmail"},
		{"ibkr_trading.place_order", "ibkr_trading"},
		{"no_dot_key", "no_dot_key"},
	}
	for _, tt := range tests {
		t.Run(tt.toolKey, func(t *testing.T) {
			got := ExtractConnectorKey(tt.toolKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCredentialResolver_UnknownConnector_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	kp := NewInMemoryKeyProvider("v0", make([]byte, 32))
	svc := NewService(kp)
	resolver := NewCredentialResolver(svc, nil)
	token, err := resolver.ResolveToken(context.Background(), "ws-test", "user-1", "unknown_connector.action")
	require.NoError(t, err)
	assert.Empty(t, token)
}

func TestCredentialResolver_NilRefresher_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	kp := NewInMemoryKeyProvider("v0", make([]byte, 32))
	svc := NewService(kp)
	// Register a connector so it's "found" but refresher is nil.
	svc.LoadSeed(seedFile{
		Connectors: []Connector{{Key: "google_calendar", Domain: "calendar.google.com", RiskLevel: "LOW", DataClass: "internal", MCPServerURL: "http://localhost:8080/google_calendar"}},
		Tools:      []ConnectorTool{{ConnectorKey: "google_calendar", ToolKey: "google_calendar.create_event", AutonomyFloor: "A1"}},
	})
	resolver := NewCredentialResolver(svc, nil)
	token, err := resolver.ResolveToken(context.Background(), "ws-test", "user-1", "google_calendar.create_event")
	require.NoError(t, err)
	assert.Empty(t, token)
}

func TestTokenRefresher_NilService_ReturnsError(t *testing.T) {
	t.Parallel()
	refresher := NewTokenRefresher(nil)
	_, err := refresher.RefreshIfExpired(context.Background(), "ws-test", "user-1", "google_calendar")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestGetConnector_Found(t *testing.T) {
	t.Parallel()
	kp := NewInMemoryKeyProvider("v0", make([]byte, 32))
	svc := NewService(kp)
	svc.LoadSeed(seedFile{
		Connectors: []Connector{{Key: "gmail", Domain: "gmail.google.com", RiskLevel: "LOW", DataClass: "confidential", MCPServerURL: "http://localhost:8080/gmail"}},
		Tools:      []ConnectorTool{{ConnectorKey: "gmail", ToolKey: "gmail.send_email", AutonomyFloor: "A2"}},
	})
	c, ok := svc.GetConnector("gmail")
	assert.True(t, ok)
	assert.Equal(t, "gmail", c.Key)
}

func TestGetConnector_NotFound(t *testing.T) {
	t.Parallel()
	kp := NewInMemoryKeyProvider("v0", make([]byte, 32))
	svc := NewService(kp)
	_, ok := svc.GetConnector("nonexistent")
	assert.False(t, ok)
}
