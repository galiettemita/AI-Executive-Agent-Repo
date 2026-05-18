package control

import (
	"context"
	"net/http"
)

// OPAPolicyMiddleware constructs a middleware that evaluates OPA policy for
// every protected request. When OPA is configured and unreachable, the request
// is denied (deny-by-default per NNR-107).
//
// The middleware builds a PolicyInput from request context and delegates to
// the OPAEvaluator for consistent policy evaluation across all decision points.
func OPAPolicyMiddleware(evaluator *OPAEvaluator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if evaluator == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Build policy input from request context.
			input := buildPolicyInputFromRequest(r)

			decision := evaluator.EvaluateGateWithOPA(r.Context(), input)

			switch decision.Decision {
			case "deny":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"policy_denied","reason":"` + decision.ReasonCode + `"}`))
				return
			case "require_approval":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"approval_required","reason":"` + decision.ReasonCode + `"}`))
				return
			default:
				// "allow" — proceed.
				next.ServeHTTP(w, r)
			}
		})
	}
}

// buildPolicyInputFromRequest constructs a PolicyInput from an HTTP request.
// This ensures consistent policy input construction across all decision points.
func buildPolicyInputFromRequest(r *http.Request) PolicyInput {
	isWrite := r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE"

	return PolicyInput{
		AutonomyLevel:          "A2",
		ToolRiskLevel:          "LOW",
		IsWrite:                isWrite,
		RateLimited:            false,
		BudgetExhausted:        false,
		FirewallAllowed:        true,
		SemanticVerifierPassed: true,
		BlockedTool:            false,
		WorkspacePlan:          r.Header.Get("X-Workspace-Plan"),
		Domain:                 "admin",
		ToolKey:                r.URL.Path,
		UserRole:               r.Header.Get("X-User-Role"),
		Timestamp:              0, // Will be filled by EvaluatePolicy.
	}
}

// EvaluatePolicyForAction provides a programmatic interface for non-HTTP
// decision points (e.g., Temporal activities) to evaluate OPA policy.
func EvaluatePolicyForAction(ctx context.Context, evaluator *OPAEvaluator, input PolicyInput) (allowed bool, reason string) {
	if evaluator == nil {
		return true, "no_evaluator"
	}
	decision := evaluator.EvaluateGateWithOPA(ctx, input)
	return decision.Decision == "allow", decision.ReasonCode
}
