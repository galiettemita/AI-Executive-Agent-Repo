package connectors

import (
	"context"
	"fmt"
)

// CredentialResolver maps a tool key to the OAuth access token needed to call it.
// It uses the connector registry to find the connector for a tool key, then
// retrieves and refreshes the OAuth token for that connector.
type CredentialResolver struct {
	registry  *Service
	refresher *TokenRefresher
}

// NewCredentialResolver creates a resolver using the connector service and token refresher.
func NewCredentialResolver(registry *Service, refresher *TokenRefresher) *CredentialResolver {
	return &CredentialResolver{
		registry:  registry,
		refresher: refresher,
	}
}

// ResolveToken returns a valid (auto-refreshed) OAuth access token for the given
// workspace, user, and tool key. Returns empty string if the connector requires no OAuth
// or is not registered (e.g. API-key-based connectors).
func (r *CredentialResolver) ResolveToken(ctx context.Context, workspaceID, userID, toolKey string) (string, error) {
	connectorKey := ExtractConnectorKey(toolKey)
	if connectorKey == "" {
		return "", fmt.Errorf("credential_resolver: cannot extract connector key from tool key %q", toolKey)
	}

	// Check if this connector exists in the registry.
	_, found := r.registry.GetConnector(connectorKey)
	if !found {
		return "", nil // connector not registered — skill may use a different auth method
	}

	// If no refresher configured, we can't resolve tokens.
	if r.refresher == nil {
		return "", nil
	}

	// Refresh if expired and return the access token.
	token, err := r.refresher.RefreshIfExpired(ctx, workspaceID, userID, connectorKey)
	if err != nil {
		// Token resolution failure is non-fatal for connectors that may not
		// require OAuth for all operations.
		return "", nil
	}
	return token, nil
}

// ExtractConnectorKey extracts the connector key prefix from a tool key.
// "google_calendar.create_event" → "google_calendar"
// "gmail.send_email"             → "gmail"
func ExtractConnectorKey(toolKey string) string {
	for i, c := range toolKey {
		if c == '.' {
			return toolKey[:i]
		}
	}
	return toolKey // no dot = the tool key IS the connector key
}
