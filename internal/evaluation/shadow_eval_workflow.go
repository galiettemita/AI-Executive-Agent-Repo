package evaluation

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ShadowEvalInput is the input for the shadow evaluation workflow.
type ShadowEvalInput struct {
	RequestID          uuid.UUID `json:"request_id"`
	WorkspaceID        uuid.UUID `json:"workspace_id"`
	UserMessage        string    `json:"user_message"`
	ChampionResponse   string    `json:"champion_response"`
	ChallengerResponse string    `json:"challenger_response"`
	ChallengerModel    string    `json:"challenger_model"`
}

// ShadowEvalResult holds the scored comparison.
type ShadowEvalResult struct {
	RequestID          uuid.UUID `json:"request_id"`
	WorkspaceID        uuid.UUID `json:"workspace_id"`
	ChampionORMScore   float64   `json:"champion_orm_score"`
	ChallengerORMScore float64   `json:"challenger_orm_score"`
	ChampionLLMScore   float64   `json:"champion_llm_score"`
	ChallengerLLMScore float64   `json:"challenger_llm_score"`
	ChallengerModel    string    `json:"challenger_model"`
	EvaluatedAt        time.Time `json:"evaluated_at"`
}

// ORMScorer scores a response for quality.
type ORMScorer interface {
	Score(ctx context.Context, response string) (float64, error)
}

// LLMJudge compares two responses and returns scores.
type LLMJudge interface {
	Compare(ctx context.Context, champion, challenger string) (championScore, challengerScore float64, err error)
}

// ShadowEvalWorkflow runs ORM + LLM judge scoring on champion vs challenger.
func ShadowEvalWorkflow(ctx workflow.Context, input ShadowEvalInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ShadowEvalWorkflow started",
		"request_id", input.RequestID,
		"challenger_model", input.ChallengerModel,
	)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})

	var champORM, challORM float64
	if err := workflow.ExecuteActivity(actCtx, ScoreChampionActivity, input.ChampionResponse).Get(ctx, &champORM); err != nil {
		logger.Error("score champion failed", "error", err)
		champORM = 5.0
	}

	if err := workflow.ExecuteActivity(actCtx, ScoreChallengerActivity, input.ChallengerResponse).Get(ctx, &challORM); err != nil {
		logger.Error("score challenger failed", "error", err)
		challORM = 5.0
	}

	var champLLM, challLLM float64
	judgeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})
	var judgeResult [2]float64
	if err := workflow.ExecuteActivity(judgeCtx, LLMJudgeEvalActivity, input.ChampionResponse, input.ChallengerResponse).Get(ctx, &judgeResult); err != nil {
		logger.Error("LLM judge failed", "error", err)
		champLLM = 5.0
		challLLM = 5.0
	} else {
		champLLM = judgeResult[0]
		challLLM = judgeResult[1]
	}

	result := ShadowEvalResult{
		RequestID:          input.RequestID,
		WorkspaceID:        input.WorkspaceID,
		ChampionORMScore:   champORM,
		ChallengerORMScore: challORM,
		ChampionLLMScore:   champLLM,
		ChallengerLLMScore: challLLM,
		ChallengerModel:    input.ChallengerModel,
		EvaluatedAt:        workflow.Now(ctx),
	}

	recordCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	_ = workflow.ExecuteActivity(recordCtx, RecordShadowResultActivity, result).Get(ctx, nil)

	logger.Info("ShadowEvalWorkflow complete",
		"champion_orm", champORM, "challenger_orm", challORM,
		"champion_llm", champLLM, "challenger_llm", challLLM,
	)
	return nil
}

// Activity stubs for Temporal registration.
func ScoreChampionActivity(_ context.Context, _ string) (float64, error)      { return 5.0, nil }
func ScoreChallengerActivity(_ context.Context, _ string) (float64, error)    { return 5.0, nil }
func LLMJudgeEvalActivity(_ context.Context, _, _ string) ([2]float64, error) { return [2]float64{5.0, 5.0}, nil }
func RecordShadowResultActivity(_ context.Context, _ ShadowEvalResult) error  { return nil }

// ShadowEvalActivities holds dependencies for activity method binding.
type ShadowEvalActivities struct {
	ORM    ORMScorer
	Judge  LLMJudge
	DB     *pgxpool.Pool
	Logger *slog.Logger
}

func (a *ShadowEvalActivities) ScoreChampion(ctx context.Context, response string) (float64, error) {
	if a.ORM == nil {
		return 5.0, nil
	}
	return a.ORM.Score(ctx, response)
}

func (a *ShadowEvalActivities) ScoreChallenger(ctx context.Context, response string) (float64, error) {
	if a.ORM == nil {
		return 5.0, nil
	}
	return a.ORM.Score(ctx, response)
}

func (a *ShadowEvalActivities) LLMJudgeEval(ctx context.Context, champion, challenger string) ([2]float64, error) {
	if a.Judge == nil {
		return [2]float64{5.0, 5.0}, nil
	}
	cs, chs, err := a.Judge.Compare(ctx, champion, challenger)
	return [2]float64{cs, chs}, err
}

func (a *ShadowEvalActivities) RecordResult(ctx context.Context, result ShadowEvalResult) error {
	if a.DB == nil {
		return nil
	}
	detailsJSON, _ := json.Marshal(result)
	_, err := a.DB.Exec(ctx,
		`INSERT INTO shadow_eval_results
		 (request_id, workspace_id, champion_orm_score, challenger_orm_score,
		  champion_llm_score, challenger_llm_score, challenger_model)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		result.RequestID, result.WorkspaceID,
		result.ChampionORMScore, result.ChallengerORMScore,
		result.ChampionLLMScore, result.ChallengerLLMScore,
		result.ChallengerModel,
	)
	_ = detailsJSON
	return err
}
