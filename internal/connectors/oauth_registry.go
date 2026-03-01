package connectors

import "strings"

type OAuthProviderConfig struct {
	ProviderKey         string
	DiscoveryURL        string
	AuthorizeURL        string
	RequiredScopes      []string
	RequiresPKCES256    bool
	RefreshTokenPolicy  string
	AccessTokenLifetime string
}

func OAuthProviderRegistry() map[string]OAuthProviderConfig {
	return map[string]OAuthProviderConfig{
		"google": {
			ProviderKey:         "google",
			DiscoveryURL:        "https://accounts.google.com/.well-known/openid-configuration",
			RequiredScopes:      []string{"openid", "email", "profile"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "access_type=offline prompt=consent",
			AccessTokenLifetime: "1h",
		},
		"microsoft": {
			ProviderKey:         "microsoft",
			DiscoveryURL:        "https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration",
			RequiredScopes:      []string{"openid", "email", "profile", "offline_access"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "offline_access scope",
			AccessTokenLifetime: "1h",
		},
		"apple": {
			ProviderKey:         "apple",
			DiscoveryURL:        "https://appleid.apple.com/.well-known/openid-configuration",
			RequiredScopes:      []string{"openid", "email", "name"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "first auth",
			AccessTokenLifetime: "10m",
		},
		"slack": {
			ProviderKey:         "slack",
			AuthorizeURL:        "https://slack.com/oauth/v2/authorize",
			RequiredScopes:      []string{"users:read", "channels:read"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "revocation_only",
			AccessTokenLifetime: "no_expiry",
		},
		"zoom": {
			ProviderKey:         "zoom",
			AuthorizeURL:        "https://zoom.us/oauth/authorize",
			RequiredScopes:      []string{"meeting:read", "meeting:write"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "default_issued",
			AccessTokenLifetime: "1h",
		},
		"github": {
			ProviderKey:         "github",
			AuthorizeURL:        "https://github.com/login/oauth/authorize",
			RequiredScopes:      []string{"repo", "read:user"},
			RequiresPKCES256:    true,
			RefreshTokenPolicy:  "not_issued",
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
		"github_repos":       {"repo", "read:org"},
		"zoom_meetings":      {"meeting:read:admin", "meeting:write:admin"},
	}
}

func OAuthScopesForConnector(providerKey, connectorKey string) []string {
	providers := OAuthProviderRegistry()
	base := append([]string(nil), providers[strings.ToLower(strings.TrimSpace(providerKey))].RequiredScopes...)
	return append(base, ConnectorAdditionalOAuthScopes()[connectorKey]...)
}
