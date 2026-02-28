package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflowClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")

	assertFileContainsTokens(t, ciPath, []string{
		"go install honnef.co/go/tools/cmd/staticcheck@v0.5.1",
		"install infrastructure and security tooling",
		"TERRAFORM_VERSION=1.9.8",
		"HELM_VERSION=v3.16.4",
		"terraform version",
		"helm version",
		"github.com/trufflesecurity/trufflehog/v3@v3.90.4",
		"trivy --version",
		"syft version",
		"gofmt check",
		"go test ./... -count=1",
		"migration lint",
		"openapi lint",
		"json schema lint",
		"determinism suite",
		"dependency cve scan (trivy)",
		"docker image scan (trivy)",
		"secrets scan (trufflehog)",
		"govulncheck baseline",
		"bash scripts/security/run_govulncheck.sh",
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

func TestCIWorkflowNoSecuritySkipPaths(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")
	body, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	content := string(body)

	disallowed := []string{
		"trivy not installed on runner; skip",
		"trufflehog not installed on runner; skip",
		"syft not installed on runner; skip",
	}
	for _, token := range disallowed {
		if strings.Contains(content, token) {
			t.Fatalf("ci workflow contains disallowed security skip token: %q", token)
		}
	}
}
