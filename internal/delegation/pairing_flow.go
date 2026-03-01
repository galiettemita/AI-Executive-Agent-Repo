package delegation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type PairingGrantProposal struct {
	AllowedToolKeys []string
	AllowedDomains  []string
	SharedMemory    []string
	MaxAutonomy     string
	ExpiresAt       time.Time
}

func GeneratePairingCode(ownerWorkspaceID, ownerUserID string, now time.Time) string {
	sum := sha256.Sum256([]byte(ownerWorkspaceID + "::" + ownerUserID + "::" + now.UTC().Format(time.RFC3339Nano)))
	return strings.ToUpper(hex.EncodeToString(sum[:]))[:6]
}

func PairingCodeTTL() time.Duration {
	return 15 * time.Minute
}

func ValidatePairingCode(createdAt, now time.Time) error {
	if now.UTC().After(createdAt.UTC().Add(PairingCodeTTL())) {
		return fmt.Errorf("pairing code expired")
	}
	return nil
}

func DelegationMaxAutonomy(level string) string {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	switch normalized {
	case "A0", "A1", "A2":
		return normalized
	default:
		return "A2"
	}
}

func NormalizePairingGrant(grant PairingGrantProposal) PairingGrantProposal {
	grant.MaxAutonomy = DelegationMaxAutonomy(grant.MaxAutonomy)
	return grant
}
