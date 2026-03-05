package connectors

import "strings"

type OAuthProviderConfig struct {
	ProviderKey         string
	ProviderURL         string
	DiscoveryURL        string
	AuthorizeURL        string
	RequiredScopes      []string
	TokenType           string
	RefreshStrategy     string
	RequiresPKCES256    bool
	RefreshTokenPolicy  string
	AccessTokenLifetime string
}

func OAuthProviderRegistry() map[string]OAuthProviderConfig {
	return map[string]OAuthProviderConfig{
		"google": {
			ProviderKey:         "google",
			ProviderURL:         "https://accounts.google.com/o/oauth2",
			DiscoveryURL:        "https://accounts.google.com/.well-known/openid-configuration",
			AuthorizeURL:        "https://accounts.google.com/o/oauth2/v2/auth",
			RequiredScopes:      []string{"calendar", "contacts.readonly", "drive", "gmail.modify", "photoslibrary.readonly", "keep", "youtube.readonly"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh_5m_before_expiry",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "access_type=offline prompt=consent",
			AccessTokenLifetime: "1h",
		},
		"spotify": {
			ProviderKey:         "spotify",
			ProviderURL:         "https://accounts.spotify.com/authorize",
			AuthorizeURL:        "https://accounts.spotify.com/authorize",
			RequiredScopes:      []string{"user-modify-playback-state", "user-read-playback-state", "user-top-read", "user-read-recently-played", "playlist-modify-public"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh_on_401",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "1h",
		},
		"microsoft": {
			ProviderKey:         "microsoft",
			ProviderURL:         "https://login.microsoftonline.com/common/oauth2/v2.0",
			DiscoveryURL:        "https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration",
			AuthorizeURL:        "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			RequiredScopes:      []string{"Mail.ReadWrite", "Calendars.ReadWrite", "offline_access"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh_5m_before_expiry",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "offline_access scope",
			AccessTokenLifetime: "1h",
		},
		"todoist": {
			ProviderKey:         "todoist",
			ProviderURL:         "https://todoist.com/oauth/authorize",
			AuthorizeURL:        "https://todoist.com/oauth/authorize",
			RequiredScopes:      []string{"task:add", "data:read", "data:read_write"},
			TokenType:           "Bearer",
			RefreshStrategy:     "reauth_on_expiry",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "no_refresh_token",
			AccessTokenLifetime: "provider_managed",
		},
		"notion": {
			ProviderKey:         "notion",
			ProviderURL:         "https://api.notion.com/v1/oauth",
			AuthorizeURL:        "https://api.notion.com/v1/oauth/authorize",
			RequiredScopes:      []string{"read_content", "update_content", "insert_content"},
			TokenType:           "Bearer",
			RefreshStrategy:     "none_non_expiring",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "no_refresh_token",
			AccessTokenLifetime: "no_expiry",
		},
		"withings": {
			ProviderKey:         "withings",
			ProviderURL:         "https://account.withings.com/oauth2_user/authorize2",
			AuthorizeURL:        "https://account.withings.com/oauth2_user/authorize2",
			RequiredScopes:      []string{"user.metrics", "user.activity"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "provider_managed",
		},
		"dexcom": {
			ProviderKey:         "dexcom",
			ProviderURL:         "https://api.dexcom.com/v2/oauth2/login",
			AuthorizeURL:        "https://api.dexcom.com/v2/oauth2/login",
			RequiredScopes:      []string{"egv:read"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "provider_managed",
		},
		"ynab": {
			ProviderKey:         "ynab",
			ProviderURL:         "https://app.ynab.com/oauth/authorize",
			AuthorizeURL:        "https://app.ynab.com/oauth/authorize",
			RequiredScopes:      []string{"read-only", "read-write"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "1h",
		},
		"monarch": {
			ProviderKey:         "monarch",
			ProviderURL:         "https://api.monarchmoney.com/auth",
			AuthorizeURL:        "https://api.monarchmoney.com/auth",
			RequiredScopes:      []string{"transactions", "accounts"},
			TokenType:           "session_token",
			RefreshStrategy:     "reauth_on_expiry",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "provider_managed",
			AccessTokenLifetime: "provider_managed",
		},
		"plaid": {
			ProviderKey:         "plaid",
			ProviderURL:         "https://plaid.com/link",
			AuthorizeURL:        "https://plaid.com/link",
			RequiredScopes:      []string{"transactions", "balance", "investments", "identity"},
			TokenType:           "access_token",
			RefreshStrategy:     "none_item_based",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "no_refresh_token",
			AccessTokenLifetime: "no_expiry",
		},
		"ticktick": {
			ProviderKey:         "ticktick",
			ProviderURL:         "https://ticktick.com/oauth/authorize",
			AuthorizeURL:        "https://ticktick.com/oauth/authorize",
			RequiredScopes:      []string{"tasks:write", "tasks:read"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "provider_managed",
		},
		"samsung-smartthings": {
			ProviderKey:         "samsung-smartthings",
			ProviderURL:         "https://api.smartthings.com/oauth",
			AuthorizeURL:        "https://api.smartthings.com/oauth/authorize",
			RequiredScopes:      []string{"r:devices:*", "x:devices:*"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "provider_managed",
		},
		"trello": {
			ProviderKey:         "trello",
			ProviderURL:         "https://trello.com/1/authorize",
			AuthorizeURL:        "https://trello.com/1/authorize",
			RequiredScopes:      []string{"read", "write"},
			TokenType:           "api_key_and_token",
			RefreshStrategy:     "none_non_expiring",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "no_refresh_token",
			AccessTokenLifetime: "no_expiry",
		},
		"reddit": {
			ProviderKey:         "reddit",
			ProviderURL:         "https://www.reddit.com/api/v1/authorize",
			AuthorizeURL:        "https://www.reddit.com/api/v1/authorize",
			RequiredScopes:      []string{"read", "submit", "identity"},
			TokenType:           "Bearer + refresh_token",
			RefreshStrategy:     "auto_refresh",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "refresh_token_issued",
			AccessTokenLifetime: "1h",
		},
		"slack": {
			ProviderKey:         "slack",
			ProviderURL:         "https://slack.com/oauth/v2/authorize",
			AuthorizeURL:        "https://slack.com/oauth/v2/authorize",
			RequiredScopes:      []string{"channels:read", "chat:write", "reactions:write", "users:read"},
			TokenType:           "bot_token",
			RefreshStrategy:     "none_non_expiring",
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "no_refresh_token",
			AccessTokenLifetime: "no_expiry",
		},
	}
}

func ConnectorAdditionalOAuthScopes() map[string][]string {
	return map[string][]string{
		"google_calendar":    {"https://www.googleapis.com/auth/calendar"},
		"google_mail":        {"https://www.googleapis.com/auth/gmail.modify"},
		"google_drive":       {"https://www.googleapis.com/auth/drive"},
		"google_contacts":    {"https://www.googleapis.com/auth/contacts"},
		"microsoft_calendar": {"Calendars.ReadWrite"},
		"microsoft_mail":     {"Mail.ReadWrite", "Mail.Send"},
		"microsoft_teams":    {"Chat.ReadWrite", "ChannelMessage.Send"},
		"microsoft_onedrive": {"Files.ReadWrite.All"},
		"slack_messaging":    {"chat:write", "channels:history", "im:history"},
		"todoist":            {"task:add", "data:read_write"},
		"ticktick":           {"tasks:write", "tasks:read"},
		"ynab":               {"read-only", "read-write"},
		"notion":             {"read_content", "update_content", "insert_content"},
	}
}

func OAuthScopesForConnector(providerKey, connectorKey string) []string {
	providers := OAuthProviderRegistry()
	provider, ok := providers[strings.ToLower(strings.TrimSpace(providerKey))]
	if !ok {
		return append([]string(nil), ConnectorAdditionalOAuthScopes()[connectorKey]...)
	}
	base := append([]string(nil), provider.RequiredScopes...)
	return append(base, ConnectorAdditionalOAuthScopes()[connectorKey]...)
}
