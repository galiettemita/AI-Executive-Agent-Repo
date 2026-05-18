package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateKillSwitchBlocksExecution verifies that kill switch evaluation
// precedes all other gates and blocks execution even when other policies allow.
func TestGateKillSwitchBlocksExecution(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Kill switch check must be FIRST in AuthorizePlanActivity.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	content, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("failed to read activities.go: %v", err)
	}
	src := string(content)

	// Verify kill switch check exists in AuthorizePlanActivity.
	if !strings.Contains(src, "killSwitchCheck") {
		t.Error("AuthorizePlanActivity does not reference killSwitchCheck")
	}
	if !strings.Contains(src, "KILL_SWITCH_ACTIVE") {
		t.Error("AuthorizePlanActivity does not return KILL_SWITCH_ACTIVE on kill switch")
	}

	// Verify kill switch check exists in ExecuteToolActivity.
	if !strings.Contains(src, "KILL_SWITCH_ACTIVE: execution blocked") {
		t.Error("ExecuteToolActivity does not check kill switch before commit")
	}

	// Kill switch must precede all other gates — NNR-107.
	// Find the AuthorizePlanActivity function and verify kill switch is checked
	// before receipt generation.
	authIdx := strings.Index(src, "func (a *Activities) AuthorizePlanActivity")
	if authIdx < 0 {
		t.Fatal("AuthorizePlanActivity not found")
	}
	authBody := src[authIdx:]
	ksIdx := strings.Index(authBody, "killSwitchCheck")
	receiptIdx := strings.Index(authBody, "receiptID := hashKey")
	if ksIdx < 0 || receiptIdx < 0 {
		t.Fatal("missing kill switch or receipt in AuthorizePlanActivity")
	}
	if ksIdx > receiptIdx {
		t.Error("NNR-107 violation: kill switch check is NOT before receipt generation in AuthorizePlanActivity")
	}
}

// TestGateKillSwitchBlocksExecutorCommit verifies kill switch in executor commit.
func TestGateKillSwitchBlocksExecutorCommit(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	content, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("failed to read activities.go: %v", err)
	}
	src := string(content)

	// Find ExecuteToolActivity and verify kill switch precedes payload hash.
	execIdx := strings.Index(src, "func (a *Activities) ExecuteToolActivity")
	if execIdx < 0 {
		t.Fatal("ExecuteToolActivity not found")
	}
	execBody := src[execIdx:]
	ksIdx := strings.Index(execBody, "killSwitchCheck")
	hashIdx := strings.Index(execBody, "payloadHash := hashKey")
	if ksIdx < 0 || hashIdx < 0 {
		t.Fatal("missing kill switch or payload hash in ExecuteToolActivity")
	}
	if ksIdx > hashIdx {
		t.Error("NNR-107 violation: kill switch check is NOT before executor commit")
	}
}

// TestGateOPAEnforcedDenyByDefault verifies OPA is enforced with deny-by-default
// behavior when OPA is configured but unreachable.
func TestGateOPAEnforcedDenyByDefault(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// OPA evaluator must implement deny-by-default.
	opaPath := filepath.Join(root, "internal", "control", "opa.go")
	assertFileContainsTokens(t, opaPath, []string{
		"OPA_UNAVAILABLE_DENY_BY_DEFAULT",
		"HasOPAClient",
		"EvaluateGateWithOPA",
		"deny-by-default",
	})

	// OPA middleware must exist for route-level enforcement.
	middlewarePath := filepath.Join(root, "internal", "control", "opa_middleware.go")
	assertFileNonEmpty(t, middlewarePath)
	assertFileContainsTokens(t, middlewarePath, []string{
		"OPAPolicyMiddleware",
		"EvaluateGateWithOPA",
		"policy_denied",
		"buildPolicyInputFromRequest",
	})

	// OPA client must have circuit breaker.
	clientPath := filepath.Join(root, "internal", "control", "opa_client.go")
	assertFileContainsTokens(t, clientPath, []string{
		"CircuitBreakerThreshold",
		"CircuitBreakerCooldown",
	})
}

// TestGateAdminEndpointsRequireRealAuth verifies admin endpoints support
// session-based authentication, not just X-User-Role header.
func TestGateAdminEndpointsRequireRealAuth(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Admin auth module must exist with session management.
	authPath := filepath.Join(root, "internal", "admin", "auth.go")
	assertFileNonEmpty(t, authPath)
	assertFileContainsTokens(t, authPath, []string{
		"AdminSession",
		"SessionStore",
		"IssueSession",
		"ValidateSession",
		"RevokeSession",
		"AdminAuthMiddleware",
		"SessionFromContext",
		"AuditAction",
	})

	// Admin handlers must check session-based auth.
	handlersPath := filepath.Join(root, "internal", "admin", "handlers.go")
	handlersContent, err := os.ReadFile(handlersPath)
	if err != nil {
		t.Fatalf("failed to read handlers.go: %v", err)
	}
	src := string(handlersContent)

	if !strings.Contains(src, "SessionFromContext") {
		t.Error("admin handlers do not use SessionFromContext for auth")
	}
}

// TestGateSkillACLOverridesIntegrated verifies skill ACL overrides are
// integrated into the authorization path.
func TestGateSkillACLOverridesIntegrated(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Skill ACL checker interface must exist.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"SkillACLChecker",
		"skillACLCheck",
		"SKILL_ACL_DENIED",
	})

	// PgSkillACLRepository must exist.
	pgACLPath := filepath.Join(root, "internal", "admin", "pg_skill_acl_repository.go")
	assertFileNonEmpty(t, pgACLPath)
	assertFileContainsTokens(t, pgACLPath, []string{
		"SkillACLRepository",
		"IsSkillAllowed",
		"SetOverride",
		"expires_at",
	})
}

// TestGateKillSwitchRepositoryExists verifies the DB-backed kill switch repository.
func TestGateKillSwitchRepositoryExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	pgKSPath := filepath.Join(root, "internal", "admin", "pg_kill_switch_repository.go")
	assertFileNonEmpty(t, pgKSPath)
	assertFileContainsTokens(t, pgKSPath, []string{
		"KillSwitchRepository",
		"IsActive",
		"Activate",
		"Deactivate",
		"LogAction",
		"agent_kill_switch_log",
	})
}

// TestGateGovernanceWiredInWorker verifies kill switch and skill ACL are wired
// into the temporal worker's production dependencies.
func TestGateGovernanceWiredInWorker(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerMainPath := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, workerMainPath, []string{
		"KillSwitchCheck",
		"SkillACLCheck",
		"NewPgKillSwitchRepository",
		"NewPgSkillACLRepository",
	})
}

// TestGateOPAMiddlewareConsistentInput verifies OPA middleware constructs
// policy inputs consistently across all decision points.
func TestGateOPAMiddlewareConsistentInput(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	middlewarePath := filepath.Join(root, "internal", "control", "opa_middleware.go")
	assertFileContainsTokens(t, middlewarePath, []string{
		"buildPolicyInputFromRequest",
		"EvaluatePolicyForAction",
		"PolicyInput",
	})
}
