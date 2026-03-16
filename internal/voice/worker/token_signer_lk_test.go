package worker

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeJWTClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.SplitN(token, ".", 3)
	require.Len(t, parts, 3, "JWT must have 3 parts")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func decodeJWTHeader(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.SplitN(token, ".", 3)
	require.Len(t, parts, 3)
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var h map[string]any
	require.NoError(t, json.Unmarshal(header, &h))
	return h
}

func getVideoGrant(t *testing.T, claims map[string]any) map[string]any {
	t.Helper()
	vRaw, ok := claims["video"]
	require.True(t, ok, "claims must contain 'video' key")
	v, ok := vRaw.(map[string]any)
	require.True(t, ok, "'video' must be an object")
	return v
}

func TestNewLiveKitTokenSigner_EmptyAPIKey(t *testing.T) {
	_, err := NewLiveKitTokenSigner("", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiKey")
}

func TestNewLiveKitTokenSigner_EmptyAPISecret(t *testing.T) {
	_, err := NewLiveKitTokenSigner("key", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiSecret")
}

func TestLiveKitTokenSigner_Sign_EmptyRoomName(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	_, err := s.Sign(LiveKitTokenClaims{ParticipantID: "p1", Role: LiveKitRoleAgent})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RoomName")
}

func TestLiveKitTokenSigner_Sign_EmptyParticipantID(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	_, err := s.Sign(LiveKitTokenClaims{RoomName: "room", Role: LiveKitRoleAgent})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ParticipantID")
}

func TestLiveKitTokenSigner_Sign_UnknownRole(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	_, err := s.Sign(LiveKitTokenClaims{RoomName: "room", ParticipantID: "p1", Role: "admin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role")
}

func TestLiveKitTokenSigner_Sign_AgentRole(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "agent-1", Role: LiveKitRoleAgent,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	video := getVideoGrant(t, claims)
	assert.Equal(t, true, video["roomJoin"])
	assert.Equal(t, true, video["canPublish"])
	assert.Equal(t, true, video["canSubscribe"])
	// roomCreate must be false or absent (omitempty removes false booleans).
	if rc, ok := video["roomCreate"]; ok {
		assert.Equal(t, false, rc)
	}
}

func TestLiveKitTokenSigner_Sign_UserRole(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "user-1", Role: LiveKitRoleUser,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	video := getVideoGrant(t, claims)
	assert.Equal(t, true, video["canPublish"])
	assert.Equal(t, true, video["canSubscribe"])
	// roomCreate must be false or absent (omitempty removes false booleans).
	if rc, ok := video["roomCreate"]; ok {
		assert.Equal(t, false, rc)
	}
}

func TestLiveKitTokenSigner_Sign_ObserverRole(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "obs-1", Role: LiveKitRoleObserver,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	video := getVideoGrant(t, claims)
	assert.Equal(t, false, video["canPublish"])
	assert.Equal(t, true, video["canSubscribe"])
	// roomCreate must be false or absent (omitempty removes false booleans).
	if rc, ok := video["roomCreate"]; ok {
		assert.Equal(t, false, rc)
	}
}

func TestLiveKitTokenSigner_Sign_DefaultExpiry(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "p1", Role: LiveKitRoleAgent,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	exp := claims["exp"].(float64)
	iat := claims["nbf"].(float64) // LiveKit SDK uses nbf, not iat
	diff := exp - iat
	assert.InDelta(t, 600, diff, 5, "default expiry should be ~600 seconds")
}

func TestLiveKitTokenSigner_Sign_CustomExpiry(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "p1", Role: LiveKitRoleAgent,
		ExpiresIn: 30 * time.Minute,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	exp := claims["exp"].(float64)
	nbf := claims["nbf"].(float64)
	diff := exp - nbf
	assert.InDelta(t, 1800, diff, 5, "custom expiry should be ~1800 seconds")
}

func TestLiveKitTokenSigner_Sign_Metadata(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "p1", Role: LiveKitRoleAgent,
		Metadata: `{"role":"exec"}`,
	})
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	md, ok := claims["metadata"]
	require.True(t, ok, "metadata field must be present")
	assert.Equal(t, `{"role":"exec"}`, md)
}

func TestLiveKitTokenSigner_SignAgentToken_Convenience(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.SignAgentToken("sess-1", "ws-1", "room-1")
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	sub, _ := claims["sub"].(string)
	assert.True(t, strings.HasPrefix(sub, "agent-"), "identity should start with agent-")
}

func TestLiveKitTokenSigner_SignUserToken_Convenience(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.SignUserToken("user-1", "Alice", "room-1")
	require.NoError(t, err)

	claims := decodeJWTClaims(t, token)
	video := getVideoGrant(t, claims)
	assert.Equal(t, true, video["canPublish"])
	// roomCreate must be false or absent (omitempty removes false booleans).
	if rc, ok := video["roomCreate"]; ok {
		assert.Equal(t, false, rc)
	}
}

func TestLiveKitTokenSigner_TokenIsValidJWT(t *testing.T) {
	s, _ := NewLiveKitTokenSigner("key", "secret")
	token, err := s.Sign(LiveKitTokenClaims{
		RoomName: "room-1", ParticipantID: "p1", Role: LiveKitRoleAgent,
	})
	require.NoError(t, err)

	parts := strings.SplitN(token, ".", 3)
	assert.Len(t, parts, 3, "JWT must have 3 dot-separated parts")

	header := decodeJWTHeader(t, token)
	assert.Equal(t, "JWT", header["typ"])
	// LiveKit SDK may use HS256 or another alg — just verify it's present.
	_, hasAlg := header["alg"]
	assert.True(t, hasAlg, "header must contain 'alg'")
}

// Suppress unused import warning for math (used by InDelta via assert).
var _ = math.Abs
