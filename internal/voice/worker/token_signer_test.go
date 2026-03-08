package worker

import (
	"strings"
	"testing"
	"time"
)

func TestSignTokenSuccess(t *testing.T) {
	t.Parallel()

	ts := NewTokenSigner()
	token, err := ts.SignToken(TokenClaims{
		RoomName:        "room-1",
		ParticipantID:   "participant-1",
		ParticipantName: "Alice",
		ExpiresIn:       5 * time.Minute,
	}, "api-key-123", "api-secret-456")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
}

func TestSignTokenValidation(t *testing.T) {
	t.Parallel()

	ts := NewTokenSigner()

	_, err := ts.SignToken(TokenClaims{RoomName: "room", ParticipantID: "p1"}, "", "secret")
	if err == nil {
		t.Fatal("expected error for empty apiKey")
	}

	_, err = ts.SignToken(TokenClaims{RoomName: "room", ParticipantID: "p1"}, "key", "")
	if err == nil {
		t.Fatal("expected error for empty apiSecret")
	}

	_, err = ts.SignToken(TokenClaims{RoomName: "", ParticipantID: "p1"}, "key", "secret")
	if err == nil {
		t.Fatal("expected error for empty room name")
	}

	_, err = ts.SignToken(TokenClaims{RoomName: "room", ParticipantID: ""}, "key", "secret")
	if err == nil {
		t.Fatal("expected error for empty participant ID")
	}
}

func TestVerifyTokenValid(t *testing.T) {
	t.Parallel()

	ts := NewTokenSigner()
	token, _ := ts.SignToken(TokenClaims{
		RoomName:      "room-1",
		ParticipantID: "p1",
		ExpiresIn:     5 * time.Minute,
	}, "key", "secret")

	valid, err := ts.VerifyToken(token, "secret")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !valid {
		t.Fatal("expected token to be valid")
	}
}

func TestVerifyTokenInvalidSecret(t *testing.T) {
	t.Parallel()

	ts := NewTokenSigner()
	token, _ := ts.SignToken(TokenClaims{
		RoomName:      "room-1",
		ParticipantID: "p1",
		ExpiresIn:     5 * time.Minute,
	}, "key", "secret")

	valid, err := ts.VerifyToken(token, "wrong-secret")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if valid {
		t.Fatal("expected token to be invalid with wrong secret")
	}
}

func TestVerifyTokenInvalidFormat(t *testing.T) {
	t.Parallel()

	ts := NewTokenSigner()
	_, err := ts.VerifyToken("not-a-jwt", "secret")
	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
}
