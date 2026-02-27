package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type CapabilityResult struct {
	NormalizedQuery string
	ResponseHash    string
	Capabilities    []string
}

type Decision string

const (
	DecisionAllow                 Decision = "ALLOW"
	DecisionDeny                  Decision = "DENY"
	DecisionRequireOperatorReview Decision = "REQUIRE_OPERATOR_REVIEW"
	DecisionRequireUserApproval   Decision = "REQUIRE_USER_APPROVAL"
)

type PolicyInput struct {
	ServerID                       string
	RiskLevel                      string
	DeniedServerIDs                []string
	AllowedServerIDs               []string
	MaxAllowedRiskLevel            string
	BudgetExhausted                bool
	RequireOperatorReviewAtOrAbove string
	OAuthOwnerApprovalRequired     bool
	MCPDeployOwnerApprovalRequired bool
}

type ArtifactManifest struct {
	ImageDigest  string
	DigestSHA256 string
}

type Service struct {
	mu    sync.Mutex
	cache map[string]CapabilityResult
}

func NewService() *Service {
	return &Service{cache: map[string]CapabilityResult{}}
}

func normalizeQuery(query string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	sort.Strings(fields)
	return strings.Join(fields, " ")
}

func hash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func (s *Service) ResolveCapabilities(query string, allowLLMFallback bool) CapabilityResult {
	normalized := normalizeQuery(query)

	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.cache[normalized]; ok {
		return cached
	}

	capabilities := []string{}
	if strings.Contains(normalized, "calendar") {
		capabilities = append(capabilities, "calendar.scheduling")
	}
	if strings.Contains(normalized, "email") {
		capabilities = append(capabilities, "email.compose")
	}
	if strings.Contains(normalized, "crm") {
		capabilities = append(capabilities, "crm.update")
	}
	if len(capabilities) == 0 && allowLLMFallback {
		capabilities = append(capabilities, "general.assist")
	}

	sort.Strings(capabilities)
	responseHash := hash(normalized + "::" + strings.Join(capabilities, ","))
	result := CapabilityResult{NormalizedQuery: normalized, ResponseHash: responseHash, Capabilities: capabilities}
	s.cache[normalized] = result
	return result
}

func riskRank(level string) int {
	switch strings.ToUpper(level) {
	case "LOW":
		return 1
	case "MEDIUM":
		return 2
	case "ELEVATED":
		return 3
	case "CRITICAL":
		return 4
	default:
		return 0
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// DecideProvisioningV1 implements V9 §7.3 decision order.
func DecideProvisioningV1(input PolicyInput) Decision {
	if contains(input.DeniedServerIDs, input.ServerID) {
		return DecisionDeny
	}
	if len(input.AllowedServerIDs) > 0 && !contains(input.AllowedServerIDs, input.ServerID) {
		return DecisionDeny
	}
	if riskRank(input.RiskLevel) > riskRank(input.MaxAllowedRiskLevel) {
		return DecisionDeny
	}
	if input.BudgetExhausted {
		return DecisionDeny
	}
	if riskRank(input.RiskLevel) >= riskRank(input.RequireOperatorReviewAtOrAbove) && input.RequireOperatorReviewAtOrAbove != "" {
		return DecisionRequireOperatorReview
	}
	if input.OAuthOwnerApprovalRequired {
		return DecisionRequireUserApproval
	}
	if input.MCPDeployOwnerApprovalRequired {
		return DecisionRequireUserApproval
	}
	return DecisionAllow
}

func VerifyArtifact(manifest ArtifactManifest, artifactBytes []byte) error {
	computed := hash(string(artifactBytes))
	if computed != manifest.DigestSHA256 {
		return fmt.Errorf("artifact digest mismatch")
	}
	if !strings.HasPrefix(manifest.ImageDigest, "sha256:") {
		return fmt.Errorf("invalid image digest format")
	}
	return nil
}

func DriftWatchdog(schemaChanged bool) string {
	if schemaChanged {
		return "quarantine"
	}
	return "healthy"
}
