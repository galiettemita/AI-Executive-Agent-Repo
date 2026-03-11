package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateV103MigrationTablesExist verifies the v10.3 cognitive architecture
// migration defines all required COG tables.
func TestGateV103MigrationTablesExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	migrationPath := filepath.Join(root, "db", "migrations", "016_BREVIO_v103_cognitive_architecture.sql")
	assertFileNonEmpty(t, migrationPath)
	assertFileContainsTokens(t, migrationPath, []string{
		"system1_heuristics",
		"thought_graphs",
		"thought_nodes",
		"domain_performance_history",
		"belief_distributions",
		"implicit_behavior_signals",
		"case_library",
		"clarification_candidates",
		"consolidation_runs",
		"behavioral_baselines",
	})
}

// TestGateCOG01HeuristicPersistence verifies System 1 heuristic artifacts are
// written/read via pgx repository and Temporal activities.
func TestGateCOG01HeuristicPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"CognitiveRepository",
		"PgCognitiveRepository",
		"UpsertHeuristic",
		"GetTopHeuristics",
		"IncrementHeuristicActivation",
		"system1_heuristics",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileNonEmpty(t, activitiesPath)
	assertFileContainsTokens(t, activitiesPath, []string{
		"UpdateHeuristicActivity",
		"cognitiveRepo",
		"IncrementHeuristicActivation",
	})
}

// TestGateCOG02ThoughtGraphPersistence verifies thought graph artifacts are persisted.
func TestGateCOG02ThoughtGraphPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"PersistThoughtGraph",
		"PersistThoughtNode",
		"GetThoughtGraph",
		"thought_graphs",
		"thought_nodes",
		"ThoughtGraphRow",
		"ThoughtNodeRow",
	})
}

// TestGateCOG03MetacognitiveRecalculation verifies domain performance and
// metacognitive tier recalculation.
func TestGateCOG03MetacognitiveRecalculation(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"UpsertDomainPerformance",
		"GetDomainPerformance",
		"RecalculateMetacognitiveTiers",
		"domain_performance_history",
		"DomainPerformanceRow",
		"SHALLOW",
		"STANDARD",
		"DEEP",
		"EXHAUSTIVE",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"RecalculateMetacognitiveActivity",
		"RecalculateMetacognitiveTiers",
	})
}

// TestGateCOG05BayesianBeliefPersistence verifies Bayesian belief distribution
// persistence with decay support.
func TestGateCOG05BayesianBeliefPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"UpsertBelief",
		"GetBelief",
		"DecayLowObservationBeliefs",
		"belief_distributions",
		"BeliefDistributionRow",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"UpdateBeliefActivity",
		"DecayBeliefsActivity",
	})
}

// TestGateCOG07ImplicitSignalPersistence verifies implicit behavior signal recording.
func TestGateCOG07ImplicitSignalPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"RecordImplicitSignal",
		"GetUnprocessedSignals",
		"MarkSignalProcessed",
		"implicit_behavior_signals",
		"ImplicitSignalRow",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"RecordImplicitSignalActivity",
	})
}

// TestGateCOG08CaseLibraryPersistence verifies case library persistence.
func TestGateCOG08CaseLibraryPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"PersistCase",
		"IncrementCaseReuse",
		"case_library",
		"CaseLibraryRow",
	})
}

// TestGateCOG09ClarificationPersistence verifies clarification candidate persistence.
func TestGateCOG09ClarificationPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"PersistClarification",
		"GetClarifications",
		"clarification_candidates",
		"ClarificationRow",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"PersistClarificationActivity",
	})
}

// TestGateCOG10ConsolidationPersistence verifies nightly consolidation run persistence.
func TestGateCOG10ConsolidationPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"PersistConsolidationRun",
		"GetLatestConsolidationRun",
		"CompleteConsolidationRun",
		"consolidation_runs",
		"ConsolidationRunRow",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"RunConsolidationActivity",
	})
}

// TestGateCOG11DriftDetectionPersistence verifies behavioral drift detection persistence.
func TestGateCOG11DriftDetectionPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "cognition", "pg_cognitive_repository.go")
	assertFileContainsTokens(t, repoPath, []string{
		"PersistBaseline",
		"GetCurrentBaseline",
		"SetCurrentBaseline",
		"behavioral_baselines",
		"BaselineRow",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"DetectDriftActivity",
	})
}

// TestGateV103CognitiveWorkflowsExist verifies all cognitive workflows are defined.
func TestGateV103CognitiveWorkflowsExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workflowPath := filepath.Join(root, "internal", "temporal", "workflows_v103.go")
	assertFileNonEmpty(t, workflowPath)
	assertFileContainsTokens(t, workflowPath, []string{
		"NightlyConsolidationWorkflow",
		"WeeklyDriftDetectionWorkflow",
		"HeuristicUpdateWorkflow",
		"BeliefMaintenanceWorkflow",
		"CognitiveSignalProcessingWorkflow",
	})
}

// TestGateV103WorkflowsRegistered verifies cognitive workflows and activities
// are registered in the Temporal worker.
func TestGateV103WorkflowsRegistered(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerPath := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerPath, []string{
		"NightlyConsolidationWorkflow",
		"WeeklyDriftDetectionWorkflow",
		"HeuristicUpdateWorkflow",
		"BeliefMaintenanceWorkflow",
		"CognitiveSignalProcessingWorkflow",
		"UpdateHeuristicActivity",
		"RecalculateMetacognitiveActivity",
		"UpdateBeliefActivity",
		"DecayBeliefsActivity",
		"RunConsolidationActivity",
		"DetectDriftActivity",
		"RecordImplicitSignalActivity",
		"PersistClarificationActivity",
		"EvaluateCognitiveSignalsActivity",
	})
}

// TestGateV103DepsWiredInWorkerMain verifies the cognitive repository dependency
// is wired in the temporal-worker main.go.
func TestGateV103DepsWiredInWorkerMain(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	mainPath := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, mainPath, []string{
		"CognitiveRepo",
		"NewPgCognitiveRepository",
		"cognitionpkg",
	})
}

// TestGateV103ActivityDepsWired verifies the cognitiveRepo field is in ActivityDeps
// and Activities structs.
func TestGateV103ActivityDepsWired(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"CognitiveRepo",
		"cognitiveRepo",
		"cognition.CognitiveRepository",
	})
}

// TestGateV103ReplayDeterminism verifies workflows_v103.go is in the determinism
// audit list.
func TestGateV103ReplayDeterminism(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	replayPath := filepath.Join(root, "internal", "temporal", "replay_test.go")
	content, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read replay_test.go: %v", err)
	}
	if !strings.Contains(string(content), "workflows_v103.go") {
		t.Error("workflows_v103.go is not in the determinism audit list")
	}
}

// TestGateT94CognitiveSignalIntegration verifies the cognitive signal evaluation
// activity integrates metacognitive monitor, clarification, and strategy adjustment.
func TestGateT94CognitiveSignalIntegration(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v103.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"EvaluateCognitiveSignalsActivity",
		"MetacognitiveMonitor",
		"ClarificationService",
		"ShouldClarify",
		"ShouldEscalate",
		"ConveneCouncil",
		"AdjustStrategy",
		"CognitiveLoad",
	})
}

// TestGateV103CognitiveSignalWorkflow verifies the signal processing workflow
// chains implicit signal → evaluate → clarification persistence.
func TestGateV103CognitiveSignalWorkflow(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workflowPath := filepath.Join(root, "internal", "temporal", "workflows_v103.go")
	assertFileContainsTokens(t, workflowPath, []string{
		"CognitiveSignalProcessingWorkflow",
		"RecordImplicitSignalActivity",
		"EvaluateCognitiveSignalsActivity",
		"PersistClarificationActivity",
		"ShouldClarify",
	})
}
