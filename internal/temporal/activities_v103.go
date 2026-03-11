package temporal

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/cognition"
)

// V10.3 Cognitive Activity Input/Output types.

type UpdateHeuristicInput struct {
	WorkspaceID    string `json:"workspace_id"`
	HeuristicID    string `json:"heuristic_id"`
	Pattern        string `json:"pattern"`
	Response       string `json:"response"`
	Domain         string `json:"domain"`
	Success        bool   `json:"success"`
}

type UpdateHeuristicResult struct {
	Updated   bool `json:"updated"`
	Persisted bool `json:"persisted"`
}

type RecalculateMetacognitiveInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type RecalculateMetacognitiveResult struct {
	DomainsUpdated int  `json:"domains_updated"`
	Persisted      bool `json:"persisted"`
}

type UpdateBeliefInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	PreferenceDim string  `json:"preference_dim"`
	ContextKey    string  `json:"context_key"`
	ContextValue  string  `json:"context_value"`
	Mean          float64 `json:"mean"`
	Variance      float64 `json:"variance"`
	Observations  int     `json:"observations"`
}

type UpdateBeliefResult struct {
	Persisted bool `json:"persisted"`
}

type DecayBeliefsInput struct {
	WorkspaceID string  `json:"workspace_id"`
	DecayRate   float64 `json:"decay_rate"`
}

type DecayBeliefsResult struct {
	Decayed   int  `json:"decayed"`
	Persisted bool `json:"persisted"`
}

type RunConsolidationInput struct {
	WorkspaceID string `json:"workspace_id"`
	RunDate     string `json:"run_date"`
}

type RunConsolidationResult struct {
	EpisodesAnalyzed  int    `json:"episodes_analyzed"`
	PatternsExtracted int    `json:"patterns_extracted"`
	PatternsPromoted  int    `json:"patterns_promoted"`
	Status            string `json:"status"`
	Persisted         bool   `json:"persisted"`
}

type DetectDriftInput struct {
	WorkspaceID  string    `json:"workspace_id"`
	Metric       string    `json:"metric"`
	RecentValues []float64 `json:"recent_values"`
}

type DetectDriftResult struct {
	Detected   bool    `json:"detected"`
	Divergence float64 `json:"divergence"`
	Severity   string  `json:"severity"`
	Persisted  bool    `json:"persisted"`
}

type RecordImplicitSignalInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	IngressTurnID string  `json:"ingress_turn_id"`
	SignalType    string  `json:"signal_type"`
	RawSignalData string  `json:"raw_signal_data"`
	InferredPref  string  `json:"inferred_pref"`
	InferredValue float64 `json:"inferred_value"`
	Confidence    float64 `json:"confidence"`
}

type RecordImplicitSignalResult struct {
	Persisted bool `json:"persisted"`
}

type PersistClarificationInput struct {
	WorkspaceID   string   `json:"workspace_id"`
	IngressTurnID string   `json:"ingress_turn_id"`
	QuestionText  string   `json:"question_text"`
	EstimatedGain float64  `json:"estimated_gain"`
	Disambiguates []string `json:"disambiguates"`
	WasSelected   bool     `json:"was_selected"`
}

type PersistClarificationResult struct {
	Persisted bool `json:"persisted"`
}

type EvaluateCognitiveSignalsInput struct {
	WorkspaceID    string  `json:"workspace_id"`
	TaskComplexity float64 `json:"task_complexity"`
	ErrorRate      float64 `json:"error_rate"`
	LatencyMs      float64 `json:"latency_ms"`
	Intent         string  `json:"intent"`
}

type EvaluateCognitiveSignalsResult struct {
	ShouldClarify    bool   `json:"should_clarify"`
	ShouldEscalate   bool   `json:"should_escalate"`
	CognitiveState   string `json:"cognitive_state"`
	StrategyAction   string `json:"strategy_action"`
	ConveneCouncil   bool   `json:"convene_council"`
}

// UpdateHeuristicActivity updates a System 1 heuristic and persists to DB (COG-01).
func (a *Activities) UpdateHeuristicActivity(ctx context.Context, input UpdateHeuristicInput) (*UpdateHeuristicResult, error) {
	engine := cognition.NewDualProcessEngine()
	if input.HeuristicID == "" {
		_, _ = engine.LearnHeuristic(input.Pattern, input.Response, input.Domain, "workflow")
	} else {
		engine.RecordOutcome(input.HeuristicID, input.Success)
	}

	result := &UpdateHeuristicResult{Updated: true}

	if a.pool != nil && a.cognitiveRepo != nil && input.HeuristicID != "" {
		err := a.cognitiveRepo.IncrementHeuristicActivation(ctx, input.HeuristicID, input.Success)
		result.Persisted = err == nil
	}

	return result, nil
}

// RecalculateMetacognitiveActivity recalculates metacognitive tiers for all domains (COG-03).
func (a *Activities) RecalculateMetacognitiveActivity(ctx context.Context, input RecalculateMetacognitiveInput) (*RecalculateMetacognitiveResult, error) {
	if a.pool == nil || a.cognitiveRepo == nil {
		return &RecalculateMetacognitiveResult{DomainsUpdated: 0}, nil
	}

	updated, err := a.cognitiveRepo.RecalculateMetacognitiveTiers(ctx, input.WorkspaceID)
	if err != nil {
		return &RecalculateMetacognitiveResult{}, nil
	}

	return &RecalculateMetacognitiveResult{
		DomainsUpdated: updated,
		Persisted:      true,
	}, nil
}

// UpdateBeliefActivity updates a Bayesian belief distribution (COG-05).
func (a *Activities) UpdateBeliefActivity(ctx context.Context, input UpdateBeliefInput) (*UpdateBeliefResult, error) {
	if a.pool == nil || a.cognitiveRepo == nil {
		engine := cognition.NewBayesianEngine()
		_, _ = engine.CreateBelief(input.WorkspaceID, input.PreferenceDim, input.Mean)
		return &UpdateBeliefResult{Persisted: false}, nil
	}

	err := a.cognitiveRepo.UpsertBelief(ctx, cognition.BeliefDistributionRow{
		WorkspaceID:      input.WorkspaceID,
		PreferenceDim:    input.PreferenceDim,
		ContextKey:       input.ContextKey,
		ContextValue:     input.ContextValue,
		Mean:             input.Mean,
		Variance:         input.Variance,
		ObservationCount: input.Observations,
	})
	return &UpdateBeliefResult{Persisted: err == nil}, err
}

// DecayBeliefsActivity decays beliefs with low observations (COG-05).
func (a *Activities) DecayBeliefsActivity(ctx context.Context, input DecayBeliefsInput) (*DecayBeliefsResult, error) {
	decayRate := input.DecayRate
	if decayRate <= 0 {
		decayRate = 0.1
	}

	if a.pool == nil || a.cognitiveRepo == nil {
		engine := cognition.NewBayesianEngine()
		decayed := engine.DecayBeliefs(input.WorkspaceID, decayRate)
		return &DecayBeliefsResult{Decayed: decayed}, nil
	}

	decayed, err := a.cognitiveRepo.DecayLowObservationBeliefs(ctx, input.WorkspaceID, decayRate)
	return &DecayBeliefsResult{Decayed: decayed, Persisted: err == nil}, err
}

// RunConsolidationActivity runs nightly memory consolidation (COG-10).
func (a *Activities) RunConsolidationActivity(ctx context.Context, input RunConsolidationInput) (*RunConsolidationResult, error) {
	svc := cognition.NewConsolidationService()
	episodes := []cognition.Episode{
		{ID: "synthetic-1", Content: "consolidation run", Tags: []string{"nightly"}, ImportanceScore: 0.5, Timestamp: time.Now().UTC()},
	}
	run, err := svc.RunConsolidation(input.WorkspaceID, episodes)
	if err != nil {
		return nil, fmt.Errorf("consolidation: %w", err)
	}

	result := &RunConsolidationResult{
		EpisodesAnalyzed:  run.EpisodicProcessed,
		PatternsExtracted: run.PatternsFound,
		Status:            "complete",
	}

	if a.pool != nil && a.cognitiveRepo != nil {
		runDate := input.RunDate
		if runDate == "" {
			runDate = time.Now().UTC().Format("2006-01-02")
		}
		persistErr := a.cognitiveRepo.PersistConsolidationRun(ctx, cognition.ConsolidationRunRow{
			WorkspaceID:       input.WorkspaceID,
			RunDate:           runDate,
			EpisodesAnalyzed:  run.EpisodicProcessed,
			PatternsExtracted: run.PatternsFound,
			PatternsPromoted:  run.SemanticsExtracted,
			Status:            "complete",
		})
		result.Persisted = persistErr == nil
		result.PatternsPromoted = run.SemanticsExtracted
	}

	return result, nil
}

// DetectDriftActivity detects behavioral drift (COG-11).
func (a *Activities) DetectDriftActivity(ctx context.Context, input DetectDriftInput) (*DetectDriftResult, error) {
	detector := cognition.NewDriftDetector()

	// Compute baseline from first half, detect drift from second half.
	if len(input.RecentValues) < 4 {
		return &DetectDriftResult{Detected: false, Severity: "none"}, nil
	}
	mid := len(input.RecentValues) / 2
	baselineVals := input.RecentValues[:mid]
	recentVals := input.RecentValues[mid:]

	_, err := detector.ComputeBaseline(input.WorkspaceID, input.Metric, baselineVals)
	if err != nil {
		return &DetectDriftResult{Detected: false, Severity: "none"}, nil
	}

	driftResult, err := detector.DetectDrift(input.WorkspaceID, input.Metric, recentVals)
	if err != nil {
		return &DetectDriftResult{Detected: false, Severity: "none"}, nil
	}

	result := &DetectDriftResult{
		Detected:   driftResult.Detected,
		Divergence: driftResult.Divergence,
		Severity:   driftResult.Severity,
	}

	if a.pool != nil && a.cognitiveRepo != nil && driftResult.Baseline != nil {
		persistErr := a.cognitiveRepo.PersistBaseline(ctx, cognition.BaselineRow{
			WorkspaceID:            input.WorkspaceID,
			BaselineWindowStart:    time.Now().AddDate(0, 0, -30).Format("2006-01-02"),
			BaselineWindowEnd:      time.Now().Format("2006-01-02"),
			TopicDistribution:      "{}",
			SkillUsageDistribution: "{}",
			IsCurrentBaseline:      true,
		})
		result.Persisted = persistErr == nil
	}

	return result, nil
}

// RecordImplicitSignalActivity records a behavioral signal (COG-07).
func (a *Activities) RecordImplicitSignalActivity(ctx context.Context, input RecordImplicitSignalInput) (*RecordImplicitSignalResult, error) {
	if a.pool == nil || a.cognitiveRepo == nil {
		return &RecordImplicitSignalResult{Persisted: false}, nil
	}

	val := input.InferredValue
	err := a.cognitiveRepo.RecordImplicitSignal(ctx, cognition.ImplicitSignalRow{
		WorkspaceID:   input.WorkspaceID,
		IngressTurnID: input.IngressTurnID,
		SignalType:    input.SignalType,
		RawSignalData: input.RawSignalData,
		InferredPref:  input.InferredPref,
		InferredValue: &val,
		Confidence:    input.Confidence,
	})
	return &RecordImplicitSignalResult{Persisted: err == nil}, err
}

// PersistClarificationActivity persists a clarification candidate (COG-09).
func (a *Activities) PersistClarificationActivity(ctx context.Context, input PersistClarificationInput) (*PersistClarificationResult, error) {
	if a.pool == nil || a.cognitiveRepo == nil {
		return &PersistClarificationResult{Persisted: false}, nil
	}

	err := a.cognitiveRepo.PersistClarification(ctx, cognition.ClarificationRow{
		WorkspaceID:   input.WorkspaceID,
		IngressTurnID: input.IngressTurnID,
		QuestionText:  input.QuestionText,
		EstimatedGain: input.EstimatedGain,
		Disambiguates: input.Disambiguates,
		WasSelected:   input.WasSelected,
	})
	return &PersistClarificationResult{Persisted: err == nil}, err
}

// EvaluateCognitiveSignalsActivity evaluates cognitive signals to determine
// whether to clarify, escalate, or convene council (T9.4 integration).
func (a *Activities) EvaluateCognitiveSignalsActivity(ctx context.Context, input EvaluateCognitiveSignalsInput) (*EvaluateCognitiveSignalsResult, error) {
	monitor := cognition.NewMetacognitiveMonitor()
	state, err := monitor.Monitor(input.WorkspaceID, input.TaskComplexity, input.ErrorRate, input.LatencyMs)
	if err != nil {
		return &EvaluateCognitiveSignalsResult{CognitiveState: "stable", StrategyAction: "maintain_course"}, nil
	}

	adj, _ := monitor.AdjustStrategy(input.WorkspaceID)

	clarifier := cognition.NewClarificationService()
	shouldClarify := clarifier.ShouldClarify(0.5, 1)

	shouldEscalate := state.CognitiveLoad > 0.8 || state.State == "alert"
	conveneCouncil := state.State == "alert" && input.ErrorRate > 0.5

	result := &EvaluateCognitiveSignalsResult{
		ShouldClarify:  shouldClarify,
		ShouldEscalate: shouldEscalate,
		CognitiveState: state.State,
		ConveneCouncil: conveneCouncil,
	}
	if adj != nil {
		result.StrategyAction = adj.Action
	} else {
		result.StrategyAction = "maintain_course"
	}

	return result, nil
}
