package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/brain"
	"github.com/brevio/brevio/internal/compliance/eu_ai_act"
	"github.com/brevio/brevio/internal/eq"
	"github.com/brevio/brevio/internal/trust"
	"github.com/google/uuid"
)

// V10.2 Intelligence Activity Input/Output types.

type ApplyEQStrategyInput struct {
	WorkspaceID     string `json:"workspace_id"`
	UserID          string `json:"user_id"`
	SessionID       string `json:"session_id"`
	DetectedState   string `json:"detected_state"`
	CommStyle       string `json:"comm_style"`
	TriggerPattern  string `json:"trigger_pattern"`
	DetectedEmotion string `json:"detected_emotion"`
}

type ApplyEQStrategyResult struct {
	ToneDirective  string  `json:"tone_directive"`
	LengthModifier float64 `json:"length_modifier"`
	FormalityLevel int     `json:"formality_level"`
	OfferHelp      bool    `json:"offer_help"`
	Logged         bool    `json:"logged"`
}

type EvaluateAutonomyDemotionInput struct {
	WorkspaceID  string  `json:"workspace_id"`
	Domain       string  `json:"domain"`
	TrustScore   float64 `json:"trust_score"`
	FailureCount int     `json:"failure_count"`
}

type EvaluateAutonomyDemotionResult struct {
	Demoted       bool   `json:"demoted"`
	PreviousLevel int    `json:"previous_level"`
	NewLevel      int    `json:"new_level"`
	Reason        string `json:"reason"`
}

type PersistCriticReflectorInput struct {
	WorkspaceID          string             `json:"workspace_id"`
	WorkflowRunID        string             `json:"workflow_run_id"`
	OverallScore         float64            `json:"overall_score"`
	DimensionScores      map[string]float64 `json:"dimension_scores"`
	Passed               bool               `json:"passed"`
	FailureModes         []string           `json:"failure_modes"`
	ImprovementDirective string             `json:"improvement_directive"`
	LessonCandidates     []brain.LessonCandidate `json:"lesson_candidates"`
	PatternDetected      bool               `json:"pattern_detected"`
	EscalateToFeedback   bool               `json:"escalate_to_feedback"`
}

type PersistCriticReflectorResult struct {
	Persisted bool `json:"persisted"`
}

type RecordCalibrationOutcomeInput struct {
	WorkspaceID         string  `json:"workspace_id"`
	Domain              string  `json:"domain"`
	PredictedConfidence float64 `json:"predicted_confidence"`
	WasCorrect          bool    `json:"was_correct"`
}

type RecordCalibrationOutcomeResult struct {
	Recorded bool `json:"recorded"`
}

type ClassifyMultiIntentInput struct {
	WorkspaceID   string `json:"workspace_id"`
	IngressTurnID string `json:"ingress_turn_id"`
	RawInput      string `json:"raw_input"`
}

type ClassifyMultiIntentResult struct {
	CompoundRequest       bool    `json:"compound_request"`
	IntentCount           int     `json:"intent_count"`
	OverallConfidence     float64 `json:"overall_confidence"`
	RequiresDecomposition bool    `json:"requires_decomposition"`
	Persisted             bool    `json:"persisted"`
}

type AssessUncertaintyInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	IngressTurnID string  `json:"ingress_turn_id"`
	RawConfidence float64 `json:"raw_confidence"`
	Domain        string  `json:"domain"`
}

type AssessUncertaintyResult struct {
	CalibratedConfidence float64 `json:"calibrated_confidence"`
	UncertaintyLabel     string  `json:"uncertainty_label"`
	ShouldQualify        bool    `json:"should_qualify"`
	QualifierPhrase      string  `json:"qualifier_phrase"`
	Persisted            bool    `json:"persisted"`
}

type EvaluateInterruptionsInput struct {
	WorkspaceID string `json:"workspace_id"`
	ContextStr  string `json:"context_str"`
}

type EvaluateInterruptionsResult struct {
	CandidateCount int  `json:"candidate_count"`
	Interrupted    bool `json:"interrupted"`
	Logged         bool `json:"logged"`
}

type PersistReasoningStepInput struct {
	WorkspaceID    string  `json:"workspace_id"`
	WorkflowRunID  string  `json:"workflow_run_id"`
	StepIndex      int     `json:"step_index"`
	ReasoningType  string  `json:"reasoning_type"`
	InputSummary   string  `json:"input_summary"`
	OutputSummary  string  `json:"output_summary"`
	Confidence     float64 `json:"confidence"`
	DurationMs     int     `json:"duration_ms"`
}

type PersistReasoningStepResult struct {
	Persisted bool `json:"persisted"`
}

// ApplyEQStrategyActivity selects and applies an EQ strategy, logging the emotional context.
func (a *Activities) ApplyEQStrategyActivity(ctx context.Context, input ApplyEQStrategyInput) (*ApplyEQStrategyResult, error) {
	// Use in-memory EQ service for strategy selection.
	eqSvc := eq.NewEQStrategyService()
	result, err := eqSvc.ApplyStrategy(input.DetectedState, input.CommStyle)
	if err != nil {
		return &ApplyEQStrategyResult{
			ToneDirective:  "neutral",
			LengthModifier: 1.0,
			FormalityLevel: 3,
		}, nil
	}

	// Log emotional context to DB if pool available.
	if a.pool != nil && a.eqRepo != nil {
		logErr := a.eqRepo.LogEmotionalContext(ctx, eq.EmotionalContextEntry{
			WorkspaceID:     input.WorkspaceID,
			UserID:          input.UserID,
			SessionID:       input.SessionID,
			DetectedValence: mapStateToValence(input.DetectedState),
			Confidence:      0.7,
			Signals:         `[]`,
			StrategyApplied: mapToneToStrategy(result.ToneDirective),
		})
		if logErr != nil {
			// Log but don't fail the activity.
			_ = logErr
		}
		return &ApplyEQStrategyResult{
			ToneDirective:  result.ToneDirective,
			LengthModifier: result.LengthModifier,
			FormalityLevel: result.FormalityLevel,
			OfferHelp:      result.OfferHelp,
			Logged:         logErr == nil,
		}, nil
	}

	return &ApplyEQStrategyResult{
		ToneDirective:  result.ToneDirective,
		LengthModifier: result.LengthModifier,
		FormalityLevel: result.FormalityLevel,
		OfferHelp:      result.OfferHelp,
	}, nil
}

// EvaluateAutonomyDemotionActivity checks and applies autonomy demotion.
func (a *Activities) EvaluateAutonomyDemotionActivity(ctx context.Context, input EvaluateAutonomyDemotionInput) (*EvaluateAutonomyDemotionResult, error) {
	if a.pool == nil || a.demotionRepo == nil {
		// Degraded mode: use in-memory service.
		svc := trust.NewAutonomyDemotionService(trust.DemotionConfig{})
		svc.SetLevel(input.WorkspaceID, input.Domain, 4)
		event, _ := svc.CheckForDemotion(input.WorkspaceID, input.Domain, input.TrustScore, input.FailureCount)
		if event != nil {
			return &EvaluateAutonomyDemotionResult{
				Demoted:       true,
				PreviousLevel: event.PreviousLevel,
				NewLevel:      event.NewLevel,
				Reason:        event.Reason,
			}, nil
		}
		return &EvaluateAutonomyDemotionResult{Demoted: false}, nil
	}

	// Production mode: read current level, evaluate, persist.
	levelRow, err := a.demotionRepo.GetAutonomyLevel(ctx, input.WorkspaceID, input.Domain)
	currentLevel := 4
	if err == nil && levelRow != nil {
		currentLevel = levelRow.CurrentLevel
	}

	if currentLevel <= 0 {
		return &EvaluateAutonomyDemotionResult{Demoted: false, NewLevel: 0}, nil
	}

	shouldDemote := false
	reason := ""
	trigger := "trust_score"

	if input.TrustScore < 0.4 {
		shouldDemote = true
		reason = fmt.Sprintf("trust score %.2f below threshold 0.40", input.TrustScore)
	}
	if input.FailureCount >= 3 {
		shouldDemote = true
		trigger = "failure_count"
		if reason != "" {
			reason += "; "
		}
		reason += fmt.Sprintf("failure count %d >= 3", input.FailureCount)
	}

	if !shouldDemote {
		return &EvaluateAutonomyDemotionResult{Demoted: false, NewLevel: currentLevel}, nil
	}

	newLevel := currentLevel - 1
	if newLevel < 0 {
		newLevel = 0
	}

	// Persist new level.
	_ = a.demotionRepo.UpsertAutonomyLevel(ctx, input.WorkspaceID, input.Domain, newLevel, input.TrustScore)

	// Record demotion event.
	ts := input.TrustScore
	fc := input.FailureCount
	_ = a.demotionRepo.RecordDemotion(ctx, trust.DemotionEventRow{
		WorkspaceID:            input.WorkspaceID,
		Domain:                 input.Domain,
		PreviousLevel:          currentLevel,
		NewLevel:               newLevel,
		Trigger:                trigger,
		Reason:                 reason,
		TrustScoreAtDemotion:   &ts,
		FailureCountAtDemotion: &fc,
	})

	// EU AI Act Art. 73: record autonomy demotion as a serious incident.
	if a.euIncidentLog != nil {
		wsID, parseErr := uuid.Parse(input.WorkspaceID)
		if parseErr == nil {
			go func() {
				_, _ = a.euIncidentLog.RecordIncident(context.Background(), eu_ai_act.IncidentEntry{
					WorkspaceID:   wsID,
					IncidentType:  "autonomy_demotion",
					TriggerMetric: fmt.Sprintf("demoted_to=%d", newLevel),
					Severity:      "high",
					Description:   fmt.Sprintf("Workspace autonomy demoted from %d to %d: %s", currentLevel, newLevel, reason),
				})
			}()
		}
	}

	return &EvaluateAutonomyDemotionResult{
		Demoted:       true,
		PreviousLevel: currentLevel,
		NewLevel:      newLevel,
		Reason:        reason,
	}, nil
}

// PersistCriticReflectorActivity writes critic/reflector outputs to the DB.
func (a *Activities) PersistCriticReflectorActivity(ctx context.Context, input PersistCriticReflectorInput) (*PersistCriticReflectorResult, error) {
	if a.pool == nil || a.intelligenceRepo == nil {
		return &PersistCriticReflectorResult{Persisted: false}, nil
	}

	err := a.intelligenceRepo.PersistCriticReflector(ctx, brain.CriticReflectorRow{
		WorkspaceID:          input.WorkspaceID,
		WorkflowRunID:        input.WorkflowRunID,
		OverallScore:         input.OverallScore,
		DimensionScores:      input.DimensionScores,
		Passed:               input.Passed,
		FailureModes:         input.FailureModes,
		ImprovementDirective: input.ImprovementDirective,
		LessonCandidates:     input.LessonCandidates,
		PatternDetected:      input.PatternDetected,
		EscalateToFeedback:   input.EscalateToFeedback,
	})
	return &PersistCriticReflectorResult{Persisted: err == nil}, err
}

// RecordCalibrationOutcomeActivity records a calibration sample and updates stats.
func (a *Activities) RecordCalibrationOutcomeActivity(ctx context.Context, input RecordCalibrationOutcomeInput) (*RecordCalibrationOutcomeResult, error) {
	if a.pool == nil || a.intelligenceRepo == nil {
		// Degraded: use in-memory calibration.
		svc := brain.NewCalibrationService()
		_ = svc.RecordOutcome(input.WorkspaceID, input.PredictedConfidence, input.WasCorrect)
		return &RecordCalibrationOutcomeResult{Recorded: true}, nil
	}

	err := a.intelligenceRepo.UpsertCalibration(ctx, input.WorkspaceID, input.Domain, input.PredictedConfidence, input.WasCorrect)
	return &RecordCalibrationOutcomeResult{Recorded: err == nil}, err
}

// ClassifyMultiIntentActivity classifies input and persists multi-intent output.
func (a *Activities) ClassifyMultiIntentActivity(ctx context.Context, input ClassifyMultiIntentInput) (*ClassifyMultiIntentResult, error) {
	classifier := brain.NewMultiIntentClassifier()
	output := classifier.Classify(input.RawInput)

	result := &ClassifyMultiIntentResult{
		CompoundRequest:       output.CompoundRequest,
		IntentCount:           len(output.Intents),
		OverallConfidence:     output.OverallConfidence,
		RequiresDecomposition: output.RequiresDecomposition,
	}

	if a.pool != nil && a.intelligenceRepo != nil {
		intentsJSON, _ := json.Marshal(output.Intents)
		err := a.intelligenceRepo.PersistMultiIntent(ctx, brain.MultiIntentRow{
			WorkspaceID:           input.WorkspaceID,
			IngressTurnID:         input.IngressTurnID,
			RawInput:              input.RawInput,
			Intents:               string(intentsJSON),
			CompoundRequest:       output.CompoundRequest,
			OverallConfidence:     output.OverallConfidence,
			RequiresDecomposition: output.RequiresDecomposition,
		})
		result.Persisted = err == nil
	}

	return result, nil
}

// AssessUncertaintyActivity quantifies uncertainty, calibrates, and persists.
func (a *Activities) AssessUncertaintyActivity(ctx context.Context, input AssessUncertaintyInput) (*AssessUncertaintyResult, error) {
	uqSvc := brain.NewUncertaintyService()
	calSvc := brain.NewCalibrationService()

	// Calibrate the raw confidence.
	calibrated, err := calSvc.Calibrate(input.WorkspaceID, input.RawConfidence)
	if err != nil {
		calibrated = input.RawConfidence
	}

	// Quantify uncertainty on calibrated confidence.
	uLevel := uqSvc.Quantify(calibrated)

	result := &AssessUncertaintyResult{
		CalibratedConfidence: calibrated,
		UncertaintyLabel:     uLevel.Label,
		ShouldQualify:        uLevel.ShouldQualify,
		QualifierPhrase:      uLevel.QualifierPhrase,
	}

	if a.pool != nil && a.intelligenceRepo != nil {
		persistErr := a.intelligenceRepo.PersistUncertainty(ctx, brain.UncertaintyRow{
			WorkspaceID:          input.WorkspaceID,
			IngressTurnID:        input.IngressTurnID,
			RawConfidence:        input.RawConfidence,
			CalibratedConfidence: &calibrated,
			UncertaintyLabel:     uLevel.Label,
			ShouldQualify:        uLevel.ShouldQualify,
			QualifierPhrase:      uLevel.QualifierPhrase,
		})
		result.Persisted = persistErr == nil
	}

	return result, nil
}

// EvaluateInterruptionsActivity evaluates interruption rules and logs results.
func (a *Activities) EvaluateInterruptionsActivity(ctx context.Context, input EvaluateInterruptionsInput) (*EvaluateInterruptionsResult, error) {
	if a.pool == nil || a.intelligenceRepo == nil {
		// Degraded: use in-memory service.
		svc := brain.NewProactiveInterruptionService()
		candidates := svc.EvaluateInterruptions(input.WorkspaceID, input.ContextStr)
		interrupted := false
		for _, c := range candidates {
			if svc.ShouldInterrupt(c) {
				interrupted = true
				break
			}
		}
		return &EvaluateInterruptionsResult{
			CandidateCount: len(candidates),
			Interrupted:    interrupted,
		}, nil
	}

	// Production: load rules from DB, evaluate, log.
	rules, err := a.intelligenceRepo.GetActiveRules(ctx, input.WorkspaceID)
	if err != nil {
		return &EvaluateInterruptionsResult{}, nil
	}

	svc := brain.NewProactiveInterruptionService()
	for _, rule := range rules {
		_, _ = svc.AddRule(brain.InterruptionRule{
			WorkspaceID:     rule.WorkspaceID,
			TriggerType:     rule.TriggerType,
			Priority:        rule.Priority,
			Condition:       rule.ConditionText,
			CooldownMinutes: rule.CooldownMinutes,
		})
	}

	candidates := svc.EvaluateInterruptions(input.WorkspaceID, input.ContextStr)
	interrupted := false
	for _, c := range candidates {
		if svc.ShouldInterrupt(c) {
			interrupted = true
		}
		_ = a.intelligenceRepo.LogInterruption(ctx, brain.InterruptionLogRow{
			WorkspaceID: input.WorkspaceID,
			RuleID:      c.RuleID,
			Urgency:     c.Urgency,
			Message:     c.Message,
			WasSurfaced: svc.ShouldInterrupt(c),
		})
	}

	return &EvaluateInterruptionsResult{
		CandidateCount: len(candidates),
		Interrupted:    interrupted,
		Logged:         true,
	}, nil
}

// PersistReasoningStepActivity writes a reasoning chain audit entry.
func (a *Activities) PersistReasoningStepActivity(ctx context.Context, input PersistReasoningStepInput) (*PersistReasoningStepResult, error) {
	if a.pool == nil || a.intelligenceRepo == nil {
		return &PersistReasoningStepResult{Persisted: false}, nil
	}

	err := a.intelligenceRepo.PersistReasoningStep(ctx,
		input.WorkspaceID, input.WorkflowRunID, input.StepIndex,
		input.ReasoningType, input.InputSummary, input.OutputSummary,
		input.Confidence, input.DurationMs,
	)
	return &PersistReasoningStepResult{Persisted: err == nil}, err
}

// mapStateToValence converts a detected_state to an emotional_valence enum value.
func mapStateToValence(state string) string {
	switch state {
	case "frustrated", "angry":
		return "negative"
	case "anxious", "worried":
		return "slightly_negative"
	case "positive", "happy":
		return "positive"
	case "excited", "enthusiastic":
		return "very_positive"
	default:
		return "neutral"
	}
}

// mapToneToStrategy converts a tone directive to an eq_strategy enum value.
func mapToneToStrategy(tone string) string {
	switch tone {
	case "empathetic":
		return "empathetic_acknowledgment"
	case "direct":
		return "direct_action"
	case "encouraging":
		return "confidence_boost"
	case "gentle":
		return "gentle_redirect"
	default:
		return "supportive_framing"
	}
}

// V102ReasoningLoopActivity runs the full PLANNER→EXECUTOR→CRITIC→REFLECTOR loop
// and persists critic/reflector outputs + reasoning chain audit.
func (a *Activities) V102ReasoningLoopActivity(ctx context.Context, input struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkflowRunID string `json:"workflow_run_id"`
	Intent        string `json:"intent"`
	Confidence    float64 `json:"confidence"`
}) (*brain.LoopResult, error) {
	loop := brain.NewReasoningLoop(brain.ReasoningLoopConfig{
		QualityTarget: 0.8,
		MaxIterations: 3,
	})

	rc := &brain.ReasoningContext{
		WorkspaceID: input.WorkspaceID,
		Intent:      input.Intent,
		Confidence:  input.Confidence,
	}

	startTime := time.Now()
	result, err := loop.RunLoop(ctx, rc, 3)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("reasoning loop: %w", err)
	}

	// Persist critic output.
	if a.pool != nil && a.intelligenceRepo != nil && result != nil {
		criticSvc := brain.NewCriticReflectorService()
		trace := brain.ExecutionTrace{
			WorkspaceID: input.WorkspaceID,
			Intent:      input.Intent,
			PlanSteps:   len(result.FinalPlan.Steps),
			Duration:    duration,
		}
		if result.FinalResult != nil {
			for _, r := range result.FinalResult.Results {
				if r.Success {
					trace.Succeeded++
				} else {
					trace.Failed++
				}
				trace.ToolsUsed = append(trace.ToolsUsed, r.ToolKey)
			}
		}

		criticOutput, criticErr := criticSvc.Critique(trace)
		if criticErr == nil && criticOutput != nil {
			reflectorOutput, _ := criticSvc.Reflect(criticOutput, trace)

			var lessons []brain.LessonCandidate
			if reflectorOutput != nil {
				lessons = reflectorOutput.LessonCandidates
			}

			_ = a.intelligenceRepo.PersistCriticReflector(ctx, brain.CriticReflectorRow{
				WorkspaceID:          input.WorkspaceID,
				WorkflowRunID:        input.WorkflowRunID,
				OverallScore:         criticOutput.OverallScore,
				DimensionScores:      criticOutput.DimensionScores,
				Passed:               criticOutput.Passed,
				FailureModes:         criticOutput.FailureModes,
				ImprovementDirective: criticOutput.ImprovementDirective,
				LessonCandidates:     lessons,
				PatternDetected:      reflectorOutput != nil && reflectorOutput.PatternDetected,
				EscalateToFeedback:   reflectorOutput != nil && reflectorOutput.EscalateToFeedback,
			})
		}

		// Persist reasoning chain audit.
		_ = a.intelligenceRepo.PersistReasoningStep(ctx,
			input.WorkspaceID, input.WorkflowRunID, 0,
			"reasoning_loop", input.Intent, fmt.Sprintf("score=%.2f iterations=%d", result.CriticScore, result.Iterations),
			result.CriticScore, int(duration.Milliseconds()),
		)
	}

	return result, nil
}
