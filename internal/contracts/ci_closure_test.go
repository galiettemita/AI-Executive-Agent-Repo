package contracts

import (
	"path/filepath"
	"testing"
)

func TestCIWorkflowClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")

	assertFileContainsTokens(t, ciPath, []string{
		"go install honnef.co/go/tools/cmd/staticcheck@v0.5.1",
		"gofmt check",
		"go test ./... -count=1",
		"migration lint",
		"openapi lint",
		"json schema lint",
		"determinism suite",
		"dependency cve scan (trivy)",
		"docker image scan (trivy)",
		"secrets scan (trufflehog)",
		"contract tests",
		"integration tests",
		"prompt injection tests",
		"webhook signature suite",
		"provisioning compensation suite",
		"onboarding fixture suite",
		"sbom generation (syft)",
		"go test ./internal/context -count=1",
		"go test ./internal/rag -count=1",
		"go test ./internal/rag/eval -count=1",
		"go test ./internal/sessions -count=1",
		"go test ./internal/temporal_reasoning -count=1",
		"go test ./internal/guardrails -count=1",
		"go test ./internal/tool_health -count=1",
		"go test ./internal/feature_flags -count=1",
		"go test ./internal/crdt -count=1",
		"go test ./internal/streaming -count=1",
		"go test ./internal/errors -count=1",
		"go test ./internal/event_schemas -count=1",
		"go test ./internal/compliance -count=1",
		"go test ./internal/admin -count=1",
		"go test ./internal/security/pii -count=1",
		"go test ./internal/security/sandbox -count=1",
		"go test ./internal/caching -count=1",
		"go test ./internal/model_tiers -count=1",
	})
}
