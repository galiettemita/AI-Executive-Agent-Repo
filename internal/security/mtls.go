package security

import "time"

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
