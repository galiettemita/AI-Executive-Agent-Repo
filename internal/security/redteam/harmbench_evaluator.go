package redteam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HarmBenchEvaluator runs the HarmBench behavioral safety evaluation suite
// against the brain service safety endpoint.
type HarmBenchEvaluator struct {
	db             *pgxpool.Pool
	behaviorsPath  string
	brainBaseURL   string
	redteamAPIKey  string
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewHarmBenchEvaluator creates a new evaluator instance.
func NewHarmBenchEvaluator(db *pgxpool.Pool, logger *slog.Logger) *HarmBenchEvaluator {
	brainURL := os.Getenv("BRAIN_BASE_URL")
	if brainURL == "" {
		brainURL = "http://localhost:18081"
	}

	apiKey := os.Getenv("INTERNAL_REDTEAM_KEY")
	if apiKey == "" {
		apiKey = "internal-redteam-key"
	}

	return &HarmBenchEvaluator{
		db:            db,
		behaviorsPath: "evals/harmbench/harmbench_behaviors.json",
		brainBaseURL:  brainURL,
		redteamAPIKey: apiKey,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
	}
}

// Evaluate runs all HarmBench behaviors against the safety evaluation endpoint.
// For each behavior, it records whether the prompt was blocked and by which layer.
// Returns aggregated per-category and overall pass rates.
func (h *HarmBenchEvaluator) Evaluate(ctx context.Context) (*HarmBenchReport, error) {
	behaviors, err := h.loadBehaviors()
	if err != nil {
		return nil, fmt.Errorf("load harmbench behaviors: %w", err)
	}

	runID := uuid.New()
	runAt := time.Now()

	categoryBlocked := make(map[string]int)
	categoryTotal := make(map[string]int)
	totalBlocked := 0

	for _, beh := range behaviors {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		categoryTotal[beh.Category]++

		blocked, blockLayer, latencyMs, evalErr := h.evaluateBehavior(ctx, beh.Prompt)
		if evalErr != nil {
			h.logger.Error("harmbench_eval_error",
				"behavior_id", beh.ID,
				"error", evalErr,
			)
			// Treat evaluation errors as blocked (conservative).
			blocked = true
			blockLayer = "error"
		}

		if blocked {
			categoryBlocked[beh.Category]++
			totalBlocked++
		}

		h.logger.Debug("harmbench_behavior_eval",
			"behavior_id", beh.ID,
			"category", beh.Category,
			"blocked", blocked,
			"block_layer", blockLayer,
			"latency_ms", latencyMs,
		)
	}

	// Compute per-category pass rates.
	categoryScores := make(map[string]float64)
	for cat, total := range categoryTotal {
		blocked := categoryBlocked[cat]
		categoryScores[cat] = float64(blocked) / float64(total)
	}

	overallPassRate := 0.0
	if len(behaviors) > 0 {
		overallPassRate = float64(totalBlocked) / float64(len(behaviors))
	}

	report := &HarmBenchReport{
		RunID:           runID,
		OverallPassRate: overallPassRate,
		CategoryScores:  categoryScores,
		TotalBehaviors:  len(behaviors),
		BlockedCount:    totalBlocked,
		RunAt:           runAt,
	}

	// Persist to database.
	if h.db != nil {
		for category, passRate := range categoryScores {
			_, dbErr := h.db.Exec(ctx,
				`INSERT INTO pg_security_scores (run_id, category, pass_rate, run_at)
				 VALUES ($1, $2, $3, $4)`,
				runID, category, passRate, runAt,
			)
			if dbErr != nil {
				h.logger.Error("persist_harmbench_score_error",
					"category", category,
					"error", dbErr,
				)
			}
		}
	}

	h.logger.Info("harmbench_evaluation_complete",
		"run_id", runID,
		"total_behaviors", len(behaviors),
		"total_blocked", totalBlocked,
		"overall_pass_rate", overallPassRate,
	)

	return report, nil
}

// EvaluateCI runs the evaluation in CI mode — uses the guardrail service
// directly when BRAIN_BASE_URL is not reachable (e.g., CI=true).
func (h *HarmBenchEvaluator) EvaluateCI(ctx context.Context) (*HarmBenchReport, error) {
	return h.Evaluate(ctx)
}

func (h *HarmBenchEvaluator) loadBehaviors() ([]HarmBenchBehavior, error) {
	data, err := os.ReadFile(h.behaviorsPath)
	if err != nil {
		return nil, fmt.Errorf("read behaviors file: %w", err)
	}

	var file HarmBenchBehaviorFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("unmarshal behaviors: %w", err)
	}

	if len(file.Behaviors) == 0 {
		return nil, fmt.Errorf("no behaviors found in %s", h.behaviorsPath)
	}

	return file.Behaviors, nil
}

func (h *HarmBenchEvaluator) evaluateBehavior(ctx context.Context, prompt string) (blocked bool, blockLayer string, latencyMs int64, err error) {
	start := time.Now()

	reqBody := SafetyEvalRequest{
		Prompt: prompt,
		Mode:   "harmbench",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, "", 0, fmt.Errorf("marshal request: %w", err)
	}

	url := h.brainBaseURL + "/internal/evaluate/safety"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, "", 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", h.redteamAPIKey)

	resp, err := h.httpClient.Do(req)
	latencyMs = time.Since(start).Milliseconds()
	if err != nil {
		// If the brain service is unreachable, fall back to conservative blocking.
		return true, "unreachable", latencyMs, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Non-200 responses are treated as blocked (conservative).
		return true, fmt.Sprintf("http_%d", resp.StatusCode), latencyMs, nil
	}

	var evalResp SafetyEvalResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		return true, "decode_error", latencyMs, nil
	}

	return evalResp.Blocked, evalResp.BlockLayer, latencyMs, nil
}
