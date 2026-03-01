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

type AuditChainEntry struct {
	PreviousChainHash string
	EntryID           string
	Event             string
	WorkspaceID       string
	UserID            string
	TimestampRFC3339  string
	ActionSummary     string
	ChainHash         string
}

type AutoCommitProofEntry struct {
	PreviousProofHash string
	ProofID           string
	ToolExecutionID   string
	EffectiveAutonomy string
	ConsentID         string
	TimestampRFC3339  string
	ProofHash         string
}

func VerifyAuditChain(key []byte, entries []AuditChainEntry) bool {
	for _, entry := range entries {
		expected := ComputeAuditChainHash(
			key,
			entry.PreviousChainHash,
			entry.EntryID,
			entry.Event,
			entry.WorkspaceID,
			entry.UserID,
			entry.TimestampRFC3339,
			entry.ActionSummary,
		)
		if entry.ChainHash != expected {
			return false
		}
	}
	return true
}

func VerifyAutoCommitProofChain(key []byte, entries []AutoCommitProofEntry) bool {
	for _, entry := range entries {
		expected := ComputeAutoCommitProofHash(
			key,
			entry.PreviousProofHash,
			entry.ProofID,
			entry.ToolExecutionID,
			entry.EffectiveAutonomy,
			entry.ConsentID,
			entry.TimestampRFC3339,
		)
		if entry.ProofHash != expected {
			return false
		}
	}
	return true
}
