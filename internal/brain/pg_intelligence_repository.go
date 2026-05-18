package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// CriticReflectorRow represents a persisted critic/reflector output.
type CriticReflectorRow struct {
	ID                   string             `json:"id"`
	WorkspaceID          string             `json:"workspace_id"`
	WorkflowRunID        string             `json:"workflow_run_id"`
	OverallScore         float64            `json:"overall_score"`
	DimensionScores      map[string]float64 `json:"dimension_scores"`
	Passed               bool               `json:"passed"`
	FailureModes         []string           `json:"failure_modes"`
	ImprovementDirective string             `json:"improvement_directive"`
	LessonCandidates     []LessonCandidate  `json:"lesson_candidates"`
	PatternDetected      bool               `json:"pattern_detected"`
	EscalateToFeedback   bool               `json:"escalate_to_feedback"`
	CreatedAt            time.Time          `json:"created_at"`
}

// MultiIntentRow represents a persisted multi-intent classification output.
type MultiIntentRow struct {
	ID                    string    `json:"id"`
	WorkspaceID           string    `json:"workspace_id"`
	IngressTurnID         string    `json:"ingress_turn_id"`
	RawInput              string    `json:"raw_input"`
	Intents               string    `json:"intents"` // JSON
	CompoundRequest       bool      `json:"compound_request"`
	OverallConfidence     float64   `json:"overall_confidence"`
	RequiresDecomposition bool      `json:"requires_decomposition"`
	CreatedAt             time.Time `json:"created_at"`
}

// UncertaintyRow represents a persisted uncertainty assessment.
type UncertaintyRow struct {
	ID                   string   `json:"id"`
	WorkspaceID          string   `json:"workspace_id"`
	IngressTurnID        string   `json:"ingress_turn_id"`
	RawConfidence        float64  `json:"raw_confidence"`
	CalibratedConfidence *float64 `json:"calibrated_confidence"`
	UncertaintyLabel     string   `json:"uncertainty_label"`
	ShouldQualify        bool     `json:"should_qualify"`
	QualifierPhrase      string   `json:"qualifier_phrase"`
}

// CalibrationRow represents a persisted confidence calibration entry.
type CalibrationRow struct {
	ID                 string     `json:"id"`
	WorkspaceID        string     `json:"workspace_id"`
	Domain             string     `json:"domain"`
	Status             string     `json:"status"`
	PredictedConfidence float64   `json:"predicted_confidence"`
	ActualAccuracy     *float64   `json:"actual_accuracy"`
	CalibrationError   *float64   `json:"calibration_error"`
	SampleCount        int        `json:"sample_count"`
	LastCalibratedAt   *time.Time `json:"last_calibrated_at"`
}

// InterruptionRuleRow represents a persisted interruption rule.
type InterruptionRuleRow struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	TriggerType     string `json:"trigger_type"`
	Priority        int    `json:"priority"`
	ConditionText   string `json:"condition_text"`
	CooldownMinutes int    `json:"cooldown_minutes"`
	IsActive        bool   `json:"is_active"`
}

// InterruptionLogRow represents a persisted interruption evaluation.
type InterruptionLogRow struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	RuleID      string  `json:"rule_id"`
	Urgency     float64 `json:"urgency"`
	Message     string  `json:"message"`
	WasSurfaced bool    `json:"was_surfaced"`
}

// IntelligenceRepository provides DB-backed persistence for v10.2 intelligence outputs.
type IntelligenceRepository interface {
	// Critic/Reflector
	PersistCriticReflector(ctx context.Context, row CriticReflectorRow) error
	GetCriticReflectorByWorkflow(ctx context.Context, workspaceID, workflowRunID string) ([]CriticReflectorRow, error)

	// Multi-Intent
	PersistMultiIntent(ctx context.Context, row MultiIntentRow) error
	GetMultiIntentByTurn(ctx context.Context, workspaceID, ingressTurnID string) (*MultiIntentRow, error)

	// Uncertainty
	PersistUncertainty(ctx context.Context, row UncertaintyRow) error
	GetUncertaintyByTurn(ctx context.Context, workspaceID, ingressTurnID string) (*UncertaintyRow, error)

	// Confidence Calibration
	UpsertCalibration(ctx context.Context, workspaceID, domain string, predictedConfidence float64, wasCorrect bool) error
	RecalibrateAll(ctx context.Context, workspaceID string) error
	GetCalibration(ctx context.Context, workspaceID, domain string) (*CalibrationRow, error)

	// Interruption Rules
	UpsertInterruptionRule(ctx context.Context, rule InterruptionRuleRow) error
	GetActiveRules(ctx context.Context, workspaceID string) ([]InterruptionRuleRow, error)
	LogInterruption(ctx context.Context, entry InterruptionLogRow) error

	// Reasoning Chain Audit
	PersistReasoningStep(ctx context.Context, workspaceID, workflowRunID string, stepIndex int, reasoningType, inputSummary, outputSummary string, confidence float64, durationMs int) error
}

// PgIntelligenceRepository implements IntelligenceRepository backed by pgx.
type PgIntelligenceRepository struct {
	q database.Querier
}

// NewPgIntelligenceRepository creates a new PgIntelligenceRepository.
func NewPgIntelligenceRepository(q database.Querier) *PgIntelligenceRepository {
	return &PgIntelligenceRepository{q: q}
}

// PersistCriticReflector writes a critic/reflector output to the DB.
func (r *PgIntelligenceRepository) PersistCriticReflector(ctx context.Context, row CriticReflectorRow) error {
	dimJSON, _ := json.Marshal(row.DimensionScores)
	lessonsJSON, _ := json.Marshal(row.LessonCandidates)

	_, err := r.q.Exec(ctx,
		`INSERT INTO critic_reflector_outputs
		   (workspace_id, workflow_run_id, overall_score, dimension_scores, passed,
		    failure_modes, improvement_directive, lesson_candidates, pattern_detected, escalate_to_feedback)
		 VALUES ($1::uuid, $2, $3, $4::jsonb, $5, $6, $7, $8::jsonb, $9, $10)`,
		row.WorkspaceID, row.WorkflowRunID, row.OverallScore, string(dimJSON), row.Passed,
		row.FailureModes, row.ImprovementDirective, string(lessonsJSON), row.PatternDetected, row.EscalateToFeedback,
	)
	if err != nil {
		return fmt.Errorf("persist critic reflector: %w", err)
	}
	return nil
}

// GetCriticReflectorByWorkflow retrieves critic/reflector outputs for a workflow run.
func (r *PgIntelligenceRepository) GetCriticReflectorByWorkflow(ctx context.Context, workspaceID, workflowRunID string) ([]CriticReflectorRow, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, workflow_run_id, overall_score, dimension_scores, passed,
		        failure_modes, improvement_directive, lesson_candidates, pattern_detected, escalate_to_feedback, created_at
		 FROM critic_reflector_outputs
		 WHERE workspace_id = $1::uuid AND workflow_run_id = $2
		 ORDER BY created_at`,
		workspaceID, workflowRunID,
	)
	if err != nil {
		return nil, fmt.Errorf("get critic reflector: %w", err)
	}
	defer rows.Close()

	var result []CriticReflectorRow
	for rows.Next() {
		var cr CriticReflectorRow
		var dimJSON, lessonsJSON string
		var failureModes []string
		if err := rows.Scan(&cr.ID, &cr.WorkspaceID, &cr.WorkflowRunID, &cr.OverallScore,
			&dimJSON, &cr.Passed, &failureModes, &cr.ImprovementDirective,
			&lessonsJSON, &cr.PatternDetected, &cr.EscalateToFeedback, &cr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan critic reflector: %w", err)
		}
		_ = json.Unmarshal([]byte(dimJSON), &cr.DimensionScores)
		_ = json.Unmarshal([]byte(lessonsJSON), &cr.LessonCandidates)
		cr.FailureModes = failureModes
		result = append(result, cr)
	}
	return result, rows.Err()
}

// PersistMultiIntent writes a multi-intent classification output.
func (r *PgIntelligenceRepository) PersistMultiIntent(ctx context.Context, row MultiIntentRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO multi_intent_outputs
		   (workspace_id, ingress_turn_id, raw_input, intents, compound_request, overall_confidence, requires_decomposition)
		 VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5, $6, $7)`,
		row.WorkspaceID, row.IngressTurnID, row.RawInput, row.Intents,
		row.CompoundRequest, row.OverallConfidence, row.RequiresDecomposition,
	)
	if err != nil {
		return fmt.Errorf("persist multi intent: %w", err)
	}
	return nil
}

// GetMultiIntentByTurn retrieves multi-intent output for a turn.
func (r *PgIntelligenceRepository) GetMultiIntentByTurn(ctx context.Context, workspaceID, ingressTurnID string) (*MultiIntentRow, error) {
	var row MultiIntentRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, ingress_turn_id, raw_input, intents, compound_request, overall_confidence, requires_decomposition
		 FROM multi_intent_outputs
		 WHERE workspace_id = $1::uuid AND ingress_turn_id = $2::uuid
		 ORDER BY created_at DESC LIMIT 1`,
		workspaceID, ingressTurnID,
	).Scan(&row.ID, &row.WorkspaceID, &row.IngressTurnID, &row.RawInput, &row.Intents,
		&row.CompoundRequest, &row.OverallConfidence, &row.RequiresDecomposition)
	if err != nil {
		return nil, fmt.Errorf("get multi intent: %w", err)
	}
	return &row, nil
}

// PersistUncertainty writes an uncertainty assessment.
func (r *PgIntelligenceRepository) PersistUncertainty(ctx context.Context, row UncertaintyRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO uncertainty_assessments
		   (workspace_id, ingress_turn_id, raw_confidence, calibrated_confidence, uncertainty_label, should_qualify, qualifier_phrase)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`,
		row.WorkspaceID, row.IngressTurnID, row.RawConfidence, row.CalibratedConfidence,
		row.UncertaintyLabel, row.ShouldQualify, row.QualifierPhrase,
	)
	if err != nil {
		return fmt.Errorf("persist uncertainty: %w", err)
	}
	return nil
}

// GetUncertaintyByTurn retrieves uncertainty assessment for a turn.
func (r *PgIntelligenceRepository) GetUncertaintyByTurn(ctx context.Context, workspaceID, ingressTurnID string) (*UncertaintyRow, error) {
	var row UncertaintyRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, ingress_turn_id, raw_confidence, calibrated_confidence,
		        uncertainty_label, should_qualify, qualifier_phrase
		 FROM uncertainty_assessments
		 WHERE workspace_id = $1::uuid AND ingress_turn_id = $2::uuid
		 ORDER BY created_at DESC LIMIT 1`,
		workspaceID, ingressTurnID,
	).Scan(&row.ID, &row.WorkspaceID, &row.IngressTurnID, &row.RawConfidence, &row.CalibratedConfidence,
		&row.UncertaintyLabel, &row.ShouldQualify, &row.QualifierPhrase)
	if err != nil {
		return nil, fmt.Errorf("get uncertainty: %w", err)
	}
	return &row, nil
}

// UpsertCalibration records a calibration sample and updates the calibration entry.
func (r *PgIntelligenceRepository) UpsertCalibration(ctx context.Context, workspaceID, domain string, predictedConfidence float64, wasCorrect bool) error {
	// Ensure the calibration entry exists.
	_, err := r.q.Exec(ctx,
		`INSERT INTO confidence_calibration (workspace_id, domain, predicted_confidence, sample_count, status)
		 VALUES ($1::uuid, $2, $3, 0, 'uncalibrated')
		 ON CONFLICT (workspace_id, domain) DO NOTHING`,
		workspaceID, domain, predictedConfidence,
	)
	if err != nil {
		return fmt.Errorf("ensure calibration entry: %w", err)
	}

	// Get calibration ID.
	var calID string
	err = r.q.QueryRow(ctx,
		`SELECT id FROM confidence_calibration WHERE workspace_id = $1::uuid AND domain = $2`,
		workspaceID, domain,
	).Scan(&calID)
	if err != nil {
		return fmt.Errorf("get calibration id: %w", err)
	}

	// Insert sample.
	_, err = r.q.Exec(ctx,
		`INSERT INTO confidence_calibration_samples (workspace_id, calibration_id, predicted_confidence, was_correct)
		 VALUES ($1::uuid, $2::uuid, $3, $4)`,
		workspaceID, calID, predictedConfidence, wasCorrect,
	)
	if err != nil {
		return fmt.Errorf("insert calibration sample: %w", err)
	}

	// Update sample count and actual accuracy.
	_, err = r.q.Exec(ctx,
		`UPDATE confidence_calibration SET
		   sample_count = (SELECT COUNT(*) FROM confidence_calibration_samples WHERE calibration_id = $2::uuid),
		   actual_accuracy = (SELECT AVG(CASE WHEN was_correct THEN 1.0 ELSE 0.0 END) FROM confidence_calibration_samples WHERE calibration_id = $2::uuid),
		   calibration_error = ABS(predicted_confidence - COALESCE((SELECT AVG(CASE WHEN was_correct THEN 1.0 ELSE 0.0 END) FROM confidence_calibration_samples WHERE calibration_id = $2::uuid), 0)),
		   status = CASE WHEN (SELECT COUNT(*) FROM confidence_calibration_samples WHERE calibration_id = $2::uuid) >= 5 THEN 'calibrating'::calibration_status ELSE status END,
		   updated_at = now()
		 WHERE workspace_id = $1::uuid AND id = $2::uuid`,
		workspaceID, calID,
	)
	if err != nil {
		return fmt.Errorf("update calibration stats: %w", err)
	}
	return nil
}

// RecalibrateAll recalibrates all domains for a workspace with sufficient samples.
func (r *PgIntelligenceRepository) RecalibrateAll(ctx context.Context, workspaceID string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE confidence_calibration SET
		   actual_accuracy = sub.accuracy,
		   calibration_error = ABS(predicted_confidence - sub.accuracy),
		   status = 'calibrated'::calibration_status,
		   last_calibrated_at = now(),
		   updated_at = now()
		 FROM (
		   SELECT calibration_id, AVG(CASE WHEN was_correct THEN 1.0 ELSE 0.0 END) AS accuracy
		   FROM confidence_calibration_samples
		   WHERE workspace_id = $1::uuid
		   GROUP BY calibration_id
		   HAVING COUNT(*) >= 5
		 ) sub
		 WHERE confidence_calibration.id = sub.calibration_id AND confidence_calibration.workspace_id = $1::uuid`,
		workspaceID,
	)
	if err != nil {
		return fmt.Errorf("recalibrate all: %w", err)
	}
	return nil
}

// GetCalibration retrieves a calibration entry for a workspace/domain.
func (r *PgIntelligenceRepository) GetCalibration(ctx context.Context, workspaceID, domain string) (*CalibrationRow, error) {
	var row CalibrationRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, domain, status, predicted_confidence, actual_accuracy,
		        calibration_error, sample_count, last_calibrated_at
		 FROM confidence_calibration
		 WHERE workspace_id = $1::uuid AND domain = $2`,
		workspaceID, domain,
	).Scan(&row.ID, &row.WorkspaceID, &row.Domain, &row.Status, &row.PredictedConfidence,
		&row.ActualAccuracy, &row.CalibrationError, &row.SampleCount, &row.LastCalibratedAt)
	if err != nil {
		return nil, fmt.Errorf("get calibration: %w", err)
	}
	return &row, nil
}

// UpsertInterruptionRule creates or updates an interruption rule.
func (r *PgIntelligenceRepository) UpsertInterruptionRule(ctx context.Context, rule InterruptionRuleRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO interruption_rules (workspace_id, trigger_type, priority, condition_text, cooldown_minutes, is_active)
		 VALUES ($1::uuid, $2::interruption_trigger_type, $3, $4, $5, $6)`,
		rule.WorkspaceID, rule.TriggerType, rule.Priority, rule.ConditionText, rule.CooldownMinutes, rule.IsActive,
	)
	if err != nil {
		return fmt.Errorf("upsert interruption rule: %w", err)
	}
	return nil
}

// GetActiveRules retrieves all active interruption rules for a workspace.
func (r *PgIntelligenceRepository) GetActiveRules(ctx context.Context, workspaceID string) ([]InterruptionRuleRow, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, trigger_type, priority, condition_text, cooldown_minutes, is_active
		 FROM interruption_rules
		 WHERE workspace_id = $1::uuid AND is_active = true
		 ORDER BY priority DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("get active rules: %w", err)
	}
	defer rows.Close()

	var result []InterruptionRuleRow
	for rows.Next() {
		var rule InterruptionRuleRow
		if err := rows.Scan(&rule.ID, &rule.WorkspaceID, &rule.TriggerType, &rule.Priority,
			&rule.ConditionText, &rule.CooldownMinutes, &rule.IsActive); err != nil {
			return nil, fmt.Errorf("scan interruption rule: %w", err)
		}
		result = append(result, rule)
	}
	return result, rows.Err()
}

// LogInterruption persists an interruption evaluation result.
func (r *PgIntelligenceRepository) LogInterruption(ctx context.Context, entry InterruptionLogRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO interruption_log (workspace_id, rule_id, urgency, message, was_surfaced)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		entry.WorkspaceID, entry.RuleID, entry.Urgency, entry.Message, entry.WasSurfaced,
	)
	if err != nil {
		return fmt.Errorf("log interruption: %w", err)
	}
	return nil
}

// PersistReasoningStep writes a reasoning chain audit entry.
func (r *PgIntelligenceRepository) PersistReasoningStep(ctx context.Context, workspaceID, workflowRunID string, stepIndex int, reasoningType, inputSummary, outputSummary string, confidence float64, durationMs int) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO reasoning_chain_audit
		   (workspace_id, workflow_run_id, step_index, reasoning_type, input_summary, output_summary, confidence, duration_ms)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)`,
		workspaceID, workflowRunID, stepIndex, reasoningType, inputSummary, outputSummary, confidence, durationMs,
	)
	if err != nil {
		return fmt.Errorf("persist reasoning step: %w", err)
	}
	return nil
}
