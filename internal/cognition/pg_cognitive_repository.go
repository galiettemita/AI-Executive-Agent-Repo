package cognition

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// -----------------------------------------------------------------------
// Row types aligned with migration 016 tables
// -----------------------------------------------------------------------

// HeuristicRow represents a persisted system1_heuristics row (COG-01).
type HeuristicRow struct {
	ID                  string  `json:"id"`
	WorkspaceID         string  `json:"workspace_id"`
	SkillSequence       string  `json:"skill_sequence"`
	ResponseTemplate    string  `json:"response_template"`
	ActivationCount     int     `json:"activation_count"`
	SuccessRate30d      float64 `json:"success_rate_30d"`
	HeuristicConfidence float64 `json:"heuristic_confidence"`
	LastActivatedAt     *time.Time `json:"last_activated_at"`
}

// ThoughtGraphRow represents a persisted thought_graphs row (COG-02).
type ThoughtGraphRow struct {
	ID            string `json:"id"`
	WorkspaceID   string `json:"workspace_id"`
	IngressTurnID string `json:"ingress_turn_id"`
	NodeCount     int    `json:"node_count"`
	BranchCount   int    `json:"branch_count"`
	MergeCount    int    `json:"merge_count"`
	MaxDepth      int    `json:"max_depth"`
	Status        string `json:"status"`
}

// ThoughtNodeRow represents a persisted thought_nodes row (COG-02).
type ThoughtNodeRow struct {
	ID             string   `json:"id"`
	GraphID        string   `json:"graph_id"`
	WorkspaceID    string   `json:"workspace_id"`
	NodeType       string   `json:"node_type"`
	ThoughtContent string   `json:"thought_content"`
	CriticScore    *float64 `json:"critic_score"`
	IsPruned       bool     `json:"is_pruned"`
	Depth          int      `json:"depth"`
}

// DomainPerformanceRow represents a persisted domain_performance_history row (COG-03).
type DomainPerformanceRow struct {
	ID                     string   `json:"id"`
	WorkspaceID            string   `json:"workspace_id"`
	Domain                 string   `json:"domain"`
	SkillID                string   `json:"skill_id"`
	TotalExecutions30d     int      `json:"total_executions_30d"`
	SuccessfulExecutions30d int     `json:"successful_executions_30d"`
	UserCorrections30d     int      `json:"user_corrections_30d"`
	EmpiricalSuccessRate   *float64 `json:"empirical_success_rate"`
	MetacognitiveTierFloor string   `json:"metacognitive_tier_floor"`
	ConfidenceAdjustment   float64  `json:"confidence_adjustment"`
}

// BeliefDistributionRow represents a persisted belief_distributions row (COG-05).
type BeliefDistributionRow struct {
	ID               string  `json:"id"`
	WorkspaceID      string  `json:"workspace_id"`
	PreferenceDim    string  `json:"preference_dim"`
	ContextKey       string  `json:"context_key"`
	ContextValue     string  `json:"context_value"`
	Mean             float64 `json:"mean"`
	Variance         float64 `json:"variance"`
	ObservationCount int     `json:"observation_count"`
	PriorSource      string  `json:"prior_source"`
}

// ImplicitSignalRow represents a persisted implicit_behavior_signals row (COG-07).
type ImplicitSignalRow struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	IngressTurnID  string     `json:"ingress_turn_id"`
	SignalType     string     `json:"signal_type"`
	RawSignalData  string     `json:"raw_signal_data"`
	InferredPref   string     `json:"inferred_pref"`
	InferredValue  *float64   `json:"inferred_value"`
	Confidence     float64    `json:"confidence"`
	ProcessedAt    *time.Time `json:"processed_at"`
}

// CaseLibraryRow represents a persisted case_library row (COG-08).
type CaseLibraryRow struct {
	ID               string  `json:"id"`
	WorkspaceID      string  `json:"workspace_id"`
	ProblemSummary   string  `json:"problem_summary"`
	Domain           string  `json:"domain"`
	TaskGraphJSON    string  `json:"task_graph_json"`
	ExecutionSummary string  `json:"execution_summary"`
	OutcomeScore     float64 `json:"outcome_score"`
	IsNegativeCase   bool    `json:"is_negative_case"`
	ReuseCount       int     `json:"reuse_count"`
}

// ClarificationRow represents a persisted clarification_candidates row (COG-09).
type ClarificationRow struct {
	ID            string   `json:"id"`
	WorkspaceID   string   `json:"workspace_id"`
	IngressTurnID string   `json:"ingress_turn_id"`
	QuestionText  string   `json:"question_text"`
	EstimatedGain float64  `json:"estimated_gain"`
	Disambiguates []string `json:"disambiguates"`
	WasSelected   bool     `json:"was_selected"`
}

// ConsolidationRunRow represents a persisted consolidation_runs row (COG-10).
type ConsolidationRunRow struct {
	ID                string  `json:"id"`
	WorkspaceID       string  `json:"workspace_id"`
	RunDate           string  `json:"run_date"`
	EpisodesAnalyzed  int     `json:"episodes_analyzed"`
	PatternsExtracted int     `json:"patterns_extracted"`
	PatternsPromoted  int     `json:"patterns_promoted"`
	PatternsDiscarded int     `json:"patterns_discarded"`
	CostUSD           float64 `json:"cost_usd"`
	Status            string  `json:"status"`
}

// BaselineRow represents a persisted behavioral_baselines row (COG-11).
type BaselineRow struct {
	ID                     string  `json:"id"`
	WorkspaceID            string  `json:"workspace_id"`
	BaselineWindowStart    string  `json:"baseline_window_start"`
	BaselineWindowEnd      string  `json:"baseline_window_end"`
	TopicDistribution      string  `json:"topic_distribution"`
	SkillUsageDistribution string  `json:"skill_usage_distribution"`
	AvgMessageHour         *float64 `json:"avg_message_hour"`
	OverrideRate           *float64 `json:"override_rate"`
	CorrectionRate         *float64 `json:"correction_rate"`
	IsCurrentBaseline      bool    `json:"is_current_baseline"`
}

// -----------------------------------------------------------------------
// CognitiveRepository interface — covers all COG tables
// -----------------------------------------------------------------------

// CognitiveRepository persists all v10.3 cognitive artifacts.
type CognitiveRepository interface {
	// COG-01: System 1 heuristics
	UpsertHeuristic(ctx context.Context, row HeuristicRow) error
	GetTopHeuristics(ctx context.Context, workspaceID string, limit int) ([]HeuristicRow, error)
	IncrementHeuristicActivation(ctx context.Context, id string, success bool) error

	// COG-02: Thought graphs
	PersistThoughtGraph(ctx context.Context, row ThoughtGraphRow) error
	PersistThoughtNode(ctx context.Context, row ThoughtNodeRow) error
	GetThoughtGraph(ctx context.Context, workspaceID, ingressTurnID string) (*ThoughtGraphRow, error)

	// COG-03: Domain performance
	UpsertDomainPerformance(ctx context.Context, row DomainPerformanceRow) error
	GetDomainPerformance(ctx context.Context, workspaceID, domain string) (*DomainPerformanceRow, error)
	RecalculateMetacognitiveTiers(ctx context.Context, workspaceID string) (int, error)

	// COG-05: Bayesian beliefs
	UpsertBelief(ctx context.Context, row BeliefDistributionRow) error
	GetBelief(ctx context.Context, workspaceID, preferenceDim string) (*BeliefDistributionRow, error)
	DecayLowObservationBeliefs(ctx context.Context, workspaceID string, decayRate float64) (int, error)

	// COG-07: Implicit signals
	RecordImplicitSignal(ctx context.Context, row ImplicitSignalRow) error
	GetUnprocessedSignals(ctx context.Context, workspaceID string, limit int) ([]ImplicitSignalRow, error)
	MarkSignalProcessed(ctx context.Context, id string) error

	// COG-08: Case library
	PersistCase(ctx context.Context, row CaseLibraryRow) error
	IncrementCaseReuse(ctx context.Context, id string) error

	// COG-09: Clarification candidates
	PersistClarification(ctx context.Context, row ClarificationRow) error
	GetClarifications(ctx context.Context, workspaceID, ingressTurnID string) ([]ClarificationRow, error)

	// COG-10: Consolidation runs
	PersistConsolidationRun(ctx context.Context, row ConsolidationRunRow) error
	GetLatestConsolidationRun(ctx context.Context, workspaceID string) (*ConsolidationRunRow, error)
	CompleteConsolidationRun(ctx context.Context, id, status string, promoted, discarded int) error

	// COG-11: Behavioral baselines
	PersistBaseline(ctx context.Context, row BaselineRow) error
	GetCurrentBaseline(ctx context.Context, workspaceID string) (*BaselineRow, error)
	SetCurrentBaseline(ctx context.Context, workspaceID, baselineID string) error
}

// -----------------------------------------------------------------------
// PgCognitiveRepository — pgx implementation
// -----------------------------------------------------------------------

// PgCognitiveRepository implements CognitiveRepository backed by pgx.
type PgCognitiveRepository struct {
	q database.Querier
}

// NewPgCognitiveRepository creates a new PgCognitiveRepository.
func NewPgCognitiveRepository(q database.Querier) *PgCognitiveRepository {
	return &PgCognitiveRepository{q: q}
}

// --- COG-01: System 1 heuristics ---

func (r *PgCognitiveRepository) UpsertHeuristic(ctx context.Context, row HeuristicRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO system1_heuristics (workspace_id, pattern_embedding, skill_sequence, response_template, activation_count, success_rate_30d, heuristic_confidence)
		 VALUES ($1::uuid, $2::vector, $3::jsonb, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
		   activation_count = EXCLUDED.activation_count,
		   success_rate_30d = EXCLUDED.success_rate_30d,
		   heuristic_confidence = EXCLUDED.heuristic_confidence,
		   updated_at = now()`,
		row.WorkspaceID, make([]float32, 1536), row.SkillSequence, row.ResponseTemplate,
		row.ActivationCount, row.SuccessRate30d, row.HeuristicConfidence,
	)
	if err != nil {
		return fmt.Errorf("upsert heuristic: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetTopHeuristics(ctx context.Context, workspaceID string, limit int) ([]HeuristicRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, skill_sequence, COALESCE(response_template, ''), activation_count, success_rate_30d, heuristic_confidence, last_activated_at
		 FROM system1_heuristics
		 WHERE workspace_id = $1::uuid
		 ORDER BY heuristic_confidence DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get top heuristics: %w", err)
	}
	defer rows.Close()

	var result []HeuristicRow
	for rows.Next() {
		var h HeuristicRow
		if err := rows.Scan(&h.ID, &h.WorkspaceID, &h.SkillSequence, &h.ResponseTemplate,
			&h.ActivationCount, &h.SuccessRate30d, &h.HeuristicConfidence, &h.LastActivatedAt); err != nil {
			return nil, fmt.Errorf("scan heuristic: %w", err)
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

func (r *PgCognitiveRepository) IncrementHeuristicActivation(ctx context.Context, id string, success bool) error {
	successIncr := 0
	if success {
		successIncr = 1
	}
	_, err := r.q.Exec(ctx,
		`UPDATE system1_heuristics SET
		   activation_count = activation_count + 1,
		   success_rate_30d = CASE WHEN activation_count > 0
		     THEN (success_rate_30d * activation_count + $2) / (activation_count + 1)
		     ELSE $2::numeric END,
		   last_activated_at = now(),
		   updated_at = now()
		 WHERE id = $1::uuid`,
		id, successIncr,
	)
	if err != nil {
		return fmt.Errorf("increment heuristic: %w", err)
	}
	return nil
}

// --- COG-02: Thought graphs ---

func (r *PgCognitiveRepository) PersistThoughtGraph(ctx context.Context, row ThoughtGraphRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO thought_graphs (workspace_id, ingress_turn_id, node_count, branch_count, merge_count, max_depth, status)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::thought_graph_status)`,
		row.WorkspaceID, row.IngressTurnID, row.NodeCount, row.BranchCount, row.MergeCount, row.MaxDepth, row.Status,
	)
	if err != nil {
		return fmt.Errorf("persist thought graph: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) PersistThoughtNode(ctx context.Context, row ThoughtNodeRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO thought_nodes (graph_id, workspace_id, node_type, thought_content, critic_score, is_pruned, depth)
		 VALUES ($1::uuid, $2::uuid, $3::thought_node_type, $4, $5, $6, $7)`,
		row.GraphID, row.WorkspaceID, row.NodeType, row.ThoughtContent, row.CriticScore, row.IsPruned, row.Depth,
	)
	if err != nil {
		return fmt.Errorf("persist thought node: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetThoughtGraph(ctx context.Context, workspaceID, ingressTurnID string) (*ThoughtGraphRow, error) {
	var row ThoughtGraphRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, ingress_turn_id, node_count, branch_count, merge_count, max_depth, status
		 FROM thought_graphs
		 WHERE workspace_id = $1::uuid AND ingress_turn_id = $2::uuid
		 ORDER BY created_at DESC LIMIT 1`,
		workspaceID, ingressTurnID,
	).Scan(&row.ID, &row.WorkspaceID, &row.IngressTurnID, &row.NodeCount, &row.BranchCount, &row.MergeCount, &row.MaxDepth, &row.Status)
	if err != nil {
		return nil, fmt.Errorf("get thought graph: %w", err)
	}
	return &row, nil
}

// --- COG-03: Domain performance ---

func (r *PgCognitiveRepository) UpsertDomainPerformance(ctx context.Context, row DomainPerformanceRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO domain_performance_history (workspace_id, domain, skill_id, total_executions_30d, successful_executions_30d, user_corrections_30d, empirical_success_rate, metacognitive_tier_floor, confidence_adjustment)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (workspace_id, domain, skill_id) DO UPDATE SET
		   total_executions_30d = EXCLUDED.total_executions_30d,
		   successful_executions_30d = EXCLUDED.successful_executions_30d,
		   user_corrections_30d = EXCLUDED.user_corrections_30d,
		   empirical_success_rate = EXCLUDED.empirical_success_rate,
		   metacognitive_tier_floor = EXCLUDED.metacognitive_tier_floor,
		   confidence_adjustment = EXCLUDED.confidence_adjustment,
		   last_recalculated_at = now(),
		   updated_at = now()`,
		row.WorkspaceID, row.Domain, row.SkillID, row.TotalExecutions30d, row.SuccessfulExecutions30d,
		row.UserCorrections30d, row.EmpiricalSuccessRate, row.MetacognitiveTierFloor, row.ConfidenceAdjustment,
	)
	if err != nil {
		return fmt.Errorf("upsert domain performance: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetDomainPerformance(ctx context.Context, workspaceID, domain string) (*DomainPerformanceRow, error) {
	var row DomainPerformanceRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, domain, COALESCE(skill_id, ''), total_executions_30d, successful_executions_30d,
		        user_corrections_30d, empirical_success_rate, metacognitive_tier_floor, confidence_adjustment
		 FROM domain_performance_history
		 WHERE workspace_id = $1::uuid AND domain = $2
		 ORDER BY updated_at DESC LIMIT 1`,
		workspaceID, domain,
	).Scan(&row.ID, &row.WorkspaceID, &row.Domain, &row.SkillID, &row.TotalExecutions30d, &row.SuccessfulExecutions30d,
		&row.UserCorrections30d, &row.EmpiricalSuccessRate, &row.MetacognitiveTierFloor, &row.ConfidenceAdjustment)
	if err != nil {
		return nil, fmt.Errorf("get domain performance: %w", err)
	}
	return &row, nil
}

func (r *PgCognitiveRepository) RecalculateMetacognitiveTiers(ctx context.Context, workspaceID string) (int, error) {
	tag, err := r.q.Exec(ctx,
		`UPDATE domain_performance_history SET
		   metacognitive_tier_floor = CASE
		     WHEN empirical_success_rate >= 0.9 THEN 'SHALLOW'
		     WHEN empirical_success_rate >= 0.7 THEN 'STANDARD'
		     WHEN empirical_success_rate >= 0.5 THEN 'DEEP'
		     ELSE 'EXHAUSTIVE'
		   END,
		   confidence_adjustment = CASE
		     WHEN empirical_success_rate >= 0.9 THEN 0.0
		     WHEN empirical_success_rate >= 0.7 THEN -0.05
		     WHEN empirical_success_rate >= 0.5 THEN -0.10
		     ELSE -0.20
		   END,
		   last_recalculated_at = now(),
		   updated_at = now()
		 WHERE workspace_id = $1::uuid`,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("recalculate tiers: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// --- COG-05: Bayesian beliefs ---

func (r *PgCognitiveRepository) UpsertBelief(ctx context.Context, row BeliefDistributionRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO belief_distributions (workspace_id, preference_dim, context_key, context_value, mean, variance, observation_count, prior_source)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (workspace_id, preference_dim, context_key, context_value) DO UPDATE SET
		   mean = EXCLUDED.mean,
		   variance = EXCLUDED.variance,
		   observation_count = EXCLUDED.observation_count,
		   updated_at = now()`,
		row.WorkspaceID, row.PreferenceDim, row.ContextKey, row.ContextValue,
		row.Mean, row.Variance, row.ObservationCount, row.PriorSource,
	)
	if err != nil {
		return fmt.Errorf("upsert belief: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetBelief(ctx context.Context, workspaceID, preferenceDim string) (*BeliefDistributionRow, error) {
	var row BeliefDistributionRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, preference_dim, COALESCE(context_key, ''), COALESCE(context_value, ''),
		        mean, variance, observation_count, COALESCE(prior_source, '')
		 FROM belief_distributions
		 WHERE workspace_id = $1::uuid AND preference_dim = $2
		 LIMIT 1`,
		workspaceID, preferenceDim,
	).Scan(&row.ID, &row.WorkspaceID, &row.PreferenceDim, &row.ContextKey, &row.ContextValue,
		&row.Mean, &row.Variance, &row.ObservationCount, &row.PriorSource)
	if err != nil {
		return nil, fmt.Errorf("get belief: %w", err)
	}
	return &row, nil
}

func (r *PgCognitiveRepository) DecayLowObservationBeliefs(ctx context.Context, workspaceID string, decayRate float64) (int, error) {
	tag, err := r.q.Exec(ctx,
		`UPDATE belief_distributions SET
		   mean = mean * (1.0 - $2),
		   updated_at = now()
		 WHERE workspace_id = $1::uuid AND observation_count < 3`,
		workspaceID, decayRate,
	)
	if err != nil {
		return 0, fmt.Errorf("decay beliefs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// --- COG-07: Implicit signals ---

func (r *PgCognitiveRepository) RecordImplicitSignal(ctx context.Context, row ImplicitSignalRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO implicit_behavior_signals (workspace_id, ingress_turn_id, signal_type, raw_signal_data, inferred_pref, inferred_value, confidence)
		 VALUES ($1::uuid, $2::uuid, $3::implicit_signal_type, $4::jsonb, $5, $6, $7)`,
		row.WorkspaceID, row.IngressTurnID, row.SignalType, row.RawSignalData, row.InferredPref, row.InferredValue, row.Confidence,
	)
	if err != nil {
		return fmt.Errorf("record implicit signal: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetUnprocessedSignals(ctx context.Context, workspaceID string, limit int) ([]ImplicitSignalRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, ingress_turn_id, signal_type, raw_signal_data, COALESCE(inferred_pref, ''), inferred_value, confidence, processed_at
		 FROM implicit_behavior_signals
		 WHERE workspace_id = $1::uuid AND processed_at IS NULL
		 ORDER BY created_at ASC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get unprocessed signals: %w", err)
	}
	defer rows.Close()

	var result []ImplicitSignalRow
	for rows.Next() {
		var s ImplicitSignalRow
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.IngressTurnID, &s.SignalType, &s.RawSignalData,
			&s.InferredPref, &s.InferredValue, &s.Confidence, &s.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan signal: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *PgCognitiveRepository) MarkSignalProcessed(ctx context.Context, id string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE implicit_behavior_signals SET processed_at = now() WHERE id = $1::uuid`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark signal processed: %w", err)
	}
	return nil
}

// --- COG-08: Case library ---

func (r *PgCognitiveRepository) PersistCase(ctx context.Context, row CaseLibraryRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO case_library (workspace_id, problem_embedding, problem_summary, domain, task_graph_json, execution_summary, outcome_score, is_negative_case, reuse_count)
		 VALUES ($1::uuid, $2::vector, $3, $4, $5::jsonb, $6, $7, $8, $9)`,
		row.WorkspaceID, make([]float32, 1536), row.ProblemSummary, row.Domain,
		row.TaskGraphJSON, row.ExecutionSummary, row.OutcomeScore, row.IsNegativeCase, row.ReuseCount,
	)
	if err != nil {
		return fmt.Errorf("persist case: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) IncrementCaseReuse(ctx context.Context, id string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE case_library SET reuse_count = reuse_count + 1, last_reused_at = now() WHERE id = $1::uuid`,
		id,
	)
	if err != nil {
		return fmt.Errorf("increment case reuse: %w", err)
	}
	return nil
}

// --- COG-09: Clarification candidates ---

func (r *PgCognitiveRepository) PersistClarification(ctx context.Context, row ClarificationRow) error {
	disambJSON, _ := json.Marshal(row.Disambiguates)
	_, err := r.q.Exec(ctx,
		`INSERT INTO clarification_candidates (workspace_id, ingress_turn_id, question_text, estimated_gain, disambiguates, was_selected)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5::text[], $6)`,
		row.WorkspaceID, row.IngressTurnID, row.QuestionText, row.EstimatedGain, string(disambJSON), row.WasSelected,
	)
	if err != nil {
		return fmt.Errorf("persist clarification: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetClarifications(ctx context.Context, workspaceID, ingressTurnID string) ([]ClarificationRow, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, ingress_turn_id, question_text, estimated_gain, disambiguates, was_selected
		 FROM clarification_candidates
		 WHERE workspace_id = $1::uuid AND ingress_turn_id = $2::uuid
		 ORDER BY estimated_gain DESC`,
		workspaceID, ingressTurnID,
	)
	if err != nil {
		return nil, fmt.Errorf("get clarifications: %w", err)
	}
	defer rows.Close()

	var result []ClarificationRow
	for rows.Next() {
		var c ClarificationRow
		var disamb string
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.IngressTurnID, &c.QuestionText,
			&c.EstimatedGain, &disamb, &c.WasSelected); err != nil {
			return nil, fmt.Errorf("scan clarification: %w", err)
		}
		_ = json.Unmarshal([]byte(disamb), &c.Disambiguates)
		result = append(result, c)
	}
	return result, rows.Err()
}

// --- COG-10: Consolidation runs ---

func (r *PgCognitiveRepository) PersistConsolidationRun(ctx context.Context, row ConsolidationRunRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO consolidation_runs (workspace_id, run_date, episodes_analyzed, patterns_extracted, patterns_promoted, patterns_discarded, cost_usd, status)
		 VALUES ($1::uuid, $2::date, $3, $4, $5, $6, $7, $8::consolidation_status)
		 ON CONFLICT (workspace_id, run_date) DO UPDATE SET
		   episodes_analyzed = EXCLUDED.episodes_analyzed,
		   patterns_extracted = EXCLUDED.patterns_extracted,
		   status = EXCLUDED.status`,
		row.WorkspaceID, row.RunDate, row.EpisodesAnalyzed, row.PatternsExtracted,
		row.PatternsPromoted, row.PatternsDiscarded, row.CostUSD, row.Status,
	)
	if err != nil {
		return fmt.Errorf("persist consolidation run: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetLatestConsolidationRun(ctx context.Context, workspaceID string) (*ConsolidationRunRow, error) {
	var row ConsolidationRunRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, run_date, episodes_analyzed, patterns_extracted, patterns_promoted, patterns_discarded, COALESCE(cost_usd, 0), status
		 FROM consolidation_runs
		 WHERE workspace_id = $1::uuid
		 ORDER BY run_date DESC LIMIT 1`,
		workspaceID,
	).Scan(&row.ID, &row.WorkspaceID, &row.RunDate, &row.EpisodesAnalyzed, &row.PatternsExtracted,
		&row.PatternsPromoted, &row.PatternsDiscarded, &row.CostUSD, &row.Status)
	if err != nil {
		return nil, fmt.Errorf("get latest consolidation run: %w", err)
	}
	return &row, nil
}

func (r *PgCognitiveRepository) CompleteConsolidationRun(ctx context.Context, id, status string, promoted, discarded int) error {
	_, err := r.q.Exec(ctx,
		`UPDATE consolidation_runs SET status = $2::consolidation_status, patterns_promoted = $3, patterns_discarded = $4
		 WHERE id = $1::uuid`,
		id, status, promoted, discarded,
	)
	if err != nil {
		return fmt.Errorf("complete consolidation run: %w", err)
	}
	return nil
}

// --- COG-11: Behavioral baselines ---

func (r *PgCognitiveRepository) PersistBaseline(ctx context.Context, row BaselineRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO behavioral_baselines (workspace_id, baseline_window_start, baseline_window_end, topic_distribution, skill_usage_distribution, avg_message_hour, override_rate, correction_rate, is_current_baseline)
		 VALUES ($1::uuid, $2::date, $3::date, $4::jsonb, $5::jsonb, $6, $7, $8, $9)
		 ON CONFLICT (workspace_id, baseline_window_start) DO UPDATE SET
		   topic_distribution = EXCLUDED.topic_distribution,
		   skill_usage_distribution = EXCLUDED.skill_usage_distribution,
		   avg_message_hour = EXCLUDED.avg_message_hour,
		   override_rate = EXCLUDED.override_rate,
		   correction_rate = EXCLUDED.correction_rate,
		   is_current_baseline = EXCLUDED.is_current_baseline`,
		row.WorkspaceID, row.BaselineWindowStart, row.BaselineWindowEnd,
		row.TopicDistribution, row.SkillUsageDistribution,
		row.AvgMessageHour, row.OverrideRate, row.CorrectionRate, row.IsCurrentBaseline,
	)
	if err != nil {
		return fmt.Errorf("persist baseline: %w", err)
	}
	return nil
}

func (r *PgCognitiveRepository) GetCurrentBaseline(ctx context.Context, workspaceID string) (*BaselineRow, error) {
	var row BaselineRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, baseline_window_start, baseline_window_end,
		        topic_distribution, skill_usage_distribution,
		        avg_message_hour, override_rate, correction_rate, is_current_baseline
		 FROM behavioral_baselines
		 WHERE workspace_id = $1::uuid AND is_current_baseline = true
		 LIMIT 1`,
		workspaceID,
	).Scan(&row.ID, &row.WorkspaceID, &row.BaselineWindowStart, &row.BaselineWindowEnd,
		&row.TopicDistribution, &row.SkillUsageDistribution,
		&row.AvgMessageHour, &row.OverrideRate, &row.CorrectionRate, &row.IsCurrentBaseline)
	if err != nil {
		return nil, fmt.Errorf("get current baseline: %w", err)
	}
	return &row, nil
}

func (r *PgCognitiveRepository) SetCurrentBaseline(ctx context.Context, workspaceID, baselineID string) error {
	// Clear all current baselines for workspace, then set the new one.
	_, err := r.q.Exec(ctx,
		`UPDATE behavioral_baselines SET is_current_baseline = false WHERE workspace_id = $1::uuid`,
		workspaceID,
	)
	if err != nil {
		return fmt.Errorf("clear current baselines: %w", err)
	}
	_, err = r.q.Exec(ctx,
		`UPDATE behavioral_baselines SET is_current_baseline = true WHERE id = $1::uuid`,
		baselineID,
	)
	if err != nil {
		return fmt.Errorf("set current baseline: %w", err)
	}
	return nil
}
