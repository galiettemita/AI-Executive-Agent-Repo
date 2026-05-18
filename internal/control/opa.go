package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PolicyInput extends DecisionInput with workspace/domain context for OPA evaluation.
type PolicyInput struct {
	// Core gate fields (mirrors DecisionInput)
	AutonomyLevel          string `json:"autonomy_level"`
	ToolRiskLevel          string `json:"tool_risk_level"`
	IsWrite                bool   `json:"is_write"`
	RateLimited            bool   `json:"rate_limited"`
	BudgetExhausted        bool   `json:"budget_exhausted"`
	FirewallAllowed        bool   `json:"firewall_allowed"`
	SemanticVerifierPassed bool   `json:"semantic_verifier_passed"`
	BlockedTool            bool   `json:"blocked_tool"`

	// Extended context
	WorkspacePlan string `json:"workspace_plan"` // free, pro, business, enterprise
	Domain        string `json:"domain"`          // e.g. calendar, email, financial
	ToolKey       string `json:"tool_key"`
	UserRole      string `json:"user_role"` // owner, admin, member, viewer
	Timestamp     int64  `json:"timestamp"` // Unix seconds
}

// PolicyDecision is the result of an OPA policy evaluation.
type PolicyDecision struct {
	Allowed         bool           `json:"allowed"`
	Reason          string         `json:"reason"`
	RequiresApproval bool          `json:"requires_approval"`
	ReceiptRequired bool           `json:"receipt_required"`
	Constraints     map[string]any `json:"constraints,omitempty"`
}

// PolicyRule represents a loaded policy rule with evaluation logic.
type PolicyRule struct {
	Name     string
	Package  string
	Source   string // raw .rego source (stored for debugging/audit)
	Priority int    // lower = evaluated first
}

// OPAEvaluator provides policy-based decision making.
// In production, it delegates to an OPA HTTP sidecar via OPAClient.
// When opaClient is nil (devtest), it uses embedded Go gate logic.
type OPAEvaluator struct {
	mu        sync.RWMutex
	rules     []PolicyRule
	service   *Service
	opaClient *OPAClient

	// DefaultPackage is the OPA package path queried for policy decisions.
	// Defaults to "brevio.v9" if unset.
	DefaultPackage string
}

// NewOPAEvaluator creates a new OPA evaluator, optionally backed by a control Service
// for fallback to hardcoded gate logic. In production, call SetOPAClient to enable
// real OPA evaluation; without it, embedded gates are used (devtest mode).
func NewOPAEvaluator(service *Service) *OPAEvaluator {
	return &OPAEvaluator{
		rules:          make([]PolicyRule, 0),
		service:        service,
		DefaultPackage: "brevio.v9",
	}
}

// SetOPAClient attaches a live OPA HTTP client for production evaluation.
// When set, EvaluatePolicy delegates to the OPA sidecar instead of embedded gates.
func (e *OPAEvaluator) SetOPAClient(client *OPAClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.opaClient = client
}

// HasOPAClient returns true if a live OPA HTTP client is configured.
func (e *OPAEvaluator) HasOPAClient() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.opaClient != nil
}

// LoadPolicies reads all .rego files from the given directory and stores them
// as policy rules. Each file is parsed for its package declaration to derive
// the rule name.
func (e *OPAEvaluator) LoadPolicies(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("opa: read policies directory: %w", err)
	}

	var rules []PolicyRule
	priority := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".rego") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("opa: read policy %s: %w", entry.Name(), err)
		}

		pkg := extractPackageName(string(content))
		if pkg == "" {
			pkg = strings.TrimSuffix(entry.Name(), ".rego")
		}

		rules = append(rules, PolicyRule{
			Name:     strings.TrimSuffix(entry.Name(), ".rego"),
			Package:  pkg,
			Source:   string(content),
			Priority: priority,
		})
		priority++
	}

	e.mu.Lock()
	e.rules = rules
	e.mu.Unlock()
	return nil
}

// extractPackageName extracts the package name from a .rego file.
func extractPackageName(source string) string {
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package "))
		}
	}
	return ""
}

// PolicyCount returns the number of loaded policy rules.
func (e *OPAEvaluator) PolicyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

// EvaluatePolicy evaluates a policy decision. If an OPA client is configured,
// it delegates to the OPA sidecar. Otherwise, it uses embedded Go gate logic
// (devtest mode only — production builds must have an OPA client).
//
// Deny-by-default: if OPA is configured but unavailable (circuit open, timeout,
// error), the decision is DENY. The embedded fallback is only used when no OPA
// client is set (i.e., devtest/unit-test contexts).
func (e *OPAEvaluator) EvaluatePolicy(ctx context.Context, input PolicyInput) (*PolicyDecision, error) {
	if input.Timestamp == 0 {
		input.Timestamp = time.Now().Unix()
	}

	e.mu.RLock()
	client := e.opaClient
	pkg := e.DefaultPackage
	e.mu.RUnlock()

	// Production path: delegate to OPA sidecar.
	if client != nil {
		return client.EvaluatePolicy(ctx, pkg, input)
	}

	// Devtest/unit-test path: embedded gate logic.
	return e.evaluateEmbeddedGates(input)
}

// evaluateEmbeddedGates runs all built-in policy gates against the given input.
// This is the devtest fallback — not used when an OPA client is configured.
// Gates are evaluated in order; the first deny or require_approval stops evaluation.
func (e *OPAEvaluator) evaluateEmbeddedGates(input PolicyInput) (*PolicyDecision, error) {
	// Gate 1: Content firewall
	if decision := e.evaluateContentFirewallGate(input); decision != nil {
		return decision, nil
	}

	// Gate 2: Rate limit
	if decision := e.evaluateRateLimitGate(input); decision != nil {
		return decision, nil
	}

	// Gate 3: Budget
	if decision := e.evaluateBudgetGate(input); decision != nil {
		return decision, nil
	}

	// Gate 4: Tool write
	if decision := e.evaluateToolWriteGate(input); decision != nil {
		return decision, nil
	}

	// Gate 5: Autonomy
	if decision := e.evaluateAutonomyGate(input); decision != nil {
		return decision, nil
	}

	return &PolicyDecision{
		Allowed: true,
		Reason:  "all_gates_passed",
	}, nil
}

// evaluateContentFirewallGate enforces L1-L3 content firewall rules.
func (e *OPAEvaluator) evaluateContentFirewallGate(input PolicyInput) *PolicyDecision {
	if !input.FirewallAllowed {
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "content_firewall_blocked",
			ReceiptRequired: true,
		}
	}
	if !input.SemanticVerifierPassed {
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "semantic_verifier_failed",
			ReceiptRequired: true,
		}
	}
	return nil
}

// evaluateRateLimitGate checks tool and global rate limits.
func (e *OPAEvaluator) evaluateRateLimitGate(input PolicyInput) *PolicyDecision {
	if input.RateLimited {
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "rate_limit_exceeded",
			ReceiptRequired: false,
			Constraints: map[string]any{
				"tool_key": input.ToolKey,
				"domain":   input.Domain,
			},
		}
	}
	return nil
}

// evaluateBudgetGate checks monthly budget exhaustion.
func (e *OPAEvaluator) evaluateBudgetGate(input PolicyInput) *PolicyDecision {
	if input.BudgetExhausted {
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "budget_exhausted",
			ReceiptRequired: true,
			Constraints: map[string]any{
				"workspace_plan": input.WorkspacePlan,
			},
		}
	}
	return nil
}

// evaluateToolWriteGate enforces write-path restrictions.
func (e *OPAEvaluator) evaluateToolWriteGate(input PolicyInput) *PolicyDecision {
	if !input.IsWrite {
		return nil
	}
	if input.BlockedTool {
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "tool_blocked",
			ReceiptRequired: true,
			Constraints: map[string]any{
				"tool_key": input.ToolKey,
			},
		}
	}

	// Financial writes by non-admin/owner roles always require approval
	if input.Domain == "financial" && input.IsWrite {
		role := strings.ToLower(strings.TrimSpace(input.UserRole))
		if role != "owner" && role != "admin" {
			return &PolicyDecision{
				Allowed:          false,
				Reason:           "financial_write_restricted_role",
				RequiresApproval: true,
				ReceiptRequired:  true,
				Constraints: map[string]any{
					"user_role":    input.UserRole,
					"required_min": "admin",
				},
			}
		}
	}

	// Free-plan users cannot use write tools in certain domains
	if strings.ToLower(input.WorkspacePlan) == "free" && input.IsWrite {
		restricted := map[string]bool{
			"financial": true,
			"crm":       true,
		}
		if restricted[strings.ToLower(input.Domain)] {
			return &PolicyDecision{
				Allowed:         false,
				Reason:          "free_plan_write_restricted",
				ReceiptRequired: false,
				Constraints: map[string]any{
					"workspace_plan":  input.WorkspacePlan,
					"restricted_domain": input.Domain,
				},
			}
		}
	}

	return nil
}

// evaluateAutonomyGate enforces the A0-A4 autonomy ladder for write operations.
func (e *OPAEvaluator) evaluateAutonomyGate(input PolicyInput) *PolicyDecision {
	if !input.IsWrite {
		return nil
	}

	autonomy := strings.ToUpper(strings.TrimSpace(input.AutonomyLevel))
	risk := strings.ToUpper(strings.TrimSpace(input.ToolRiskLevel))

	switch autonomy {
	case "A0":
		return &PolicyDecision{
			Allowed:         false,
			Reason:          "autonomy_a0_write_denied",
			ReceiptRequired: true,
		}
	case "A1":
		return &PolicyDecision{
			Allowed:          false,
			Reason:           "autonomy_a1_approval_required",
			RequiresApproval: true,
			ReceiptRequired:  true,
		}
	case "A2":
		if risk == "CRITICAL" || risk == "ELEVATED" {
			return &PolicyDecision{
				Allowed:          false,
				Reason:           "autonomy_a2_elevated_risk_approval",
				RequiresApproval: true,
				ReceiptRequired:  true,
				Constraints: map[string]any{
					"risk_level": risk,
				},
			}
		}
		return &PolicyDecision{
			Allowed:         true,
			Reason:          "autonomy_a2_allow",
			ReceiptRequired: true,
		}
	case "A3":
		if risk == "CRITICAL" {
			return &PolicyDecision{
				Allowed:          false,
				Reason:           "autonomy_a3_critical_approval",
				RequiresApproval: true,
				ReceiptRequired:  true,
			}
		}
		return &PolicyDecision{
			Allowed:         true,
			Reason:          "autonomy_a3_auto_commit",
			ReceiptRequired: true,
		}
	case "A4":
		return &PolicyDecision{
			Allowed:         true,
			Reason:          "autonomy_a4_full_auto",
			ReceiptRequired: false,
		}
	default:
		return &PolicyDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("unknown_autonomy_%s", autonomy),
		}
	}
}

// EvaluateGateWithOPA evaluates a gate decision via OPA.
//
// Deny-by-default behavior:
//   - If OPA client is configured and OPA is unavailable, returns DENY (no fallback).
//   - If no OPA client is configured (devtest), falls back to embedded Service.EvaluateGate.
func (e *OPAEvaluator) EvaluateGateWithOPA(ctx context.Context, input PolicyInput) DecisionOutput {
	decision, err := e.EvaluatePolicy(ctx, input)
	if err != nil {
		// When OPA client is configured, deny-by-default on failure.
		if e.HasOPAClient() {
			return DecisionOutput{
				Decision:   "deny",
				ReasonCode: "OPA_UNAVAILABLE_DENY_BY_DEFAULT",
			}
		}
		// Devtest only: fall back to embedded gate logic.
		return e.fallbackGate(input)
	}

	if decision.RequiresApproval {
		return DecisionOutput{
			Decision:   "require_approval",
			ReasonCode: strings.ToUpper(decision.Reason),
		}
	}
	if !decision.Allowed {
		return DecisionOutput{
			Decision:   "deny",
			ReasonCode: strings.ToUpper(decision.Reason),
		}
	}
	return DecisionOutput{
		Decision:   "allow",
		ReasonCode: strings.ToUpper(decision.Reason),
	}
}

// fallbackGate converts PolicyInput to DecisionInput and delegates to the
// hardcoded Service.EvaluateGate. Only used in devtest mode (no OPA client).
func (e *OPAEvaluator) fallbackGate(input PolicyInput) DecisionOutput {
	if e.service == nil {
		return DecisionOutput{Decision: "deny", ReasonCode: "NO_SERVICE_FALLBACK"}
	}
	return e.service.EvaluateGate(DecisionInput{
		AutonomyLevel:          input.AutonomyLevel,
		ToolRiskLevel:          input.ToolRiskLevel,
		IsWrite:                input.IsWrite,
		RateLimited:            input.RateLimited,
		BudgetExhausted:        input.BudgetExhausted,
		FirewallAllowed:        input.FirewallAllowed,
		SemanticVerifierPassed: input.SemanticVerifierPassed,
		BlockedTool:            input.BlockedTool,
	})
}
