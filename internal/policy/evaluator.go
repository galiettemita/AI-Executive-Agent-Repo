package policy

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/rego"
)

//go:generate make -C ../.. opa-sync
//go:embed rego/*.rego
var regoFS embed.FS

// Evaluator compiles all embedded Rego modules at startup and exposes
// prepared queries. All methods are safe for concurrent use.
type Evaluator struct {
	authzQuery     *rego.PreparedEvalQuery
	toolWriteQuery *rego.PreparedEvalQuery
	autonomyQuery  *rego.PreparedEvalQuery
	budgetQuery    *rego.PreparedEvalQuery
	v10Query       *rego.PreparedEvalQuery
}

// Decision is the result of a policy evaluation.
type Decision struct {
	Allowed bool
	Reason  string // non-empty when Allowed=false
	Policy  string // which policy fired
}

// PlanAuthzInput is the input to EvaluatePlan.
type PlanAuthzInput struct {
	WorkspaceID string   `json:"workspace_id"`
	PlanID      string   `json:"plan_id"`
	ToolKeys    []string `json:"tool_keys"`
	RiskLevel   string   `json:"risk_level"`   // "low"|"elevated"|"critical"
	Autonomy    string   `json:"autonomy_tier"` // "A0"|"A1"|"A2"|"A3"|"A4"
	BudgetCents int      `json:"budget_cents"`
	UsedCents   int      `json:"used_cents"`
	UserTier    string   `json:"user_tier"` // "free"|"pro"|"enterprise"
}

// NewEvaluator loads all embedded Rego modules and prepares eval queries.
// Returns an error if any module fails to parse or compile.
func NewEvaluator() (*Evaluator, error) {
	modules := map[string]string{}
	err := fs.WalkDir(regoFS, "rego", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".rego" {
			return nil
		}
		data, rerr := regoFS.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", path, rerr)
		}
		modules[path] = string(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("policy: walk rego dir: %w", err)
	}

	build := func(query string, moduleNames ...string) (*rego.PreparedEvalQuery, error) {
		opts := []func(*rego.Rego){rego.Query(query)}
		for name, src := range modules {
			// If specific modules requested, only include those.
			if len(moduleNames) > 0 {
				include := false
				for _, mn := range moduleNames {
					if strings.Contains(name, mn) {
						include = true
						break
					}
				}
				if !include {
					continue
				}
			}
			opts = append(opts, rego.Module(name, src))
		}
		r := rego.New(opts...)
		q, cerr := r.PrepareForEval(context.Background())
		if cerr != nil {
			return nil, fmt.Errorf("prepare %q: %w", query, cerr)
		}
		return &q, nil
	}

	// Each query targets the specific package in the corresponding rego file.
	// Compile each module separately to avoid cross-package conflicts (especially v10_gates
	// which uses rego.v1 syntax).
	authzQ, err := build("data.brevio.authz", "authz.rego")
	if err != nil {
		return nil, err
	}
	toolWQ, err := build("data.brevio.tool_write_gate", "tool_write_gate.rego")
	if err != nil {
		return nil, err
	}
	autoQ, err := build("data.brevio.autonomy", "autonomy.rego")
	if err != nil {
		return nil, err
	}
	budgetQ, err := build("data.brevio.budget", "budget_enforcement.rego")
	if err != nil {
		return nil, err
	}
	v10Q, err := build("data.brevio.v10", "v10_gates.rego")
	if err != nil {
		return nil, err
	}

	return &Evaluator{
		authzQuery:     authzQ,
		toolWriteQuery: toolWQ,
		autonomyQuery:  autoQ,
		budgetQuery:    budgetQ,
		v10Query:       v10Q,
	}, nil
}

// EvaluatePlan runs all applicable gates against a plan authorization request.
// Evaluation order: autonomy → budget → write-gate per tool → v10 kill-switch.
// Returns the first deny encountered, or Allowed=true if all pass.
func (e *Evaluator) EvaluatePlan(ctx context.Context, input PlanAuthzInput) Decision {
	// Gate 1: Autonomy tier — brevio.autonomy.allow requires autonomy_level == "A4"
	if d := e.evalAutonomy(ctx, input); !d.Allowed {
		return d
	}

	// Gate 2: Budget enforcement — brevio.budget.deny contains "BUDGET_CALLS_EXHAUSTED" when exhausted
	if d := e.evalBudget(ctx, input); !d.Allowed {
		return d
	}

	// Gate 3: Write-gate per tool key — brevio.tool_write_gate.require_approval
	for _, toolKey := range input.ToolKeys {
		if isWriteOp(toolKey) {
			if d := e.evalWriteGate(ctx, input, toolKey); !d.Allowed {
				return d
			}
		}
	}

	return Decision{Allowed: true}
}

func (e *Evaluator) evalAutonomy(ctx context.Context, input PlanAuthzInput) Decision {
	if e.autonomyQuery == nil {
		return Decision{Allowed: true}
	}
	// The autonomy.rego checks input.autonomy_level == "A4"
	inp := map[string]any{
		"workspace_id":   input.WorkspaceID,
		"autonomy_level": input.Autonomy,
		"risk_level":     input.RiskLevel,
		"tool_keys":      input.ToolKeys,
	}
	rs, err := e.autonomyQuery.Eval(ctx, rego.EvalInput(inp))
	if err != nil {
		return Decision{Allowed: false, Reason: fmt.Sprintf("OPA_EVAL_ERROR:%v", err), Policy: "autonomy"}
	}
	// data.brevio.autonomy returns a result set; check for the "allow" key
	allowed := extractBool(rs, "allow")
	if !allowed {
		return Decision{Allowed: false, Reason: "AUTONOMY_DENY", Policy: "autonomy"}
	}
	return Decision{Allowed: true}
}

func (e *Evaluator) evalBudget(ctx context.Context, input PlanAuthzInput) Decision {
	if e.budgetQuery == nil {
		return Decision{Allowed: true}
	}
	// brevio.budget.deny is a set of deny messages when budget_exhausted is true
	budgetExhausted := input.BudgetCents > 0 && input.UsedCents >= input.BudgetCents
	inp := map[string]any{
		"workspace_id":     input.WorkspaceID,
		"budget_exhausted": budgetExhausted,
		"budget_cents":     input.BudgetCents,
		"used_cents":       input.UsedCents,
	}
	rs, err := e.budgetQuery.Eval(ctx, rego.EvalInput(inp))
	if err != nil {
		return Decision{Allowed: false, Reason: fmt.Sprintf("OPA_EVAL_ERROR:%v", err), Policy: "budget_enforcement"}
	}
	// Check if deny set is non-empty
	if hasDenyMessages(rs) {
		return Decision{Allowed: false, Reason: "BUDGET_CALLS_EXHAUSTED", Policy: "budget_enforcement"}
	}
	return Decision{Allowed: true}
}

func (e *Evaluator) evalWriteGate(ctx context.Context, input PlanAuthzInput, toolKey string) Decision {
	if e.toolWriteQuery == nil {
		return Decision{Allowed: true}
	}
	inp := map[string]any{
		"workspace_id":   input.WorkspaceID,
		"tool_key":       toolKey,
		"is_write":       true,
		"autonomy_level": input.Autonomy,
		"risk_level":     input.RiskLevel,
		"user_tier":      input.UserTier,
	}
	rs, err := e.toolWriteQuery.Eval(ctx, rego.EvalInput(inp))
	if err != nil {
		return Decision{Allowed: false, Reason: fmt.Sprintf("OPA_EVAL_ERROR:%v", err), Policy: "tool_write_gate"}
	}
	// require_approval == true means the write needs human approval → deny autonomous execution
	requireApproval := extractBool(rs, "require_approval")
	if requireApproval {
		return Decision{Allowed: false, Reason: fmt.Sprintf("TOOL_WRITE_REQUIRES_APPROVAL:%s", toolKey), Policy: "tool_write_gate"}
	}
	return Decision{Allowed: true}
}

// extractBool looks for a boolean value in the OPA result set.
// The query "data.package.name" returns a result set where each expression
// is a map of the package's rules to their values.
func extractBool(rs rego.ResultSet, key string) bool {
	if len(rs) == 0 {
		return false
	}
	for _, expr := range rs[0].Expressions {
		switch v := expr.Value.(type) {
		case bool:
			return v
		case map[string]any:
			if b, ok := v[key].(bool); ok {
				return b
			}
		}
	}
	return false
}

// hasDenyMessages checks if the OPA result contains a non-empty deny set.
func hasDenyMessages(rs rego.ResultSet) bool {
	if len(rs) == 0 {
		return false
	}
	for _, expr := range rs[0].Expressions {
		switch v := expr.Value.(type) {
		case map[string]any:
			if deny, ok := v["deny"]; ok {
				switch d := deny.(type) {
				case []any:
					return len(d) > 0
				}
			}
		}
	}
	return false
}

// isWriteOp returns true if the tool key represents a write-side operation.
func isWriteOp(toolKey string) bool {
	lower := strings.ToLower(toolKey)
	for _, verb := range []string{"send", "create", "update", "delete", "post", "write",
		"modify", "cancel", "pay", "transfer", "book", "order", "move", "set"} {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}
