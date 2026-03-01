package identity

import "time"

type UserJWTClaims struct {
	Sub         string   `json:"sub"`
	Iss         string   `json:"iss"`
	Aud         string   `json:"aud"`
	Iat         int64    `json:"iat"`
	Exp         int64    `json:"exp"`
	WorkspaceID string   `json:"workspace_id"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes"`
}

type AdminJWTClaims struct {
	UserJWTClaims
	AdminLevel  string   `json:"admin_level"`
	AdminScopes []string `json:"admin_scopes"`
}

func UserJWTLifetime() time.Duration {
	return time.Hour
}

func AdminJWTLifetime() time.Duration {
	return 15 * time.Minute
}

func JWKSPath() string {
	return "/.well-known/jwks.json"
}

func JWTSigningAlgorithm() string {
	return "RS256"
}
