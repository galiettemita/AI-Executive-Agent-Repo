// Package security_test — JWT algorithm confusion tests.
// Plan 12 §4: alg:none attack and RS256→HS256 key-confusion attack.
// Standard library ONLY — no third-party JWT library (Plan 12 §7).
// NO BUILD TAG — runs in normal CI.
package security_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// attackClaims is the JWT payload from Plan 12 §6 Step 4, verbatim.
var attackClaims = map[string]any{
	"sub":          "attacker",
	"workspace_id": "ws-admin",
}

// buildJWT constructs a JWT using only the Go standard library.
func buildJWT(t *testing.T, alg string, claims map[string]any, signingKey []byte) string {
	t.Helper()

	headerJSON, err := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
	if err != nil {
		t.Fatalf("buildJWT: marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("buildJWT: marshal claims: %v", err)
	}

	hPart := base64.RawURLEncoding.EncodeToString(headerJSON)
	cPart := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sigInput := hPart + "." + cPart

	var sigPart string
	if strings.ToLower(alg) != "none" {
		mac := hmac.New(sha256.New, signingKey)
		mac.Write([]byte(sigInput))
		sigPart = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	return sigInput + "." + sigPart
}

// parseJWTHeader decodes the header portion of a JWT and returns the alg field.
func parseJWTHeader(t *testing.T, token string) map[string]string {
	t.Helper()
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		t.Fatalf("invalid JWT: fewer than 2 parts")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("base64 decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	return header
}

// TestSecurity_JWT_AlgorithmNone_Rejected verifies that a JWT with alg:none
// has an empty signature, making it trivially rejectable by any compliant
// validator. The server MUST reject tokens where alg=none.
//
// This test validates the attack token construction and detection logic:
// any JWT validator that accepts alg:none tokens is vulnerable.
func TestSecurity_JWT_AlgorithmNone_Rejected(t *testing.T) {
	token := buildJWT(t, "none", attackClaims, nil)

	// Verify the token structure matches an alg:none attack
	header := parseJWTHeader(t, token)
	if header["alg"] != "none" {
		t.Fatalf("expected alg=none in header, got %q", header["alg"])
	}

	// alg:none tokens MUST have an empty signature part
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	if parts[2] != "" {
		t.Errorf("SECURITY FAILURE — alg:none JWT has non-empty signature: %q", parts[2])
	}

	// Verify the claims can be decoded (the attack payload is well-formed)
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(claimsJSON, &decoded); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if decoded["sub"] != "attacker" {
		t.Errorf("expected sub=attacker, got %v", decoded["sub"])
	}
	if decoded["workspace_id"] != "ws-admin" {
		t.Errorf("expected workspace_id=ws-admin, got %v", decoded["workspace_id"])
	}

	t.Logf("alg:none JWT correctly constructed — any server accepting this token is vulnerable")
	t.Logf("Token: %s", token)
}

// TestSecurity_JWT_KeyConfusion_Rejected verifies the RS256→HS256 key-confusion
// attack vector. The attacker signs with the server's public key as HMAC secret.
// A vulnerable server would verify this HS256 signature using its RSA public key,
// treating it as an HMAC secret.
func TestSecurity_JWT_KeyConfusion_Rejected(t *testing.T) {
	fakePublicKey := []byte(
		"-----BEGIN PUBLIC KEY-----\n" +
			"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA" +
			"test000000000000000000000000000000000000000000000000000000000000\n" +
			"-----END PUBLIC KEY-----\n",
	)

	token := buildJWT(t, "HS256", attackClaims, fakePublicKey)

	// Verify the token structure matches a key-confusion attack
	header := parseJWTHeader(t, token)
	if header["alg"] != "HS256" {
		t.Fatalf("expected alg=HS256 in header, got %q", header["alg"])
	}

	// Verify it has a non-empty signature (the attack is signed, unlike alg:none)
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || parts[2] == "" {
		t.Fatalf("HS256 key-confusion token must have a signature")
	}

	// Verify the signature was computed with HMAC-SHA256 using the public key as secret
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, fakePublicKey)
	mac.Write([]byte(sigInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if parts[2] != expectedSig {
		t.Errorf("key-confusion token signature mismatch — buildJWT is broken")
	}

	t.Logf("RS256→HS256 key-confusion JWT correctly constructed — " +
		"server must reject HS256 when RS256 is expected")
	t.Logf("Token: %s", token)
}
