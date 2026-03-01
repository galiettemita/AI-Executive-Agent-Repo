package provisioning

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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

type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleDelegate Role = "delegate"
	RoleAuditor  Role = "auditor"
	RoleOperator Role = "operator"
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
	ImageDigest         string
	DigestSHA256        string
	SignaturePublicKey  string
	Signature           string
	SBOMS3URI           string
	VulnerabilityPassed bool
}

type CandidateMetrics struct {
	RiskPenalty      float64
	ReliabilityScore float64
	CostEfficiency   float64
}

type RankedCandidate struct {
	ServerID string
	Score    float64
}

type RankingInputV1 struct {
	ServerID                  string
	CapabilityMatchScore      float64
	ReliabilityScore          float64
	EstimatedMonthlyCost      float64
	BudgetRemaining           float64
	P95LatencyMS              float64
	ArtifactVerificationState string
	InAllowedServerIDs        bool
	PreviouslyDeclined        bool
}

type RankingFactorsV1 struct {
	CapabilityMatch     float64
	Reliability         float64
	Cost                float64
	Latency             float64
	Security            float64
	WorkspacePreference float64
}

type RankerVersion struct {
	Version int
	Weights map[string]float64
}

type Service struct {
	mu                sync.Mutex
	cache             map[string]CapabilityResult
	rankers           map[int]RankerVersion
	activeRanker      int
	explanationReplay map[string]string
}

func NewService() *Service {
	return &Service{
		cache:             map[string]CapabilityResult{},
		rankers:           map[int]RankerVersion{},
		explanationReplay: map[string]string{},
	}
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
	if manifest.SBOMS3URI == "" {
		return fmt.Errorf("missing sbom uri")
	}
	if !manifest.VulnerabilityPassed {
		return fmt.Errorf("vulnerability gate failed")
	}
	if manifest.SignaturePublicKey != "" || manifest.Signature != "" {
		if manifest.SignaturePublicKey == "" || manifest.Signature == "" {
			return fmt.Errorf("signature and public key must both be provided")
		}
		pubKey, err := base64.StdEncoding.DecodeString(manifest.SignaturePublicKey)
		if err != nil {
			return fmt.Errorf("invalid signature public key encoding: %w", err)
		}
		signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
		if err != nil {
			return fmt.Errorf("invalid signature encoding: %w", err)
		}
		if len(pubKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize {
			return fmt.Errorf("invalid signature key size")
		}
		if !ed25519.Verify(ed25519.PublicKey(pubKey), artifactBytes, signature) {
			return fmt.Errorf("artifact signature verification failed")
		}
	}
	return nil
}

func DriftWatchdog(schemaChanged bool) string {
	if schemaChanged {
		return "quarantine"
	}
	return "healthy"
}

func RoleRank(role Role) int {
	switch role {
	case RoleOwner:
		return 5
	case RoleAdmin:
		return 4
	case RoleDelegate:
		return 3
	case RoleAuditor:
		return 2
	case RoleOperator:
		return 1
	default:
		return 0
	}
}

func CanApproveOAuthAndDeploy(role Role) bool {
	return role == RoleOwner || role == RoleAdmin
}

func (s *Service) RegisterRankerVersion(version int, weights map[string]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyWeights := map[string]float64{}
	for k, v := range weights {
		copyWeights[k] = v
	}
	s.rankers[version] = RankerVersion{Version: version, Weights: copyWeights}
	if s.activeRanker == 0 {
		s.activeRanker = version
	}
}

func (s *Service) SetActiveRankerVersion(version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rankers[version]; !ok {
		return fmt.Errorf("ranker version not found: %d", version)
	}
	s.activeRanker = version
	return nil
}

func (s *Service) RankServers(metrics map[string]CandidateMetrics) ([]RankedCandidate, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ranker, ok := s.rankers[s.activeRanker]
	if !ok {
		return nil, "", fmt.Errorf("active ranker not configured")
	}
	riskWeight := ranker.Weights["risk_penalty"]
	reliabilityWeight := ranker.Weights["reliability_score"]
	costWeight := ranker.Weights["cost_efficiency"]

	out := make([]RankedCandidate, 0, len(metrics))
	for serverID, values := range metrics {
		score := (values.ReliabilityScore * reliabilityWeight) + (values.CostEfficiency * costWeight) - (values.RiskPenalty * riskWeight)
		out = append(out, RankedCandidate{ServerID: serverID, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ServerID < out[j].ServerID
		}
		return out[i].Score > out[j].Score
	})

	explanationKey := fmt.Sprintf("v=%d::%s", ranker.Version, rankedCandidatesKey(out))
	if replay, ok := s.explanationReplay[explanationKey]; ok {
		return out, replay, nil
	}
	explanation := hash(explanationKey)
	s.explanationReplay[explanationKey] = explanation
	return out, explanation, nil
}

func rankedCandidatesKey(candidates []RankedCandidate) string {
	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parts = append(parts, fmt.Sprintf("%s=%.6f", candidate.ServerID, candidate.Score))
	}
	return strings.Join(parts, ",")
}

func DefaultRankerWeightsV1() map[string]float64 {
	return map[string]float64{
		"capability_match":     0.30,
		"reliability":          0.25,
		"cost":                 0.15,
		"latency":              0.10,
		"security":             0.15,
		"workspace_preference": 0.05,
	}
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func securityScoreForState(state string) float64 {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "verified":
		return 1.0
	case "unverified":
		return 0.5
	case "rejected":
		return 0.0
	default:
		return 0.5
	}
}

func workspacePreferenceScore(inAllowedServerIDs, previouslyDeclined bool) float64 {
	if previouslyDeclined {
		return 0.0
	}
	if inAllowedServerIDs {
		return 1.0
	}
	return 0.5
}

func RankFactorsV1(input RankingInputV1) RankingFactorsV1 {
	cost := 0.0
	if input.BudgetRemaining > 0 {
		cost = 1.0 - (input.EstimatedMonthlyCost / input.BudgetRemaining)
	}
	latency := 1.0 - (input.P95LatencyMS / 5000.0)
	return RankingFactorsV1{
		CapabilityMatch:     clamp01(input.CapabilityMatchScore),
		Reliability:         clamp01(input.ReliabilityScore),
		Cost:                clamp01(cost),
		Latency:             clamp01(latency),
		Security:            securityScoreForState(input.ArtifactVerificationState),
		WorkspacePreference: workspacePreferenceScore(input.InAllowedServerIDs, input.PreviouslyDeclined),
	}
}

func RankScoreV1(input RankingInputV1, weights map[string]float64) float64 {
	f := RankFactorsV1(input)
	return (weights["capability_match"] * f.CapabilityMatch) +
		(weights["reliability"] * f.Reliability) +
		(weights["cost"] * f.Cost) +
		(weights["latency"] * f.Latency) +
		(weights["security"] * f.Security) +
		(weights["workspace_preference"] * f.WorkspacePreference)
}

func RankServersByFormulaV1(inputs []RankingInputV1, weights map[string]float64) []RankedCandidate {
	out := make([]RankedCandidate, 0, len(inputs))
	for _, input := range inputs {
		out = append(out, RankedCandidate{
			ServerID: input.ServerID,
			Score:    RankScoreV1(input, weights),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if absDiff(out[i].Score, out[j].Score) <= 0.001 {
			return out[i].ServerID < out[j].ServerID
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
