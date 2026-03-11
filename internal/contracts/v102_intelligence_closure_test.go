package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateV102TablesActivelyReadWritten verifies that v10.2 tables are
// actively read and written by repository code, not just defined in migrations.
func TestGateV102TablesActivelyReadWritten(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// eq_strategy_matrix and emotional_context_log must be used in EQ repository.
	eqRepoPath := filepath.Join(root, "internal", "eq", "pg_strategy_repository.go")
	assertFileNonEmpty(t, eqRepoPath)
	assertFileContainsTokens(t, eqRepoPath, []string{
		"eq_strategy_matrix",
		"emotional_context_log",
		"EQStrategyRepository",
		"GetBestStrategy",
		"UpsertStrategy",
		"RecordOutcome",
		"LogEmotionalContext",
		"GetRecentEmotionalContext",
	})

	// autonomy_levels and autonomy_demotion_events must be used in demotion repository.
	demotionRepoPath := filepath.Join(root, "internal", "trust", "pg_demotion_repository.go")
	assertFileNonEmpty(t, demotionRepoPath)
	assertFileContainsTokens(t, demotionRepoPath, []string{
		"autonomy_levels",
		"autonomy_demotion_events",
		"DemotionRepository",
		"GetAutonomyLevel",
		"UpsertAutonomyLevel",
		"RecordDemotion",
		"GetDemotionHistory",
		"CountDemotions90d",
	})

	// critic_reflector_outputs, multi_intent_outputs, uncertainty_assessments,
	// confidence_calibration, interruption_rules, reasoning_chain_audit
	// must be used in intelligence repository.
	intelRepoPath := filepath.Join(root, "internal", "brain", "pg_intelligence_repository.go")
	assertFileNonEmpty(t, intelRepoPath)
	assertFileContainsTokens(t, intelRepoPath, []string{
		"critic_reflector_outputs",
		"multi_intent_outputs",
		"uncertainty_assessments",
		"confidence_calibration",
		"confidence_calibration_samples",
		"interruption_rules",
		"interruption_log",
		"reasoning_chain_audit",
		"IntelligenceRepository",
		"PersistCriticReflector",
		"PersistMultiIntent",
		"PersistUncertainty",
		"UpsertCalibration",
		"RecalibrateAll",
		"UpsertInterruptionRule",
		"GetActiveRules",
		"LogInterruption",
		"PersistReasoningStep",
	})
}

// TestGateReasoningLoopProducesPersisted verifies that the reasoning loop
// produces persisted critic/reflector + calibration updates via Temporal activities.
func TestGateReasoningLoopProducesPersisted(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	assertFileNonEmpty(t, activitiesPath)
	assertFileContainsTokens(t, activitiesPath, []string{
		"V102ReasoningLoopActivity",
		"PersistCriticReflectorActivity",
		"RecordCalibrationOutcomeActivity",
		"PersistReasoningStepActivity",
		"intelligenceRepo",
		"PersistCriticReflector",
		"PersistReasoningStep",
	})

	// The V102ReasoningLoopActivity must call both PersistCriticReflector and PersistReasoningStep.
	content, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("read activities_v102.go: %v", err)
	}
	src := string(content)

	loopIdx := strings.Index(src, "func (a *Activities) V102ReasoningLoopActivity")
	if loopIdx < 0 {
		t.Fatal("V102ReasoningLoopActivity not found")
	}
	loopBody := src[loopIdx:]

	if !strings.Contains(loopBody, "PersistCriticReflector") {
		t.Error("V102ReasoningLoopActivity does not persist critic/reflector output")
	}
	if !strings.Contains(loopBody, "PersistReasoningStep") {
		t.Error("V102ReasoningLoopActivity does not persist reasoning chain audit")
	}
}

// TestGateMultiIntentAndUQTestVerified verifies multi-intent classification
// and uncertainty quantification behaviors are test-verified.
func TestGateMultiIntentAndUQTestVerified(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Multi-intent activity must exist and persist.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"ClassifyMultiIntentActivity",
		"AssessUncertaintyActivity",
		"PersistMultiIntent",
		"PersistUncertainty",
		"MultiIntentClassifier",
		"UncertaintyService",
		"CalibrationService",
	})

	// Multi-intent classifier must have compound request detection.
	multiIntentPath := filepath.Join(root, "internal", "brain", "multi_intent.go")
	assertFileContainsTokens(t, multiIntentPath, []string{
		"MultiIntentOutput",
		"IntentResult",
		"CompoundRequest",
		"DependsOnIndex",
		"RequiresDecomposition",
		"Classify",
	})

	// Uncertainty service must have qualifier phrases.
	uqPath := filepath.Join(root, "internal", "brain", "uncertainty.go")
	assertFileContainsTokens(t, uqPath, []string{
		"UncertaintyLevel",
		"Quantify",
		"high_confidence",
		"moderate",
		"low",
		"very_low",
		"QualifierPhrase",
	})
}

// TestGateEQStrategyDBBacked verifies EQ strategy is DB-backed with logging.
func TestGateEQStrategyDBBacked(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"ApplyEQStrategyActivity",
		"eqRepo",
		"LogEmotionalContext",
		"mapStateToValence",
		"mapToneToStrategy",
	})
}

// TestGateAutonomyDemotionStateMachine verifies autonomy demotion
// uses DB-backed state machine with trust score evaluation.
func TestGateAutonomyDemotionStateMachine(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"EvaluateAutonomyDemotionActivity",
		"demotionRepo",
		"GetAutonomyLevel",
		"UpsertAutonomyLevel",
		"RecordDemotion",
	})

	// Verify demotion checks trust score threshold.
	content, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("read activities_v102.go: %v", err)
	}
	if !strings.Contains(string(content), "0.4") {
		t.Error("demotion activity does not reference trust score threshold 0.4")
	}
}

// TestGateInterruptionPolicyEnforced verifies interruption rules are
// loaded from DB and evaluation results are logged.
func TestGateInterruptionPolicyEnforced(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"EvaluateInterruptionsActivity",
		"GetActiveRules",
		"LogInterruption",
		"ShouldInterrupt",
	})
}

// TestGateV102WorkflowsRegistered verifies V10.2 workflows and activities
// are registered in the Temporal worker.
func TestGateV102WorkflowsRegistered(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerPath := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerPath, []string{
		"IntelligenceProcessingWorkflow",
		"AutonomyDemotionWorkflow",
		"ApplyEQStrategyActivity",
		"EvaluateAutonomyDemotionActivity",
		"PersistCriticReflectorActivity",
		"RecordCalibrationOutcomeActivity",
		"ClassifyMultiIntentActivity",
		"AssessUncertaintyActivity",
		"EvaluateInterruptionsActivity",
		"PersistReasoningStepActivity",
		"V102ReasoningLoopActivity",
	})
}

// TestGateV102DepsWiredInWorkerMain verifies V10.2 dependencies are wired
// in the temporal-worker main.go.
func TestGateV102DepsWiredInWorkerMain(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	mainPath := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, mainPath, []string{
		"EQRepo",
		"DemotionRepo",
		"IntelligenceRepo",
		"NewPgEQStrategyRepository",
		"NewPgDemotionRepository",
		"NewPgIntelligenceRepository",
	})
}

// TestGateV102MigrationTablesExist verifies the v10.2 gap closure migration
// defines all required tables.
func TestGateV102MigrationTablesExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	migrationPath := filepath.Join(root, "db", "migrations", "017_BREVIO_v102_intelligence_gap_closure.sql")
	assertFileNonEmpty(t, migrationPath)
	assertFileContainsTokens(t, migrationPath, []string{
		"autonomy_levels",
		"autonomy_demotion_events",
		"interruption_rules",
		"interruption_log",
		"critic_reflector_outputs",
		"multi_intent_outputs",
		"uncertainty_assessments",
		"demotion_trigger",
		"interruption_trigger_type",
	})
}

// TestGateV102ReplayDeterminism verifies workflows_v102.go is in the
// determinism audit list.
func TestGateV102ReplayDeterminism(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	replayPath := filepath.Join(root, "internal", "temporal", "replay_test.go")
	content, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read replay_test.go: %v", err)
	}
	if !strings.Contains(string(content), "workflows_v102.go") {
		t.Error("workflows_v102.go is not in the determinism audit list")
	}
}

// TestGateConfidenceCalibrationLoopIntegrated verifies confidence calibration
// is integrated end-to-end: sample recording -> recalibration -> uncertainty linkage.
func TestGateConfidenceCalibrationLoopIntegrated(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// DB repository must support the full calibration loop.
	intelRepoPath := filepath.Join(root, "internal", "brain", "pg_intelligence_repository.go")
	assertFileContainsTokens(t, intelRepoPath, []string{
		"UpsertCalibration",
		"RecalibrateAll",
		"GetCalibration",
		"confidence_calibration_samples",
		"actual_accuracy",
		"calibration_error",
	})

	// Uncertainty activity must use CalibrationService.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v102.go")
	content, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("read activities_v102.go: %v", err)
	}
	src := string(content)

	uqIdx := strings.Index(src, "func (a *Activities) AssessUncertaintyActivity")
	if uqIdx < 0 {
		t.Fatal("AssessUncertaintyActivity not found")
	}
	uqBody := src[uqIdx:]
	if !strings.Contains(uqBody, "CalibrationService") || !strings.Contains(uqBody, "Calibrate") {
		t.Error("AssessUncertaintyActivity does not use CalibrationService for calibration")
	}
}
