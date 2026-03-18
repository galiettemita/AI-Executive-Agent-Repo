package evaluation

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"
)

// ShadowReceiver handles mirrored traffic from Istio and triggers shadow evaluation.
type ShadowReceiver struct {
	temporalClient temporalclient.Client
	taskQueue      string
	logger         *slog.Logger
}

// NewShadowReceiver creates a shadow traffic receiver.
func NewShadowReceiver(tc temporalclient.Client, taskQueue string, logger *slog.Logger) *ShadowReceiver {
	return &ShadowReceiver{
		temporalClient: tc,
		taskQueue:      taskQueue,
		logger:         logger,
	}
}

type shadowRequest struct {
	RequestID   string `json:"request_id"`
	WorkspaceID string `json:"workspace_id"`
	UserMessage string `json:"user_message"`
}

// ServeHTTP handles POST /internal/shadow/evaluate.
func (r *ShadowReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Check if shadow eval is enabled.
	if os.Getenv("SHADOW_EVAL_ENABLED") != "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Application-level sampling (defense in depth on top of Istio rate).
	rate := 0.05
	if rateStr := os.Getenv("SHADOW_EVAL_RATE"); rateStr != "" {
		if parsed, err := strconv.ParseFloat(rateStr, 64); err == nil {
			rate = parsed
		}
	}
	if rand.Float64() > rate {
		w.WriteHeader(http.StatusOK)
		return
	}

	var body shadowRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	reqID, _ := uuid.Parse(body.RequestID)
	wsID, _ := uuid.Parse(body.WorkspaceID)

	challengerModel := os.Getenv("CHALLENGER_MODEL")
	if challengerModel == "" {
		challengerModel = "challenger-default"
	}

	// Start shadow eval workflow asynchronously.
	go func() {
		input := ShadowEvalInput{
			RequestID:          reqID,
			WorkspaceID:        wsID,
			UserMessage:        body.UserMessage,
			ChampionResponse:   "", // filled by workflow activities
			ChallengerResponse: "",
			ChallengerModel:    challengerModel,
		}

		if r.temporalClient != nil {
			opts := temporalclient.StartWorkflowOptions{
				ID:        "shadow-eval-" + reqID.String(),
				TaskQueue: r.taskQueue,
			}
			_, err := r.temporalClient.ExecuteWorkflow(req.Context(), opts, ShadowEvalWorkflow, input)
			if err != nil {
				r.logger.Error("shadow_eval_workflow_start_error", "error", err)
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
}
