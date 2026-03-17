package a2a

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// M2MToken represents a validated machine-to-machine OAuth token.
type M2MToken struct {
	AgentID   string
	Scopes    []string
	ExpiresAt time.Time
}

// M2MTokenValidator validates inbound OAuth 2.0 M2M bearer tokens.
type M2MTokenValidator interface {
	Validate(ctx context.Context, bearerToken string) (*M2MToken, error)
}

// StaticM2MValidator is a simple validator for development/testing.
type StaticM2MValidator struct {
	validTokens map[string]M2MToken
}

func NewStaticM2MValidator(tokens map[string]M2MToken) *StaticM2MValidator {
	return &StaticM2MValidator{validTokens: tokens}
}

func (v *StaticM2MValidator) Validate(_ context.Context, bearerToken string) (*M2MToken, error) {
	t, ok := v.validTokens[bearerToken]
	if !ok {
		return nil, fmt.Errorf("m2m_auth: invalid token")
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, fmt.Errorf("m2m_auth: token expired")
	}
	return &t, nil
}

// M2MAuthMiddleware validates the Authorization: Bearer <token> header.
func M2MAuthMiddleware(validator M2MTokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			m2m, err := validator.Validate(r.Context(), token)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), agentIDKey{}, m2m.AgentID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type agentIDKey struct{}

// AgentIDFromContext extracts the validated agent ID from the request context.
func AgentIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(agentIDKey{}).(string)
	return id, ok && id != ""
}
