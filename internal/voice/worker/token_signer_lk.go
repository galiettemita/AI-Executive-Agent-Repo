package worker

import (
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
)

// LiveKitTokenRole defines the permission level of a LiveKit token.
type LiveKitTokenRole string

const (
	// LiveKitRoleAgent — can publish (TTS audio) and subscribe (user audio),
	// cannot create rooms.
	LiveKitRoleAgent LiveKitTokenRole = "agent"

	// LiveKitRoleUser — can publish (microphone) and subscribe (agent TTS),
	// cannot create rooms.
	LiveKitRoleUser LiveKitTokenRole = "user"

	// LiveKitRoleObserver — can only subscribe (listen-only / monitoring),
	// cannot publish or create rooms.
	LiveKitRoleObserver LiveKitTokenRole = "observer"
)

// LiveKitTokenClaims holds the parameters for generating a LiveKit access token.
type LiveKitTokenClaims struct {
	RoomName        string
	ParticipantID   string
	ParticipantName string
	Role            LiveKitTokenRole
	ExpiresIn       time.Duration // defaults to 10 minutes if zero
	Metadata        string        // optional: JSON string for participant metadata
}

// LiveKitTokenSigner creates scoped LiveKit access tokens using the official SDK.
type LiveKitTokenSigner struct {
	apiKey    string
	apiSecret string
}

// NewLiveKitTokenSigner creates a LiveKitTokenSigner.
// Both apiKey and apiSecret must be non-empty.
func NewLiveKitTokenSigner(apiKey, apiSecret string) (*LiveKitTokenSigner, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("livekit token signer: apiKey must not be empty")
	}
	if apiSecret == "" {
		return nil, fmt.Errorf("livekit token signer: apiSecret must not be empty")
	}
	return &LiveKitTokenSigner{apiKey: apiKey, apiSecret: apiSecret}, nil
}

// Sign generates a LiveKit access token for the given claims.
// The token permissions are determined by Claims.Role.
func (l *LiveKitTokenSigner) Sign(claims LiveKitTokenClaims) (string, error) {
	if claims.RoomName == "" {
		return "", fmt.Errorf("livekit token: RoomName must not be empty")
	}
	if claims.ParticipantID == "" {
		return "", fmt.Errorf("livekit token: ParticipantID must not be empty")
	}

	expiresIn := claims.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 10 * time.Minute
	}

	// Build grant based on role.
	canPublish := false
	canSubscribe := false

	switch claims.Role {
	case LiveKitRoleAgent:
		canPublish = true
		canSubscribe = true
	case LiveKitRoleUser:
		canPublish = true
		canSubscribe = true
	case LiveKitRoleObserver:
		canPublish = false
		canSubscribe = true
	default:
		return "", fmt.Errorf("livekit token: unknown role %q", claims.Role)
	}

	// RoomCreate is explicitly never granted to any participant token.
	// Rooms must be created via the LiveKit admin API server-side.
	grant := &auth.VideoGrant{
		Room:         claims.RoomName,
		RoomJoin:     true,
		RoomCreate:   false,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}

	at := auth.NewAccessToken(l.apiKey, l.apiSecret).
		SetIdentity(claims.ParticipantID).
		SetName(claims.ParticipantName).
		SetValidFor(expiresIn).
		SetVideoGrant(grant)

	if claims.Metadata != "" {
		at.SetMetadata(claims.Metadata)
	}

	return at.ToJWT()
}

// SignAgentToken is a convenience method for agent tokens.
func (l *LiveKitTokenSigner) SignAgentToken(sessionID, workspaceID, roomName string) (string, error) {
	return l.Sign(LiveKitTokenClaims{
		RoomName:        roomName,
		ParticipantID:   "agent-" + sessionID,
		ParticipantName: "Brevio Agent",
		Role:            LiveKitRoleAgent,
		ExpiresIn:       10 * time.Minute,
	})
}

// SignUserToken is a convenience method for user participant tokens.
func (l *LiveKitTokenSigner) SignUserToken(userID, displayName, roomName string) (string, error) {
	return l.Sign(LiveKitTokenClaims{
		RoomName:        roomName,
		ParticipantID:   userID,
		ParticipantName: displayName,
		Role:            LiveKitRoleUser,
		ExpiresIn:       10 * time.Minute,
	})
}
