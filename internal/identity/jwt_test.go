package identity

import (
	"testing"
	"time"
)

func TestJWTSpecConstants(t *testing.T) {
	t.Parallel()

	if got := JWTSigningAlgorithm(); got != "RS256" {
		t.Fatalf("unexpected jwt signing algorithm: %s", got)
	}
	if got := UserJWTLifetime(); got != time.Hour {
		t.Fatalf("unexpected user jwt lifetime: %s", got)
	}
	if got := AdminJWTLifetime(); got != 15*time.Minute {
		t.Fatalf("unexpected admin jwt lifetime: %s", got)
	}
	if got := JWKSPath(); got != "/.well-known/jwks.json" {
		t.Fatalf("unexpected jwks path: %s", got)
	}
}
