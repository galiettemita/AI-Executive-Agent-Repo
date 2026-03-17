package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/brevio/brevio/internal/memory"
	"github.com/brevio/brevio/internal/preference"
)

// PreferenceUpdateActivity extracts a preference fact from a user correction signal
// and writes it to long-term episodic memory with confidence=0.95.
func (a *Activities) PreferenceUpdateActivity(ctx context.Context, in preference.UpdateInput) (preference.PreferenceFact, error) {
	logger := activity.GetLogger(ctx)

	validSignals := map[string]bool{
		"undo": true, "edit": true, "retry": true,
		"skip": true, "explicit_thumbsdown": true,
	}
	if !validSignals[in.Signal.SignalType] {
		return preference.PreferenceFact{}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("signal type %q is not a correction signal", in.Signal.SignalType),
			"INVALID_SIGNAL", nil,
		)
	}
	if in.Signal.WorkspaceID == "" || in.Signal.UserID == "" {
		return preference.PreferenceFact{}, temporal.NewNonRetryableApplicationError(
			"workspace_id and user_id are required", "MISSING_FIELDS", nil,
		)
	}

	fact := preference.ExtractFact(in.Signal)

	logger.Info("PreferenceUpdateActivity: extracted fact",
		"category", fact.Category,
		"workspace_id", in.Signal.WorkspaceID,
		"signal_type", in.Signal.SignalType,
	)

	if a.memorySvc != nil {
		_, err := a.memorySvc.WriteWithRequest(memory.WriteRequest{
			WorkspaceID:       in.Signal.WorkspaceID,
			UserID:            in.Signal.UserID,
			MemoryType:        "preference",
			Body:              fact.Preference,
			Confidence:        fact.Confidence,
			DataClass:         "internal",
			SensitivityLabel:  "moderate",
			RetentionPolicyID: "preference",
			AllowedProcessors: []string{"brain", "control"},
			ContentTrust:      "verified",
		})
		if err != nil {
			return fact, fmt.Errorf("PreferenceUpdateActivity: write memory: %w", err)
		}
	}

	return fact, nil
}

// PreferenceRetrievalActivity fetches the top K preference facts for a workspace+user.
func (a *Activities) PreferenceRetrievalActivity(ctx context.Context, in preference.RetrievalInput) (preference.PreferenceContext, error) {
	logger := activity.GetLogger(ctx)

	empty := preference.PreferenceContext{
		WorkspaceID: in.WorkspaceID,
		UserID:      in.UserID,
	}

	if a.preferenceRetriever == nil {
		logger.Warn("PreferenceRetrievalActivity: retriever not configured")
		return empty, nil
	}

	if in.TopK == 0 {
		in.TopK = 5
	}

	facts, err := a.preferenceRetriever.FetchTopK(ctx, in.WorkspaceID, in.UserID, in.Intent, in.TopK)
	if err != nil {
		logger.Warn("PreferenceRetrievalActivity: retrieval failed", "error", err)
		return empty, nil
	}

	return preference.PreferenceContext{
		WorkspaceID:   in.WorkspaceID,
		UserID:        in.UserID,
		Facts:         facts,
		FormattedText: preference.FormatForLLM(facts),
	}, nil
}
