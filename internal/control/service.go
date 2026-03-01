package control

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrFirewallBlocked  = errors.New("firewall blocked content")
	ErrTokenExpired     = errors.New("approval token expired")
	ErrTokenInvalid     = errors.New("approval token invalid")
	ErrTokenReplay      = errors.New("approval token nonce replay")
	ErrSchemaValidation = errors.New("schema validation failed")
	ErrSemanticVerifier = errors.New("semantic verifier failed")
	ErrToolRateCap      = errors.New("tool rate cap exceeded")
	ErrBudgetExceeded   = errors.New("budget exhausted")
)

type FirewallResult struct {
	Allowed bool
	Reason  string
}

type DecisionInput struct {
	AutonomyLevel          string
	ToolRiskLevel          string
	IsWrite                bool
	RateLimited            bool
	BudgetExhausted        bool
	FirewallAllowed        bool
	SemanticVerifierPassed bool
	BlockedTool            bool
}

type DecisionOutput struct {
	Decision   string
	ReasonCode string
}

type RateLimitPolicy struct {
	Category     string
	CallsPerMin  int
	CallsPerHour int
	Action       string
}

type GlobalRateLimitPolicy struct {
	Key          string
	Limit        int
	Window       string
	Action       string
	SoftEnforced bool
}

type BudgetDefault struct {
	Plan                    string
	MaxMonthlyLLMCostUSD    float64
	MaxMonthlyAPICalls      int
	MaxConcurrentMCPServers int
	MaxSingleTransactionUSD float64
}

type ProactiveDecision struct {
	AllowSilent bool
	ReasonCode  string
}

type LoadSheddingInput struct {
	Tier                   string
	IsHealthOrAudit        bool
	IsWriteOperation       bool
	IsProactiveBehavior    bool
	IsA3PlusAutoCommit     bool
	IsNonCriticalConnector bool
}

type FinancialApprovalDecision struct {
	Decision               string
	ApprovalRisk           string
	TTL                    time.Duration
	RequiresSecondApprover bool
}

type RecipientVerificationInput struct {
	ContactMatch       bool
	RecentConversation bool
	ExplicitAllowlist  bool
	SameDomain         bool
}

type approvalPayload struct {
	Action     string    `json:"action"`
	RiskLevel  string    `json:"risk_level"`
	Nonce      string    `json:"nonce"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	KeyVersion string    `json:"key_version"`
}

type ApprovalService struct {
	mu         sync.Mutex
	secret     []byte
	keyVersion string
	seenNonces map[string]struct{}
}

func NewApprovalService(secret, keyVersion string) *ApprovalService {
	return &ApprovalService{
		secret:     []byte(secret),
		keyVersion: keyVersion,
		seenNonces: map[string]struct{}{},
	}
}

func (a *ApprovalService) GenerateToken(action, riskLevel, nonce string, now time.Time) (string, error) {
	ttl := 5 * time.Minute
	if strings.EqualFold(riskLevel, "CRITICAL") {
		ttl = 30 * time.Second
	}
	payload := approvalPayload{
		Action:     action,
		RiskLevel:  strings.ToUpper(riskLevel),
		Nonce:      nonce,
		IssuedAt:   now.UTC(),
		ExpiresAt:  now.UTC().Add(ttl),
		KeyVersion: a.keyVersion,
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sig := sign(a.secret, blob)
	return base64.StdEncoding.EncodeToString(blob) + "." + sig, nil
}

func (a *ApprovalService) ValidateToken(token string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrTokenInvalid
	}

	blob, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrTokenInvalid
	}
	if !hmac.Equal([]byte(parts[1]), []byte(sign(a.secret, blob))) {
		return ErrTokenInvalid
	}

	var payload approvalPayload
	if err := json.Unmarshal(blob, &payload); err != nil {
		return ErrTokenInvalid
	}

	if now.UTC().After(payload.ExpiresAt) {
		return ErrTokenExpired
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if _, seen := a.seenNonces[payload.Nonce]; seen {
		return ErrTokenReplay
	}
	a.seenNonces[payload.Nonce] = struct{}{}
	return nil
}

func sign(secret, blob []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(blob)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type Service struct {
	approval *ApprovalService
	mu       sync.Mutex

	toolRateCaps      map[string]int
	toolUsage         map[string]int
	monthlyBudgetCaps map[string]int
	monthlyBudgetUsed map[string]int
}

func NewService(secret string) *Service {
	return &Service{
		approval:          NewApprovalService(secret, "v1"),
		toolRateCaps:      map[string]int{},
		toolUsage:         map[string]int{},
		monthlyBudgetCaps: map[string]int{},
		monthlyBudgetUsed: map[string]int{},
	}
}

func DefaultToolRateLimits() map[string]RateLimitPolicy {
	return map[string]RateLimitPolicy{
		"calendar_read":  {Category: "calendar_read", CallsPerMin: 60, CallsPerHour: 600, Action: "warn"},
		"calendar_write": {Category: "calendar_write", CallsPerMin: 20, CallsPerHour: 100, Action: "require_confirm"},
		"email_read":     {Category: "email_read", CallsPerMin: 30, CallsPerHour: 300, Action: "warn"},
		"email_send":     {Category: "email_send", CallsPerMin: 10, CallsPerHour: 50, Action: "require_confirm"},
		"message_send":   {Category: "message_send", CallsPerMin: 30, CallsPerHour: 200, Action: "require_confirm"},
		"document_read":  {Category: "document_read", CallsPerMin: 30, CallsPerHour: 300, Action: "warn"},
		"document_write": {Category: "document_write", CallsPerMin: 15, CallsPerHour: 100, Action: "require_confirm"},
		"financial_read": {Category: "financial_read", CallsPerMin: 20, CallsPerHour: 100, Action: "warn"},
		"financial_write": {
			Category: "financial_write", CallsPerMin: 5, CallsPerHour: 20, Action: "deny",
		},
		"crm_read":   {Category: "crm_read", CallsPerMin: 30, CallsPerHour: 300, Action: "warn"},
		"crm_write":  {Category: "crm_write", CallsPerMin: 10, CallsPerHour: 50, Action: "require_confirm"},
		"web_search": {Category: "web_search", CallsPerMin: 30, CallsPerHour: 200, Action: "downgrade_tier"},
		"web_fetch":  {Category: "web_fetch", CallsPerMin: 20, CallsPerHour: 100, Action: "downgrade_tier"},
		"mcp_tool":   {Category: "mcp_tool", CallsPerMin: 20, CallsPerHour: 200, Action: "require_confirm"},
	}
}

func DefaultGlobalRateLimits() map[string]GlobalRateLimitPolicy {
	return map[string]GlobalRateLimitPolicy{
		"tool_calls_per_min":  {Key: "tool_calls_per_min", Limit: 60, Window: "minute", Action: "deny"},
		"tool_calls_per_hour": {Key: "tool_calls_per_hour", Limit: 500, Window: "hour", Action: "deny"},
		"llm_calls_per_min":   {Key: "llm_calls_per_min", Limit: 30, Window: "minute", Action: "downgrade_tier"},
		"inbound_msgs_per_min": {
			Key: "inbound_msgs_per_min", Limit: 20, Window: "minute", Action: "warn", SoftEnforced: true,
		},
	}
}

func DefaultBudgetByPlan() map[string]BudgetDefault {
	return map[string]BudgetDefault{
		"free": {
			Plan:                    "free",
			MaxMonthlyLLMCostUSD:    5.00,
			MaxMonthlyAPICalls:      5000,
			MaxConcurrentMCPServers: 2,
			MaxSingleTransactionUSD: 0,
		},
		"pro": {
			Plan:                    "pro",
			MaxMonthlyLLMCostUSD:    50.00,
			MaxMonthlyAPICalls:      50000,
			MaxConcurrentMCPServers: 10,
			MaxSingleTransactionUSD: 500,
		},
		"business": {
			Plan:                    "business",
			MaxMonthlyLLMCostUSD:    200.00,
			MaxMonthlyAPICalls:      200000,
			MaxConcurrentMCPServers: 25,
			MaxSingleTransactionUSD: 5000,
		},
		"enterprise": {
			Plan:                    "enterprise",
			MaxMonthlyLLMCostUSD:    -1,
			MaxMonthlyAPICalls:      -1,
			MaxConcurrentMCPServers: -1,
			MaxSingleTransactionUSD: -1,
		},
	}
}

func BudgetStatus(usedUSD, capUSD float64) (warn bool, exhausted bool) {
	if capUSD <= 0 {
		return false, false
	}
	if usedUSD >= capUSD {
		return true, true
	}
	return usedUSD >= (capUSD * 0.80), false
}

func (s *Service) Approval() *ApprovalService {
	return s.approval
}

func (s *Service) FirewallCheck(rawInput string) FirewallResult {
	if len(rawInput) > 8000 {
		return FirewallResult{Allowed: false, Reason: "INPUT_TOO_LARGE"}
	}

	l1 := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, rawInput)

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+all\s+previous\s+instructions`),
		regexp.MustCompile(`(?i)system\s*prompt`),
		regexp.MustCompile(`(?i)exfiltrate`),
		regexp.MustCompile(`(?i)\\b(ssrf|169\.254\.169\.254)\\b`),
	}
	for _, p := range patterns {
		if p.MatchString(l1) {
			return FirewallResult{Allowed: false, Reason: "PATTERN_MATCH_BLOCK"}
		}
	}

	if strings.Contains(strings.ToLower(l1), "call arbitrary tool") {
		return FirewallResult{Allowed: false, Reason: "SEMANTIC_BLOCK"}
	}

	return FirewallResult{Allowed: true, Reason: "ALLOW"}
}

// FirewallCheckWithSchema applies L1-L3 content checks and L4 schema checks.
func (s *Service) FirewallCheckWithSchema(rawInput string, toolInput map[string]any, requiredFields []string) FirewallResult {
	result := s.FirewallCheck(rawInput)
	if !result.Allowed {
		return result
	}
	if err := s.ValidateToolInput(requiredFields, toolInput); err != nil {
		return FirewallResult{Allowed: false, Reason: "SCHEMA_VALIDATION_FAILED"}
	}
	return FirewallResult{Allowed: true, Reason: "ALLOW"}
}

func (s *Service) EvaluateGate(input DecisionInput) DecisionOutput {
	if !input.FirewallAllowed {
		return DecisionOutput{Decision: "deny", ReasonCode: "FIREWALL_BLOCKED"}
	}
	if !input.SemanticVerifierPassed {
		return DecisionOutput{Decision: "deny", ReasonCode: "SEMANTIC_VERIFIER_FAILED"}
	}
	if input.BlockedTool {
		return DecisionOutput{Decision: "deny", ReasonCode: "TOOL_BLOCKED"}
	}
	if input.RateLimited {
		return DecisionOutput{Decision: "deny", ReasonCode: "RATE_LIMIT_EXCEEDED"}
	}
	if input.BudgetExhausted {
		return DecisionOutput{Decision: "deny", ReasonCode: "BUDGET_CALLS_EXHAUSTED"}
	}
	if !input.IsWrite {
		return DecisionOutput{Decision: "allow", ReasonCode: "READ_ONLY"}
	}

	autonomy := strings.ToUpper(input.AutonomyLevel)
	risk := strings.ToUpper(input.ToolRiskLevel)
	switch autonomy {
	case "A0":
		return DecisionOutput{Decision: "deny", ReasonCode: "AUTONOMY_A0_WRITE_DENIED"}
	case "A1":
		return DecisionOutput{Decision: "require_approval", ReasonCode: "AUTONOMY_A1_CONFIRM_REQUIRED"}
	case "A2":
		if risk == "CRITICAL" || risk == "ELEVATED" {
			return DecisionOutput{Decision: "require_approval", ReasonCode: "AUTONOMY_A2_CONFIRM_REQUIRED"}
		}
		return DecisionOutput{Decision: "allow", ReasonCode: "AUTONOMY_A2_ALLOW"}
	case "A3":
		if risk == "CRITICAL" {
			return DecisionOutput{Decision: "require_approval", ReasonCode: "AUTONOMY_A3_CRITICAL_CONFIRM"}
		}
		return DecisionOutput{Decision: "allow", ReasonCode: "AUTONOMY_A3_AUTO_COMMIT"}
	case "A4":
		return DecisionOutput{Decision: "allow", ReasonCode: "AUTONOMY_A4_FULL_AUTO"}
	default:
		return DecisionOutput{Decision: "deny", ReasonCode: fmt.Sprintf("UNKNOWN_AUTONOMY_%s", autonomy)}
	}
}

// EvaluateExecutionPolicy applies core gate logic and additive recipient/memory constraints.
func (s *Service) EvaluateExecutionPolicy(input DecisionInput, recipientVerified bool, memoryWriteAllowed bool) DecisionOutput {
	baseDecision := s.EvaluateGate(input)
	if baseDecision.Decision == "deny" {
		return baseDecision
	}
	if !recipientVerified {
		return DecisionOutput{Decision: "deny", ReasonCode: "RECIPIENT_UNVERIFIED"}
	}
	if input.IsWrite && !memoryWriteAllowed {
		return DecisionOutput{Decision: "deny", ReasonCode: "MEMORY_WRITE_BLOCKED"}
	}
	return baseDecision
}

func (s *Service) ValidateToolInput(requiredFields []string, toolInput map[string]any) error {
	for _, field := range requiredFields {
		value, ok := toolInput[field]
		if !ok {
			return fmt.Errorf("%w: missing required field %s", ErrSchemaValidation, field)
		}
		if strValue, isString := value.(string); isString && strings.TrimSpace(strValue) == "" {
			return fmt.Errorf("%w: empty required field %s", ErrSchemaValidation, field)
		}
	}
	return nil
}

func (s *Service) VerifyToolOutput(requiredFields []string, output map[string]any) error {
	for _, field := range requiredFields {
		if _, ok := output[field]; !ok {
			return fmt.Errorf("%w: missing field %s", ErrSemanticVerifier, field)
		}
	}
	return nil
}

func (s *Service) SetToolRateCap(workspaceID, toolKey string, maxCalls int) error {
	if workspaceID == "" || toolKey == "" {
		return fmt.Errorf("workspace_id and tool_key are required")
	}
	if maxCalls <= 0 {
		return fmt.Errorf("max_calls must be > 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolRateCaps[toolUsageKey(workspaceID, toolKey)] = maxCalls
	return nil
}

func (s *Service) ConsumeToolCall(workspaceID, toolKey string) error {
	key := toolUsageKey(workspaceID, toolKey)
	s.mu.Lock()
	defer s.mu.Unlock()
	maxCalls, hasCap := s.toolRateCaps[key]
	if !hasCap {
		return nil
	}
	next := s.toolUsage[key] + 1
	if next > maxCalls {
		return ErrToolRateCap
	}
	s.toolUsage[key] = next
	return nil
}

func (s *Service) SetMonthlyBudgetCap(workspaceID string, maxUnits int) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if maxUnits <= 0 {
		return fmt.Errorf("max_units must be > 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monthlyBudgetCaps[workspaceID] = maxUnits
	return nil
}

func (s *Service) ConsumeBudget(workspaceID string, units int) error {
	if units <= 0 {
		return fmt.Errorf("units must be > 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	capUnits, hasCap := s.monthlyBudgetCaps[workspaceID]
	if !hasCap {
		return nil
	}
	next := s.monthlyBudgetUsed[workspaceID] + units
	if next > capUnits {
		return ErrBudgetExceeded
	}
	s.monthlyBudgetUsed[workspaceID] = next
	return nil
}

func (s *Service) BudgetExhausted(workspaceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	capUnits, hasCap := s.monthlyBudgetCaps[workspaceID]
	if !hasCap {
		return false
	}
	return s.monthlyBudgetUsed[workspaceID] >= capUnits
}

func toolUsageKey(workspaceID, toolKey string) string {
	return workspaceID + "::" + toolKey
}

// EvaluateProactiveSilentExecution enforces V9 proactive action rules.
// Silent execution requires BOTH domain autonomy >= A2 and explicit proactive opt-in.
func (s *Service) EvaluateProactiveSilentExecution(domainAutonomy string, proactiveEnabled bool) ProactiveDecision {
	normalized := strings.ToUpper(strings.TrimSpace(domainAutonomy))
	switch normalized {
	case "A2", "A3", "A4":
		if proactiveEnabled {
			return ProactiveDecision{AllowSilent: true, ReasonCode: "PROACTIVE_SILENT_ALLOWED"}
		}
		return ProactiveDecision{AllowSilent: false, ReasonCode: "PROACTIVE_USER_CONSENT_REQUIRED"}
	case "A0", "A1":
		return ProactiveDecision{AllowSilent: false, ReasonCode: "PROACTIVE_AUTONOMY_TOO_LOW"}
	default:
		return ProactiveDecision{AllowSilent: false, ReasonCode: "PROACTIVE_UNKNOWN_AUTONOMY"}
	}
}

// EvaluateLoadShedding enforces V9 load shedding tiers D0-D5.
func (s *Service) EvaluateLoadShedding(input LoadSheddingInput) DecisionOutput {
	tier := strings.ToUpper(strings.TrimSpace(input.Tier))
	if tier == "" {
		tier = "D0"
	}

	if tier == "D5" {
		if input.IsHealthOrAudit {
			return DecisionOutput{Decision: "allow", ReasonCode: "LOAD_SHEDDING_D5_HEALTH_AUDIT_ONLY"}
		}
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_D5_MINIMAL_MODE"}
	}

	if tier == "D4" && input.IsWriteOperation {
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_D4_READ_ONLY"}
	}
	if tier == "D3" && input.IsNonCriticalConnector {
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_D3_NON_CRITICAL_DISABLED"}
	}
	if tier == "D2" && input.IsA3PlusAutoCommit {
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_D2_A3_PLUS_AUTOCOMMIT_DISABLED"}
	}
	if tier == "D1" && input.IsProactiveBehavior {
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_D1_PROACTIVE_DISABLED"}
	}

	switch tier {
	case "D0", "D1", "D2", "D3", "D4":
		return DecisionOutput{Decision: "allow", ReasonCode: "LOAD_SHEDDING_ALLOWED"}
	default:
		return DecisionOutput{Decision: "deny", ReasonCode: "LOAD_SHEDDING_UNKNOWN_TIER"}
	}
}

var autonomyOrder = map[string]int{
	"A0": 0,
	"A1": 1,
	"A2": 2,
	"A3": 3,
	"A4": 4,
}

func normalizeAutonomy(level string) string {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	if _, ok := autonomyOrder[normalized]; ok {
		return normalized
	}
	return "A0"
}

// EffectiveAutonomy returns MIN(global_autonomy, domain_autonomy, workspace_cap).
func EffectiveAutonomy(globalAutonomy, domainAutonomy, workspaceCap string) string {
	candidates := []string{
		normalizeAutonomy(globalAutonomy),
		normalizeAutonomy(domainAutonomy),
		normalizeAutonomy(workspaceCap),
	}
	minLevel := candidates[0]
	for _, level := range candidates[1:] {
		if autonomyOrder[level] < autonomyOrder[minLevel] {
			minLevel = level
		}
	}
	return minLevel
}

// CanUpgradeAutonomy enforces the A0->A4 upgrade path guards.
func CanUpgradeAutonomy(fromLevel, toLevel string, hasConsent bool, zeroOverrideDays int, adminApproved bool) bool {
	from := normalizeAutonomy(fromLevel)
	to := normalizeAutonomy(toLevel)
	if autonomyOrder[to] <= autonomyOrder[from] {
		return false
	}

	switch {
	case from == "A0" && to == "A1":
		return true
	case from == "A1" && to == "A2":
		return hasConsent
	case from == "A2" && to == "A3":
		return hasConsent && zeroOverrideDays >= 7
	case from == "A3" && to == "A4":
		return hasConsent && zeroOverrideDays >= 30 && adminApproved
	default:
		return false
	}
}

// HoldWindowForAction returns the outbox hold window by autonomy with risk overrides.
func HoldWindowForAction(autonomyLevel, riskLevel string) time.Duration {
	var base time.Duration
	switch normalizeAutonomy(autonomyLevel) {
	case "A2":
		base = 60 * time.Second
	case "A3", "A4":
		base = 0
	default:
		base = 0
	}

	switch strings.ToLower(strings.TrimSpace(riskLevel)) {
	case "critical":
		if base < 300*time.Second {
			base = 300 * time.Second
		}
	case "elevated":
		if base < 120*time.Second {
			base = 120 * time.Second
		}
	}
	return base
}

func WriteBudgetForTier(tier string) int {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case "T1":
		return 3
	case "T2":
		return 8
	case "T3":
		return 15
	default:
		return 3
	}
}

func IsRecipientVerified(input RecipientVerificationInput) bool {
	return input.ContactMatch || input.RecentConversation || input.ExplicitAllowlist || input.SameDomain
}

func RecipientVerificationPrompt(recipient string) string {
	return fmt.Sprintf("I don't have %s in your contacts. Please confirm you'd like to send this.", recipient)
}

// EvaluateFinancialApproval determines whether financial action requires approval and whether two-man applies.
func EvaluateFinancialApproval(amount, dailySpend, monthlyBudget, maxSingleTransaction float64, isNewPayee, modifiesRecurringPayment bool, workspaceType string, hasAdminUsers bool) FinancialApprovalDecision {
	effectiveSingleThreshold := 1000.0
	if maxSingleTransaction > 0 && maxSingleTransaction < effectiveSingleThreshold {
		effectiveSingleThreshold = maxSingleTransaction
	}
	if amount > effectiveSingleThreshold {
		return FinancialApprovalDecision{
			Decision:               "REQUIRE_APPROVAL",
			ApprovalRisk:           "critical",
			TTL:                    30 * time.Second,
			RequiresSecondApprover: strings.EqualFold(strings.TrimSpace(workspaceType), "professional") && hasAdminUsers,
		}
	}

	effectiveDailyThreshold := 5000.0
	if monthlyBudget > 0 {
		halfMonthly := monthlyBudget * 0.50
		if halfMonthly < effectiveDailyThreshold {
			effectiveDailyThreshold = halfMonthly
		}
	}
	if dailySpend > effectiveDailyThreshold || isNewPayee || modifiesRecurringPayment {
		return FinancialApprovalDecision{
			Decision:               "REQUIRE_APPROVAL",
			ApprovalRisk:           "elevated",
			TTL:                    5 * time.Minute,
			RequiresSecondApprover: false,
		}
	}

	return FinancialApprovalDecision{Decision: "ALLOW", ApprovalRisk: "none", TTL: 0}
}

func SupportedConsentTypes() []string {
	return []string{
		"autonomy_upgrade",
		"proactive_behavior",
		"data_sharing",
		"memory_persistence",
		"delegation_access",
		"financial_access",
		"health_access",
	}
}

func SupportedConsentScopes() []string {
	return []string{
		"workspace:{workspace_id}",
		"domain:{domain_name}",
		"connector:{connector_key}",
		"global",
	}
}

func SupportedConsentProofChannels() []string {
	return []string{
		"interactive_reply",
		"button_tap",
		"sms_otp",
		"email_link",
	}
}
