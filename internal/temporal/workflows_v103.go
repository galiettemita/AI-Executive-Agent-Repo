package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// V10.3 Cognitive Intelligence Workflows — deterministic, replay-safe.
// No nondeterministic calls (time, rand, uuid, os) in workflow code.

// NightlyConsolidationWorkflow runs nightly memory consolidation (COG-10).
// Schedule: daily at 02:00 UTC via Temporal cron.
func NightlyConsolidationWorkflow(ctx workflow.Context, input NightlyConsolidationInput) (*NightlyConsolidationResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Run consolidation.
	var consolidationResult *RunConsolidationResult
	err := workflow.ExecuteActivity(ctx, (*Activities).RunConsolidationActivity, RunConsolidationInput{
		WorkspaceID: input.WorkspaceID,
		RunDate:     input.RunDate,
	}).Get(ctx, &consolidationResult)
	if err != nil {
		return &NightlyConsolidationResult{Status: "failed"}, err
	}

	// Step 2: Decay beliefs with low observations.
	var decayResult *DecayBeliefsResult
	err = workflow.ExecuteActivity(ctx, (*Activities).DecayBeliefsActivity, DecayBeliefsInput{
		WorkspaceID: input.WorkspaceID,
		DecayRate:   input.DecayRate,
	}).Get(ctx, &decayResult)
	if err != nil {
		return &NightlyConsolidationResult{
			Status:           "partial",
			EpisodesAnalyzed: consolidationResult.EpisodesAnalyzed,
			PatternsFound:    consolidationResult.PatternsExtracted,
		}, err
	}

	// Step 3: Recalculate metacognitive tiers.
	var metacogResult *RecalculateMetacognitiveResult
	err = workflow.ExecuteActivity(ctx, (*Activities).RecalculateMetacognitiveActivity, RecalculateMetacognitiveInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &metacogResult)
	if err != nil {
		return &NightlyConsolidationResult{
			Status:           "partial",
			EpisodesAnalyzed: consolidationResult.EpisodesAnalyzed,
			PatternsFound:    consolidationResult.PatternsExtracted,
			BeliefsDecayed:   decayResult.Decayed,
		}, err
	}

	return &NightlyConsolidationResult{
		Status:           "complete",
		EpisodesAnalyzed: consolidationResult.EpisodesAnalyzed,
		PatternsFound:    consolidationResult.PatternsExtracted,
		PatternsPromoted: consolidationResult.PatternsPromoted,
		BeliefsDecayed:   decayResult.Decayed,
		DomainsUpdated:   metacogResult.DomainsUpdated,
	}, nil
}

// WeeklyDriftDetectionWorkflow runs weekly behavioral drift detection (COG-11).
// Schedule: weekly on Sundays at 03:00 UTC via Temporal cron.
func WeeklyDriftDetectionWorkflow(ctx workflow.Context, input WeeklyDriftDetectionInput) (*WeeklyDriftDetectionResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var results []DriftMetricResult

	for _, metric := range input.Metrics {
		var driftResult *DetectDriftResult
		err := workflow.ExecuteActivity(ctx, (*Activities).DetectDriftActivity, DetectDriftInput{
			WorkspaceID:  input.WorkspaceID,
			Metric:       metric.Name,
			RecentValues: metric.Values,
		}).Get(ctx, &driftResult)
		if err != nil {
			results = append(results, DriftMetricResult{
				Metric:   metric.Name,
				Severity: "error",
			})
			continue
		}
		results = append(results, DriftMetricResult{
			Metric:     metric.Name,
			Detected:   driftResult.Detected,
			Divergence: driftResult.Divergence,
			Severity:   driftResult.Severity,
		})
	}

	driftsDetected := 0
	for _, r := range results {
		if r.Detected {
			driftsDetected++
		}
	}

	return &WeeklyDriftDetectionResult{
		MetricsChecked: len(input.Metrics),
		DriftsDetected: driftsDetected,
		Results:        results,
		Status:         "complete",
	}, nil
}

// HeuristicUpdateWorkflow updates a System 1 heuristic and recalculates metacognitive tiers (COG-01 + COG-03).
func HeuristicUpdateWorkflow(ctx workflow.Context, input HeuristicUpdateWorkflowInput) (*HeuristicUpdateWorkflowResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var heuristicResult *UpdateHeuristicResult
	err := workflow.ExecuteActivity(ctx, (*Activities).UpdateHeuristicActivity, UpdateHeuristicInput{
		WorkspaceID: input.WorkspaceID,
		HeuristicID: input.HeuristicID,
		Pattern:     input.Pattern,
		Response:    input.Response,
		Domain:      input.Domain,
		Success:     input.Success,
	}).Get(ctx, &heuristicResult)
	if err != nil {
		return &HeuristicUpdateWorkflowResult{Status: "failed"}, err
	}

	// Recalculate metacognitive tiers after heuristic update.
	var metacogResult *RecalculateMetacognitiveResult
	err = workflow.ExecuteActivity(ctx, (*Activities).RecalculateMetacognitiveActivity, RecalculateMetacognitiveInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &metacogResult)
	if err != nil {
		return &HeuristicUpdateWorkflowResult{
			Status:  "partial",
			Updated: heuristicResult.Updated,
		}, err
	}

	return &HeuristicUpdateWorkflowResult{
		Status:         "complete",
		Updated:        heuristicResult.Updated,
		Persisted:      heuristicResult.Persisted,
		DomainsUpdated: metacogResult.DomainsUpdated,
	}, nil
}

// BeliefMaintenanceWorkflow updates beliefs and evaluates cognitive signals (COG-05 + T9.4).
func BeliefMaintenanceWorkflow(ctx workflow.Context, input BeliefMaintenanceWorkflowInput) (*BeliefMaintenanceWorkflowResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var beliefResult *UpdateBeliefResult
	err := workflow.ExecuteActivity(ctx, (*Activities).UpdateBeliefActivity, UpdateBeliefInput{
		WorkspaceID:   input.WorkspaceID,
		PreferenceDim: input.PreferenceDim,
		ContextKey:    input.ContextKey,
		ContextValue:  input.ContextValue,
		Mean:          input.Mean,
		Variance:      input.Variance,
		Observations:  input.Observations,
	}).Get(ctx, &beliefResult)
	if err != nil {
		return &BeliefMaintenanceWorkflowResult{Status: "failed"}, err
	}

	// Evaluate cognitive signals to decide next action.
	var cogResult *EvaluateCognitiveSignalsResult
	err = workflow.ExecuteActivity(ctx, (*Activities).EvaluateCognitiveSignalsActivity, EvaluateCognitiveSignalsInput{
		WorkspaceID:    input.WorkspaceID,
		TaskComplexity: input.TaskComplexity,
		ErrorRate:      input.ErrorRate,
		LatencyMs:      input.LatencyMs,
		Intent:         input.Intent,
	}).Get(ctx, &cogResult)
	if err != nil {
		return &BeliefMaintenanceWorkflowResult{
			Status:    "partial",
			Persisted: beliefResult.Persisted,
		}, err
	}

	return &BeliefMaintenanceWorkflowResult{
		Status:         "complete",
		Persisted:      beliefResult.Persisted,
		CognitiveState: cogResult.CognitiveState,
		StrategyAction: cogResult.StrategyAction,
		ShouldClarify:  cogResult.ShouldClarify,
		ShouldEscalate: cogResult.ShouldEscalate,
		ConveneCouncil: cogResult.ConveneCouncil,
	}, nil
}

// CognitiveSignalProcessingWorkflow records implicit signals and evaluates cognitive state (COG-07 + COG-09 + T9.4).
func CognitiveSignalProcessingWorkflow(ctx workflow.Context, input CognitiveSignalProcessingInput) (*CognitiveSignalProcessingResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Record implicit signal.
	var signalResult *RecordImplicitSignalResult
	err := workflow.ExecuteActivity(ctx, (*Activities).RecordImplicitSignalActivity, RecordImplicitSignalInput{
		WorkspaceID:   input.WorkspaceID,
		IngressTurnID: input.IngressTurnID,
		SignalType:    input.SignalType,
		RawSignalData: input.RawSignalData,
		InferredPref:  input.InferredPref,
		InferredValue: input.InferredValue,
		Confidence:    input.Confidence,
	}).Get(ctx, &signalResult)
	if err != nil {
		return &CognitiveSignalProcessingResult{Status: "failed"}, err
	}

	// Step 2: Evaluate cognitive signals.
	var cogResult *EvaluateCognitiveSignalsResult
	err = workflow.ExecuteActivity(ctx, (*Activities).EvaluateCognitiveSignalsActivity, EvaluateCognitiveSignalsInput{
		WorkspaceID:    input.WorkspaceID,
		TaskComplexity: input.TaskComplexity,
		ErrorRate:      input.ErrorRate,
		LatencyMs:      input.LatencyMs,
		Intent:         input.Intent,
	}).Get(ctx, &cogResult)
	if err != nil {
		return &CognitiveSignalProcessingResult{
			Status:          "partial",
			SignalPersisted: signalResult.Persisted,
		}, err
	}

	// Step 3: If clarification is needed, persist candidate.
	clarificationPersisted := false
	if cogResult.ShouldClarify && input.ClarificationText != "" {
		var clarResult *PersistClarificationResult
		err = workflow.ExecuteActivity(ctx, (*Activities).PersistClarificationActivity, PersistClarificationInput{
			WorkspaceID:   input.WorkspaceID,
			IngressTurnID: input.IngressTurnID,
			QuestionText:  input.ClarificationText,
			EstimatedGain: input.ClarificationGain,
			Disambiguates: input.Disambiguates,
			WasSelected:   false,
		}).Get(ctx, &clarResult)
		if err == nil {
			clarificationPersisted = clarResult.Persisted
		}
	}

	return &CognitiveSignalProcessingResult{
		Status:                 "complete",
		SignalPersisted:        signalResult.Persisted,
		CognitiveState:         cogResult.CognitiveState,
		StrategyAction:         cogResult.StrategyAction,
		ShouldClarify:          cogResult.ShouldClarify,
		ShouldEscalate:         cogResult.ShouldEscalate,
		ConveneCouncil:         cogResult.ConveneCouncil,
		ClarificationPersisted: clarificationPersisted,
	}, nil
}

// Workflow input/output types.

type NightlyConsolidationInput struct {
	WorkspaceID string  `json:"workspace_id"`
	RunDate     string  `json:"run_date"`
	DecayRate   float64 `json:"decay_rate"`
}

type NightlyConsolidationResult struct {
	Status           string `json:"status"`
	EpisodesAnalyzed int    `json:"episodes_analyzed"`
	PatternsFound    int    `json:"patterns_found"`
	PatternsPromoted int    `json:"patterns_promoted"`
	BeliefsDecayed   int    `json:"beliefs_decayed"`
	DomainsUpdated   int    `json:"domains_updated"`
}

type WeeklyDriftDetectionInput struct {
	WorkspaceID string          `json:"workspace_id"`
	Metrics     []DriftMetric   `json:"metrics"`
}

type DriftMetric struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

type WeeklyDriftDetectionResult struct {
	MetricsChecked int                 `json:"metrics_checked"`
	DriftsDetected int                 `json:"drifts_detected"`
	Results        []DriftMetricResult `json:"results"`
	Status         string              `json:"status"`
}

type DriftMetricResult struct {
	Metric     string  `json:"metric"`
	Detected   bool    `json:"detected"`
	Divergence float64 `json:"divergence"`
	Severity   string  `json:"severity"`
}

type HeuristicUpdateWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
	HeuristicID string `json:"heuristic_id"`
	Pattern     string `json:"pattern"`
	Response    string `json:"response"`
	Domain      string `json:"domain"`
	Success     bool   `json:"success"`
}

type HeuristicUpdateWorkflowResult struct {
	Status         string `json:"status"`
	Updated        bool   `json:"updated"`
	Persisted      bool   `json:"persisted"`
	DomainsUpdated int    `json:"domains_updated"`
}

type BeliefMaintenanceWorkflowInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	PreferenceDim string  `json:"preference_dim"`
	ContextKey    string  `json:"context_key"`
	ContextValue  string  `json:"context_value"`
	Mean          float64 `json:"mean"`
	Variance      float64 `json:"variance"`
	Observations  int     `json:"observations"`
	TaskComplexity float64 `json:"task_complexity"`
	ErrorRate      float64 `json:"error_rate"`
	LatencyMs      float64 `json:"latency_ms"`
	Intent         string  `json:"intent"`
}

type BeliefMaintenanceWorkflowResult struct {
	Status         string `json:"status"`
	Persisted      bool   `json:"persisted"`
	CognitiveState string `json:"cognitive_state"`
	StrategyAction string `json:"strategy_action"`
	ShouldClarify  bool   `json:"should_clarify"`
	ShouldEscalate bool   `json:"should_escalate"`
	ConveneCouncil bool   `json:"convene_council"`
}

type CognitiveSignalProcessingInput struct {
	WorkspaceID       string   `json:"workspace_id"`
	IngressTurnID     string   `json:"ingress_turn_id"`
	SignalType        string   `json:"signal_type"`
	RawSignalData     string   `json:"raw_signal_data"`
	InferredPref      string   `json:"inferred_pref"`
	InferredValue     float64  `json:"inferred_value"`
	Confidence        float64  `json:"confidence"`
	TaskComplexity    float64  `json:"task_complexity"`
	ErrorRate         float64  `json:"error_rate"`
	LatencyMs         float64  `json:"latency_ms"`
	Intent            string   `json:"intent"`
	ClarificationText string   `json:"clarification_text"`
	ClarificationGain float64  `json:"clarification_gain"`
	Disambiguates     []string `json:"disambiguates"`
}

type CognitiveSignalProcessingResult struct {
	Status                 string `json:"status"`
	SignalPersisted        bool   `json:"signal_persisted"`
	CognitiveState         string `json:"cognitive_state"`
	StrategyAction         string `json:"strategy_action"`
	ShouldClarify          bool   `json:"should_clarify"`
	ShouldEscalate         bool   `json:"should_escalate"`
	ConveneCouncil         bool   `json:"convene_council"`
	ClarificationPersisted bool   `json:"clarification_persisted"`
}
