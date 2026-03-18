// Package e2e_test contains the end-to-end test suite for the Brevio backend.
// Run with: CI_E2E=true go test ./tests/e2e/... -v -timeout 300s
package e2e_test

import (
	"os"
	"testing"

	"github.com/brevio/brevio/tests/e2e/harness"
)

func TestE2ESuite(t *testing.T) {
	if os.Getenv("CI_E2E") != "true" {
		t.Skip("Set CI_E2E=true to run E2E tests")
	}

	h := harness.New(t)
	defer h.Teardown()

	t.Run("E2E001_HealthEndpoint", func(t *testing.T) { E2E001_HealthEndpoint(t, h) })
	t.Run("E2E002_ComplianceDSRCreate", func(t *testing.T) { E2E002_ComplianceDSRCreate(t, h) })
	t.Run("E2E003_AdminControlPlane", func(t *testing.T) { E2E003_AutonomyGateBlocksWrite(t, h) })
	t.Run("E2E004_IPIGuardBlocksInjection", func(t *testing.T) { E2E004_IPIGuardBlocksInjection(t, h) })
	t.Run("E2E005_WorkspaceIsolation", func(t *testing.T) { E2E005_WorkspaceIsolation(t, h) })
	t.Run("E2E006_DSRIntakeEndpoint", func(t *testing.T) { E2E006_DLQRetryExhaustion(t, h) })
	t.Run("E2E007_KillSwitchTerminates", func(t *testing.T) { E2E007_KillSwitchTerminates(t, h) })
	t.Run("E2E008_VoicePipeline", func(t *testing.T) { E2E008_VoicePipeline(t, h) })
	t.Run("E2E009_EpisodicMemory", func(t *testing.T) { E2E009_EpisodicMemory(t, h) })
	t.Run("E2E010_ProactiveCalendarConflict", func(t *testing.T) { E2E010_ProactiveCalendarConflict(t, h) })
	t.Run("E2E011_DSRErasureCascade", func(t *testing.T) { E2E011_DSRErasureCascade(t, h) })
	t.Run("E2E012_WorldModelSurvivesRestart", func(t *testing.T) { E2E012_WorldModelSurvivesRestart(t, h) })
}
