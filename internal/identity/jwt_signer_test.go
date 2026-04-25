package identity

import (
	"strings"
	"testing"
	"time"
)

func TestIssueAndVerifyUserJWT(t *testing.T) {
	t.Parallel()

	privateKey, err := GenerateJWTSigningKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := NewJWTSigner(privateKey)
	now := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)

	token, err := signer.IssueUserJWT(UserJWTClaims{
		Sub:         "018f9f6f-763a-7f64-a5d3-b9ff3c0a2be4",
		WorkspaceID: "018f9f6f-763a-7f64-a5d3-b9ff3c0a2be5",
		Role:        "owner",
		Scopes:      []string{"read", "write"},
	}, now)
	if err != nil {
		t.Fatalf("issue user jwt: %v", err)
	}

	claims, err := signer.VerifyUserJWT(token, "https://auth.brevio.app", UserJWTAudience(), now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("verify user jwt: %v", err)
	}
	if claims.Sub == "" || claims.WorkspaceID == "" || claims.Exp <= claims.Iat {
		t.Fatalf("unexpected user claims: %+v", claims)
	}
	if claims.TokenUse != "user_access" || claims.Version != 2 {
		t.Fatalf("unexpected user token typing: %+v", claims)
	}
	if _, err := signer.VerifyUserJWT(token, "https://bad-issuer", UserJWTAudience(), now); err == nil {
		t.Fatal("expected issuer validation failure")
	}
}

func TestIssueAndVerifyAdminJWT(t *testing.T) {
	t.Parallel()

	privateKey, err := GenerateJWTSigningKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := NewJWTSigner(privateKey)
	now := time.Date(2026, 3, 1, 18, 30, 0, 0, time.UTC)

	token, err := signer.IssueAdminJWT(AdminJWTClaims{
		UserJWTClaims: UserJWTClaims{
			Sub:    "018f9f6f-763a-7f64-a5d3-b9ff3c0a2be4",
			Role:   "operator",
			Scopes: []string{"forensics"},
		},
		AdminLevel:  "operator",
		AdminScopes: []string{"forensics", "review_tasks"},
	}, now)
	if err != nil {
		t.Fatalf("issue admin jwt: %v", err)
	}

	claims, err := signer.VerifyAdminJWT(token, "https://auth.brevio.app", AdminJWTAudience(), now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("verify admin jwt: %v", err)
	}
	if claims.AdminLevel != "operator" || len(claims.AdminScopes) == 0 {
		t.Fatalf("unexpected admin claims: %+v", claims)
	}
	if claims.TokenUse != "admin_access" || claims.Version != 2 {
		t.Fatalf("unexpected admin token typing: %+v", claims)
	}
	if _, err := signer.VerifyAdminJWT(token, "https://auth.brevio.app", AdminJWTAudience(), now.Add(16*time.Minute)); err == nil {
		t.Fatal("expected admin token expiry failure")
	}
}

func TestJWKSIncludesRS256Key(t *testing.T) {
	t.Parallel()

	privateKey, err := GenerateJWTSigningKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := NewJWTSigner(privateKey)
	jwks := signer.JWKS()
	keys, ok := jwks["keys"].([]map[string]string)
	if !ok || len(keys) != 1 {
		t.Fatalf("unexpected jwks payload: %+v", jwks)
	}
	if keys[0]["alg"] != "RS256" || keys[0]["kty"] != "RSA" || strings.TrimSpace(keys[0]["kid"]) == "" {
		t.Fatalf("unexpected jwks key: %+v", keys[0])
	}
}
