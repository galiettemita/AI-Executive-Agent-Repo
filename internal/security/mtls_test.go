package security

import (
	"testing"
	"time"
)

func TestDefaultInternalMTLSPolicy(t *testing.T) {
	t.Parallel()

	policy := DefaultInternalMTLSPolicy()
	if policy.KeyType != "ECDSA_P256" || policy.CertificateLifetime != 90*24*time.Hour || policy.RotateBeforeExpiry != 30*24*time.Hour {
		t.Fatalf("unexpected mtls policy: %+v", policy)
	}
	identities := InternalServiceIdentities()
	if len(identities) != 5 {
		t.Fatalf("unexpected internal identity count: %d", len(identities))
	}
}
