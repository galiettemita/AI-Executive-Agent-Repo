package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCIWorkflowAuthorityClosure asserts that exactly one CI workflow governs
// mainline gating and that the quarantined duplicate does not fire.
func TestCIWorkflowAuthorityClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflowDir := filepath.Join(root, ".github", "workflows")

	// The authoritative CI workflow must exist.
	authoritative := filepath.Join(workflowDir, "ci.yml")
	body, err := os.ReadFile(authoritative)
	if err != nil {
		t.Fatalf("authoritative CI workflow missing: %v", err)
	}

	content := string(body)
	if !strings.Contains(content, "name: ci") {
		t.Fatal("authoritative CI workflow ci.yml must have name 'ci'")
	}

	// The quarantined workflow must NOT exist at its original active location.
	quarantined := filepath.Join(workflowDir, "ci.yaml")
	if _, err := os.Stat(quarantined); err == nil {
		t.Fatal("quarantined CI workflow ci.yaml still exists at .github/workflows/ci.yaml — must be removed or moved to quarantine/")
	}

	// Verify the quarantine record exists.
	quarantineRecord := filepath.Join(workflowDir, "quarantine", "ci.yaml.quarantined")
	if _, err := os.Stat(quarantineRecord); err != nil {
		t.Fatalf("quarantine record missing: %v — ci.yaml must be quarantined at .github/workflows/quarantine/ci.yaml.quarantined", err)
	}

	// Verify no duplicate trigger overlap: scan all active .yml/.yaml in workflows dir
	// and ensure only one has both pull_request and push-to-main triggers.
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("read workflows dir: %v", err)
	}

	mainlineCICount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		wfBody, err := os.ReadFile(filepath.Join(workflowDir, name))
		if err != nil {
			continue
		}
		wfContent := string(wfBody)

		// A "mainline CI" workflow is one that triggers on both pull_request
		// and push to main, AND runs contract tests or unit tests.
		hasPR := strings.Contains(wfContent, "pull_request")
		hasPushMain := strings.Contains(wfContent, "branches:") && strings.Contains(wfContent, "main")
		hasTests := strings.Contains(wfContent, "go test") || strings.Contains(wfContent, "contract")

		if hasPR && hasPushMain && hasTests {
			mainlineCICount++
		}
	}

	if mainlineCICount != 1 {
		t.Fatalf("expected exactly 1 mainline CI workflow with PR+push-to-main+tests triggers, got %d", mainlineCICount)
	}
}
