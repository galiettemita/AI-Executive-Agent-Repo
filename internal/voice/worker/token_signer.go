// Deprecated: Use LiveKitTokenSigner from token_signer_lk.go instead.
// This file's manual JWT implementation is kept for backward compatibility
// during the migration window. Remove after all callers are updated.
package worker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TokenClaims holds the claims for a LiveKit room token.
type TokenClaims struct {
	RoomName        string
	ParticipantID   string
	ParticipantName string
	ExpiresIn       time.Duration
}

// TokenSigner creates JWT tokens for LiveKit room access.
type TokenSigner struct{}

// NewTokenSigner creates a new TokenSigner.
func NewTokenSigner() *TokenSigner {
	return &TokenSigner{}
}

// jwtHeader is the fixed JWT header for HS256.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// jwtPayload represents the JWT claims.
type jwtPayload struct {
	ISS             string `json:"iss"`
	Sub             string `json:"sub"`
	Name            string `json:"name"`
	Room            string `json:"room"`
	RoomJoin        bool   `json:"roomJoin"`
	RoomCreate      bool   `json:"roomCreate"`
	CanPublish      bool   `json:"canPublish"`
	CanSubscribe    bool   `json:"canSubscribe"`
	IAT             int64  `json:"iat"`
	EXP             int64  `json:"exp"`
}

// SignToken creates a signed JWT token with HS256.
func (ts *TokenSigner) SignToken(claims TokenClaims, apiKey, apiSecret string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("apiKey must not be empty")
	}
	if apiSecret == "" {
		return "", fmt.Errorf("apiSecret must not be empty")
	}
	if claims.RoomName == "" {
		return "", fmt.Errorf("room name must not be empty")
	}
	if claims.ParticipantID == "" {
		return "", fmt.Errorf("participant ID must not be empty")
	}

	if claims.ExpiresIn <= 0 {
		claims.ExpiresIn = 10 * time.Minute
	}

	now := time.Now()

	header := jwtHeader{Alg: "HS256", Typ: "JWT"}
	payload := jwtPayload{
		ISS:          apiKey,
		Sub:          claims.ParticipantID,
		Name:         claims.ParticipantName,
		Room:         claims.RoomName,
		RoomJoin:     true,
		RoomCreate:   true,
		CanPublish:   true,
		CanSubscribe: true,
		IAT:          now.Unix(),
		EXP:          now.Add(claims.ExpiresIn).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerB64 + "." + payloadB64

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

// VerifyToken verifies a JWT token signature.
func (ts *TokenSigner) VerifyToken(token, apiSecret string) (bool, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return false, fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(signingInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(parts[2])), nil
}
