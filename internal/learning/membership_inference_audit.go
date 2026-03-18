package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditResult captures the outcome of a membership inference audit.
type AuditResult struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	AUC         float64   `json:"auc"`
	RunAt       time.Time `json:"run_at"`
	AlertFired  bool      `json:"alert_fired"`
}

// ShadowModelClient is the interface for membership inference scoring.
type ShadowModelClient interface {
	Predict(ctx context.Context, sample string) (float64, error)
}

// MembershipInferenceAudit runs membership inference attacks against the
// fine-tuned model to detect memorization of training data.
type MembershipInferenceAudit struct {
	db           *pgxpool.Pool
	shadowModel  ShadowModelClient
	logger       *slog.Logger
}

// NewMembershipInferenceAudit creates an audit runner.
func NewMembershipInferenceAudit(db *pgxpool.Pool, shadowModel ShadowModelClient, logger *slog.Logger) *MembershipInferenceAudit {
	return &MembershipInferenceAudit{
		db:          db,
		shadowModel: shadowModel,
		logger:      logger,
	}
}

// RunAudit executes a membership inference audit for a workspace.
// It compares model confidence on training data (members) vs non-training data
// (non-members) and computes AUC-ROC. Fires an alert if AUC > 0.6.
func (m *MembershipInferenceAudit) RunAudit(ctx context.Context, workspaceID uuid.UUID) (*AuditResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	// Load 100 training samples (members) from preference_pairs.
	members, err := m.loadMemberSamples(ctx, workspaceID, 100)
	if err != nil {
		return nil, fmt.Errorf("load member samples: %w", err)
	}

	// Load 100 non-member samples from preference_pairs with used_in_round IS NULL.
	nonMembers, err := m.loadNonMemberSamples(ctx, workspaceID, 100)
	if err != nil {
		return nil, fmt.Errorf("load non-member samples: %w", err)
	}

	if len(members) == 0 && len(nonMembers) == 0 {
		m.logger.Info("membership_inference_no_data", "workspace_id", workspaceID)
		return &AuditResult{
			WorkspaceID: workspaceID,
			AUC:         0.5,
			RunAt:       time.Now(),
			AlertFired:  false,
		}, nil
	}

	// Score all samples via shadow model.
	var scored []miScoredSample

	for _, s := range members {
		score, err := m.shadowModel.Predict(ctx, s)
		if err != nil {
			m.logger.Warn("shadow_model_predict_error", "error", err)
			score = 0.5
		}
		scored = append(scored, miScoredSample{score: score, isMember: true})
	}

	for _, s := range nonMembers {
		score, err := m.shadowModel.Predict(ctx, s)
		if err != nil {
			m.logger.Warn("shadow_model_predict_error", "error", err)
			score = 0.5
		}
		scored = append(scored, miScoredSample{score: score, isMember: false})
	}

	// Compute AUC-ROC via trapezoidal rule.
	auc := computeAUCROC(scored)

	alertFired := false
	if auc > 0.6 {
		alertFired = true
		m.logger.Warn("membership_inference_alert",
			"workspace_id", workspaceID,
			"auc", auc,
			"message", fmt.Sprintf("Membership inference AUC exceeded threshold for workspace %s: AUC=%.3f", workspaceID, auc),
		)
	}

	result := &AuditResult{
		WorkspaceID: workspaceID,
		AUC:         auc,
		RunAt:       time.Now(),
		AlertFired:  alertFired,
	}

	// Persist result.
	if m.db != nil {
		_, dbErr := m.db.Exec(ctx,
			`INSERT INTO membership_inference_results (workspace_id, auc, alert_fired, run_at)
			 VALUES ($1, $2, $3, $4)`,
			result.WorkspaceID, result.AUC, result.AlertFired, result.RunAt,
		)
		if dbErr != nil {
			m.logger.Error("persist_mi_result_error", "error", dbErr)
		}
	}

	m.logger.Info("membership_inference_audit_complete",
		"workspace_id", workspaceID,
		"auc", auc,
		"alert_fired", alertFired,
		"members", len(members),
		"non_members", len(nonMembers),
	)

	return result, nil
}

func (m *MembershipInferenceAudit) loadMemberSamples(ctx context.Context, workspaceID uuid.UUID, limit int) ([]string, error) {
	rows, err := m.db.Query(ctx,
		`SELECT prompt_text FROM preference_pairs
		 WHERE workspace_id = $1 AND used_in_round IS NOT NULL
		 ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

func (m *MembershipInferenceAudit) loadNonMemberSamples(ctx context.Context, workspaceID uuid.UUID, limit int) ([]string, error) {
	rows, err := m.db.Query(ctx,
		`SELECT prompt_text FROM preference_pairs
		 WHERE workspace_id = $1 AND used_in_round IS NULL
		 ORDER BY created_at ASC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

// miScoredSample pairs a model confidence score with its membership label.
type miScoredSample struct {
	score    float64
	isMember bool
}

// computeAUCROC computes the Area Under the ROC Curve using the trapezoidal rule.
func computeAUCROC(samples []miScoredSample) float64 {
	if len(samples) == 0 {
		return 0.5
	}

	// Sort by score descending.
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].score > samples[j].score
	})

	totalPositive := 0
	totalNegative := 0
	for _, s := range samples {
		if s.isMember {
			totalPositive++
		} else {
			totalNegative++
		}
	}

	if totalPositive == 0 || totalNegative == 0 {
		return 0.5
	}

	auc := 0.0
	tp := 0
	fp := 0
	prevTPR := 0.0
	prevFPR := 0.0

	for _, s := range samples {
		if s.isMember {
			tp++
		} else {
			fp++
		}
		tpr := float64(tp) / float64(totalPositive)
		fpr := float64(fp) / float64(totalNegative)

		// Trapezoidal rule.
		auc += (fpr - prevFPR) * (tpr + prevTPR) / 2.0

		prevTPR = tpr
		prevFPR = fpr
	}

	return auc
}

// LocalShadowModelClient uses Ollama's generate API to compute a membership
// signal based on evaluation latency.
type LocalShadowModelClient struct {
	ollamaURL   string
	shadowModel string
	httpClient  *http.Client
	logger      *slog.Logger
}

// NewLocalShadowModelClient creates a shadow model client using Ollama.
func NewLocalShadowModelClient(logger *slog.Logger) *LocalShadowModelClient {
	ollamaURL := os.Getenv("OLLAMA_BASE_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	shadowModel := os.Getenv("OLLAMA_SHADOW_MODEL")
	if shadowModel == "" {
		shadowModel = "llama3.2:3b"
	}

	return &LocalShadowModelClient{
		ollamaURL:   ollamaURL,
		shadowModel: shadowModel,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		logger:      logger,
	}
}

// Predict sends a sample to Ollama and derives a membership signal from
// prompt evaluation latency. Members (training data) tend to have lower
// per-token eval duration.
func (c *LocalShadowModelClient) Predict(ctx context.Context, sample string) (float64, error) {
	reqBody := map[string]any{
		"model":  c.shadowModel,
		"prompt": sample,
		"stream": false,
		"options": map[string]any{
			"num_predict": 1,
			"temperature": 0.0,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0.5, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ollamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return 0.5, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("ollama_unavailable", "error", err)
		return 0.5, nil
	}
	defer resp.Body.Close()

	var result struct {
		EvalDuration       int64 `json:"eval_duration"`
		PromptEvalDuration int64 `json:"prompt_eval_duration"`
		PromptEvalCount    int   `json:"prompt_eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0.5, fmt.Errorf("decode response: %w", err)
	}

	if result.PromptEvalCount == 0 {
		return 0.5, nil
	}

	nsPerToken := float64(result.PromptEvalDuration) / float64(result.PromptEvalCount)
	const baselineNsPerToken = 500_000.0
	score := 1.0 - (nsPerToken / (2.0 * baselineNsPerToken))
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score, nil
}
