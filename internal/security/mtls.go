package security

import (
	"os"
	"time"
)

type MTLSCertificatePolicy struct {
	CAProvider          string
	KeyType             string
	CertificateLifetime time.Duration
	RotateBeforeExpiry  time.Duration
}

func DefaultInternalMTLSPolicy() MTLSCertificatePolicy {
	return MTLSCertificatePolicy{
		CAProvider:          "aws_acm_private_ca",
		KeyType:             "ECDSA_P256",
		CertificateLifetime: 90 * 24 * time.Hour,
		RotateBeforeExpiry:  30 * 24 * time.Hour,
	}
}

func InternalServiceIdentities() []string {
	return []string{
		"gateway.brevio.internal",
		"brain.brevio.internal",
		"control.brevio.internal",
		"executor.brevio.internal",
		"canvas.brevio.internal",
	}
}

// IsIstioMTLSEnabled returns true when Istio handles inter-service mTLS.
// When enabled, manual mTLS cert provisioning for inter-service calls is skipped;
// cert_rotator remains active for external API client certificates (Anthropic, OpenAI, etc.).
func IsIstioMTLSEnabled() bool {
	return os.Getenv("ISTIO_MTLS_ENABLED") == "true"
}

// InternalMTLSRequired returns true if manual mTLS certs are needed for inter-service comms.
// Returns false when Istio handles mTLS via sidecar injection.
func InternalMTLSRequired() bool {
	return !IsIstioMTLSEnabled()
}
