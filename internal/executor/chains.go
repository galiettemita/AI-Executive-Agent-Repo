package executor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func ComputeAuditChainHash(key []byte, previousChainHash, entryID, event, workspaceID, userID, timestampRFC3339, actionSummary string) string {
	parts := []string{
		strings.TrimSpace(previousChainHash),
		strings.TrimSpace(entryID),
		strings.TrimSpace(event),
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(userID),
		strings.TrimSpace(timestampRFC3339),
		strings.TrimSpace(actionSummary),
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strings.Join(parts, "||")))
	return hex.EncodeToString(mac.Sum(nil))
}

func ComputeAutoCommitProofHash(key []byte, previousProofHash, proofID, toolExecutionID, effectiveAutonomy, consentID, timestampRFC3339 string) string {
	parts := []string{
		strings.TrimSpace(previousProofHash),
		strings.TrimSpace(proofID),
		strings.TrimSpace(toolExecutionID),
		strings.TrimSpace(effectiveAutonomy),
		strings.TrimSpace(consentID),
		strings.TrimSpace(timestampRFC3339),
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strings.Join(parts, "||")))
	return hex.EncodeToString(mac.Sum(nil))
}
