package mcp

import (
	"strings"
	"testing"
)

func TestBuildExecutorRolloutPlanDeterministic(t *testing.T) {
	t.Parallel()

	plan, err := BuildExecutorRolloutPlan([]string{
		"google_drive",
		"google_calendar",
		"google_drive",
		"tesla",
	}, ExecutorRolloutConfig{
		ImageRepository: "105914556507.dkr.ecr.us-east-1.amazonaws.com/brevio-executor",
		ImageTag:        "v9.2.0",
		Namespace:       "brevio-system",
		ReleaseName:     "brevio-executor",
		ChartPath:       "./helm/BREVIO-executor",
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if plan.ServerCount != 3 {
		t.Fatalf("expected 3 unique servers, got %d", plan.ServerCount)
	}
	if got, want := strings.Join(plan.ServerIDs, ","), "google_calendar,google_drive,tesla"; got != want {
		t.Fatalf("unexpected server ordering: got %q want %q", got, want)
	}
	if got, want := plan.AllowlistCSV, "google_calendar,google_drive,tesla"; got != want {
		t.Fatalf("unexpected allowlist: got %q want %q", got, want)
	}
	if !strings.Contains(plan.ValuesYAML, "MCP_RUNTIME_MODE") {
		t.Fatalf("values yaml missing MCP_RUNTIME_MODE: %s", plan.ValuesYAML)
	}
	if !strings.Contains(plan.ValuesYAML, "MCP_SERVER_ALLOWLIST") {
		t.Fatalf("values yaml missing MCP_SERVER_ALLOWLIST: %s", plan.ValuesYAML)
	}
	if !strings.Contains(plan.DockerBuildLine, "SERVICE=executor") {
		t.Fatalf("docker build command missing SERVICE=executor: %s", plan.DockerBuildLine)
	}
	if !strings.Contains(plan.HelmUpgradeLine, "helm upgrade --install brevio-executor ./helm/BREVIO-executor") {
		t.Fatalf("helm upgrade command unexpected: %s", plan.HelmUpgradeLine)
	}
}

func TestBuildExecutorRolloutPlanRejectsInvalidServerID(t *testing.T) {
	t.Parallel()

	_, err := BuildExecutorRolloutPlan([]string{"google_calendar", "bad id"}, ExecutorRolloutConfig{
		ImageRepository: "example.com/repo",
		ImageTag:        "v1",
		Namespace:       "default",
		ReleaseName:     "brevio-executor",
		ChartPath:       "./helm/BREVIO-executor",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid server_id") {
		t.Fatalf("expected invalid server_id error, got %v", err)
	}
}

func TestBuildExecutorRolloutPlanRequiresConfigFields(t *testing.T) {
	t.Parallel()

	_, err := BuildExecutorRolloutPlan([]string{"google_calendar"}, ExecutorRolloutConfig{
		ImageRepository: "",
		ImageTag:        "v1",
		Namespace:       "default",
		ReleaseName:     "brevio-executor",
		ChartPath:       "./helm/BREVIO-executor",
	})
	if err == nil || !strings.Contains(err.Error(), "image_repository is required") {
		t.Fatalf("expected image_repository validation error, got %v", err)
	}
}
