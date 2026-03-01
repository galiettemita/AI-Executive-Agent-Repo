package connectors

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrOAuthStateInvalid = errors.New("oauth state invalid")
	ErrOAuthStateExpired = errors.New("oauth state expired")
)

type OAuthStatePayload struct {
	WorkspaceID  string `json:"workspace_id"`
	ConnectorKey string `json:"connector_key"`
	Nonce        string `json:"nonce"`
	Timestamp    int64  `json:"timestamp"`
}

func OAuthStateRedisKey(nonce string) string {
	return "oauth_state:" + strings.TrimSpace(nonce)
}

func GenerateOAuthState(signingKey []byte, workspaceID, connectorKey, nonce string, now time.Time) (string, error) {
	if len(signingKey) == 0 {
		return "", fmt.Errorf("signing key is required")
	}
	payload := OAuthStatePayload{
		WorkspaceID:  strings.TrimSpace(workspaceID),
		ConnectorKey: strings.TrimSpace(connectorKey),
		Nonce:        strings.TrimSpace(nonce),
		Timestamp:    now.UTC().Unix(),
	}
	if payload.WorkspaceID == "" || payload.ConnectorKey == "" || payload.Nonce == "" {
		return "", fmt.Errorf("workspace_id, connector_key, and nonce are required")
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(payload.WorkspaceID))
	mac.Write([]byte("||"))
	mac.Write([]byte(payload.ConnectorKey))
	mac.Write([]byte("||"))
	mac.Write([]byte(payload.Nonce))
	mac.Write([]byte("||"))
	mac.Write([]byte(strconv.FormatInt(payload.Timestamp, 10)))
	signature := hex.EncodeToString(mac.Sum(nil))
	encoded := base64.RawURLEncoding.EncodeToString(rawPayload)
	return encoded + "." + signature, nil
}

func ValidateOAuthState(signingKey []byte, state string, now time.Time, ttl time.Duration) (OAuthStatePayload, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	parts := strings.Split(strings.TrimSpace(state), ".")
	if len(parts) != 2 {
		return OAuthStatePayload{}, ErrOAuthStateInvalid
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return OAuthStatePayload{}, ErrOAuthStateInvalid
	}
	var payload OAuthStatePayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return OAuthStatePayload{}, ErrOAuthStateInvalid
	}

	expectedState, err := GenerateOAuthState(signingKey, payload.WorkspaceID, payload.ConnectorKey, payload.Nonce, time.Unix(payload.Timestamp, 0).UTC())
	if err != nil {
		return OAuthStatePayload{}, ErrOAuthStateInvalid
	}
	expectedParts := strings.Split(expectedState, ".")
	if len(expectedParts) != 2 || !hmac.Equal([]byte(expectedParts[1]), []byte(parts[1])) {
		return OAuthStatePayload{}, ErrOAuthStateInvalid
	}

	if now.UTC().After(time.Unix(payload.Timestamp, 0).UTC().Add(ttl)) {
		return OAuthStatePayload{}, ErrOAuthStateExpired
	}
	return payload, nil
}

func GeneratePKCECodeVerifier(seed []byte) string {
	if len(seed) == 0 {
		hash := sha256.Sum256([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
		seed = hash[:]
	}
	verifier := base64.RawURLEncoding.EncodeToString(seed)
	if len(verifier) < 43 {
		extra := sha256.Sum256(seed)
		verifier += base64.RawURLEncoding.EncodeToString(extra[:])
	}
	if len(verifier) > 128 {
		verifier = verifier[:128]
	}
	return verifier
}

func BuildPKCECodeChallengeS256(codeVerifier string) (string, error) {
	codeVerifier = strings.TrimSpace(codeVerifier)
	if len(codeVerifier) < 43 || len(codeVerifier) > 128 {
		return "", fmt.Errorf("pkce code_verifier must be 43-128 chars")
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func BuildOAuthAuthorizationURL(authorizeURL, clientID, redirectURI string, scopes []string, state, codeChallenge string, extra map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(authorizeURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")

	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		query.Set(key, extra[key])
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func TokenNeedsRefresh(expiresAt, now time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	return now.UTC().Add(5 * time.Minute).After(expiresAt.UTC())
}

func OAuthErrorAction(errorCode string) string {
	switch strings.ToLower(strings.TrimSpace(errorCode)) {
	case "invalid_grant":
		return "mark_needs_reauth"
	case "access_denied":
		return "record_provisioning_declined"
	case "network_failure":
		return "retry_token_exchange_3x"
	case "state_invalid":
		return "http_400_state_invalid"
	default:
		return "fail_provisioning_step"
	}
}
