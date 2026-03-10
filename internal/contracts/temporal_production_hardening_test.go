package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTemporalWorkerUsesNewWorkerWithDeps verifies the temporal-worker entrypoint
// uses NewWorkerWithDeps for dependency injection (P5-T001).
func TestTemporalWorkerUsesNewWorkerWithDeps(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	mainFile := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, mainFile, []string{
		"NewWorkerWithDeps",
		"ActivityDeps",
		"outbox.NewService",
		"pgxpool.New",
	})
}

// TestTemporalActivitiesMethodBased verifies that the Activities struct has
// method-based activity implementations (no standalone wrapper functions).
func TestTemporalActivitiesMethodBased(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	activitiesFile := filepath.Join(root, "internal", "temporal", "activities.go")

	assertFileContainsTokens(t, activitiesFile, []string{
		"NewActivitiesWithProdDeps",
		"ActivityDeps",
		"OutboxDispatcher",
		"func (a *Activities) FetchPendingOutboxActivity",
		"func (a *Activities) DispatchOutboxEntryActivity",
		"func (a *Activities) InitVoiceSessionActivity",
		"func (a *Activities) ClusterCorrectionsActivity",
		"func (a *Activities) ExecuteFederationSyncActivity",
	})

	// Verify no standalone wrapper functions remain.
	body, err := os.ReadFile(activitiesFile)
	if err != nil {
		t.Fatalf("read activities.go: %v", err)
	}
	content := string(body)
	standalonePattern := "\nfunc ValidateEnvelopeActivity("
	if strings.Contains(content, standalonePattern) {
		t.Fatal("activities.go still contains standalone ValidateEnvelopeActivity function")
	}
	standalonePattern2 := "\nfunc FetchPendingOutboxActivity("
	if strings.Contains(content, standalonePattern2) {
		t.Fatal("activities.go still contains standalone FetchPendingOutboxActivity function")
	}
}

// TestTemporalWorkerRegistersV91Workflows verifies the worker registers
// V9.1 soft intelligence workflows (P5-T003).
func TestTemporalWorkerRegistersV91Workflows(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	workerFile := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerFile, []string{
		"workflows.TrustScoringWorkflow",
		"workflows.GoalProgressWorkflow",
		"workflows.LearningConsolidationWorkflow",
		"workflows.DailyIntrospectionWorkflow",
		"workflows.DailyLogCaptureWorkflow",
		"workflows.CrossRepoAnalysisWorkflow",
		"workflows.MissionControlRefreshWorkflow",
		"workflows.CapabilityExplorationWorkflow",
		"v91.CollectTrustMetricsActivity",
		"v91.ComputeTrustScoreActivity",
	})
}

// TestOutboxDispatchWorkflowHasDLQ verifies the OutboxDispatchWorkflow
// includes DLQ tracking in its result type (P5-T002).
func TestOutboxDispatchWorkflowHasDLQ(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	workflowsFile := filepath.Join(root, "internal", "temporal", "workflows.go")
	assertFileContainsTokens(t, workflowsFile, []string{
		"TotalDLQ",
		"dlqCount",
		"ComputeDeterministicBackoff",
	})
}

// TestOutboxActivityUsesOutboxService verifies FetchPendingOutboxActivity
// calls outbox.Service.FetchPending in production mode (P5-T002).
func TestOutboxActivityUsesOutboxService(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	activitiesFile := filepath.Join(root, "internal", "temporal", "activities.go")
	assertFileContainsTokens(t, activitiesFile, []string{
		"a.outboxSvc.FetchPending",
		"a.outboxSvc.MarkDispatched",
		"a.outboxSvc.MarkFailed",
		"a.outboxDispatcher.Dispatch",
	})
}

// TestV91StandaloneWrappersRemoved verifies standalone wrapper functions
// have been removed from V91 activities.
func TestV91StandaloneWrappersRemoved(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	v91File := filepath.Join(root, "internal", "workflows", "v91_activities.go")

	body, err := os.ReadFile(v91File)
	if err != nil {
		t.Fatalf("read v91_activities.go: %v", err)
	}
	content := string(body)

	standalones := []string{
		"\nfunc CollectTrustMetricsActivity(",
		"\nfunc ComputeTrustScoreActivity(",
		"\nfunc ReviewGoalsActivity(",
		"\nfunc RefreshWidgetsActivity(",
	}
	for _, pattern := range standalones {
		if strings.Contains(content, pattern) {
			t.Fatalf("v91_activities.go still contains standalone wrapper: %s", pattern)
		}
	}
}

// TestV91WorkflowsUseMethodReferences verifies V91 workflows use
// method-based activity references (var a *V91Activities pattern).
func TestV91WorkflowsUseMethodReferences(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	v91File := filepath.Join(root, "internal", "workflows", "v91_workflows.go")
	assertFileContainsTokens(t, v91File, []string{
		"var a *V91Activities",
		"a.CollectTrustMetricsActivity",
		"a.ComputeTrustScoreActivity",
		"a.ReviewGoalsActivity",
		"a.ConsolidateFeedbackActivity",
		"a.SummarizeDailyActivity",
		"a.AnalyzeCapabilityGapsActivity",
	})
}

// TestTemporalWorkerProductionPath verifies the temporal-worker main.go
// has both production (DATABASE_URL) and degraded paths.
func TestTemporalWorkerProductionPath(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	mainFile := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, mainFile, []string{
		`os.Getenv("DATABASE_URL")`,
		"deps.Pool = pool",
		"deps.OutboxSvc = outbox.NewService(pool)",
		"breviotemporal.NewWorkerWithDeps",
	})
}
