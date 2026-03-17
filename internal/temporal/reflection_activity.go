package temporal

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/brevio/brevio/internal/memory"
	"github.com/brevio/brevio/internal/reflection"
)

// ReflectionActivity performs daily hindsight reflection for a workspace.
// It clusters intents by theme, identifies tool failure patterns,
// generates structured insight records, and writes them to long-term memory.
func (a *Activities) ReflectionActivity(ctx context.Context, in reflection.ReflectionInput) (reflection.ReflectionResult, error) {
	logger := activity.GetLogger(ctx)

	if in.WorkspaceID == "" {
		return reflection.ReflectionResult{}, fmt.Errorf("ReflectionActivity: workspace_id required")
	}
	if in.Date == "" {
		in.Date = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if in.MaxInsights == 0 {
		in.MaxInsights = 10
	}

	logger.Info("ReflectionActivity: starting",
		"workspace_id", in.WorkspaceID,
		"date", in.Date,
	)

	// Load day log from audit trail
	dayLog := a.loadDayLog(ctx, in.WorkspaceID, in.Date)

	// Cluster intents by theme
	clusters := reflection.ClusterIntents(dayLog.IntentEvents)

	// Identify tool failure patterns
	failures := reflection.IdentifyFailurePatterns(dayLog.ToolEvents)

	// Generate insights
	insights := reflection.GenerateInsights(in.WorkspaceID, in.Date, clusters, failures, in.MaxInsights)

	// Write each insight to long-term memory
	written := 0
	if a.memorySvc != nil {
		for _, insight := range insights {
			_, err := a.memorySvc.WriteWithRequest(memory.WriteRequest{
				WorkspaceID:       insight.WorkspaceID,
				UserID:            "system",
				MemoryType:        "episodic",
				Body:              insight.Body,
				Confidence:        insight.Strength,
				DataClass:         "internal",
				SensitivityLabel:  "moderate",
				RetentionPolicyID: "reflection",
				AllowedProcessors: []string{"brain", "control"},
				ContentTrust:      "verified",
			})
			if err != nil {
				logger.Warn("ReflectionActivity: failed to write insight", "error", err)
				continue
			}
			written++
		}
	} else {
		written = len(insights)
	}

	logger.Info("ReflectionActivity: complete",
		"workspace_id", in.WorkspaceID,
		"date", in.Date,
		"insights_written", written,
		"clusters", len(clusters),
		"failure_patterns", len(failures),
	)

	return reflection.ReflectionResult{
		WorkspaceID:     in.WorkspaceID,
		Date:            in.Date,
		InsightsWritten: written,
		IntentClusters:  clusters,
		FailurePatterns: failures,
		TopInsights:     insights,
	}, nil
}

// loadDayLog retrieves intent and tool events for the given workspace/date.
func (a *Activities) loadDayLog(ctx context.Context, workspaceID, date string) reflection.DayLog {
	dayLog := reflection.DayLog{
		WorkspaceID: workspaceID,
		Date:        date,
	}

	if a.pool == nil {
		return dayLog
	}

	rows, err := a.pool.Query(ctx, `
		SELECT
			COALESCE(payload->>'intent', 'unknown') AS intent,
			COALESCE((payload->>'confidence')::float, 0.5) AS confidence,
			CASE WHEN error IS NULL THEN 'success' ELSE 'failure' END AS outcome
		FROM audit_log
		WHERE workspace_id = $1::uuid
		  AND event_type LIKE 'BREVIO.brain.workflow%'
		  AND created_at::date = $2::date
		LIMIT 200`,
		workspaceID, date,
	)
	if err != nil {
		return dayLog
	}
	defer rows.Close()

	for rows.Next() {
		var intent, outcome string
		var confidence float64
		if err := rows.Scan(&intent, &confidence, &outcome); err != nil {
			continue
		}
		dayLog.IntentEvents = append(dayLog.IntentEvents, reflection.IntentEvent{
			Intent:     intent,
			Confidence: confidence,
			Outcome:    outcome,
		})
	}

	toolRows, err := a.pool.Query(ctx, `
		SELECT
			COALESCE(payload->>'tool_key', 'unknown') AS tool_key,
			error IS NULL AS success,
			COALESCE(payload->>'error_code', '') AS error_code
		FROM audit_log
		WHERE workspace_id = $1::uuid
		  AND event_type LIKE 'BREVIO.hands.tool%'
		  AND created_at::date = $2::date
		LIMIT 500`,
		workspaceID, date,
	)
	if err != nil {
		return dayLog
	}
	defer toolRows.Close()

	for toolRows.Next() {
		var toolKey, errorCode string
		var success bool
		if err := toolRows.Scan(&toolKey, &success, &errorCode); err != nil {
			continue
		}
		dayLog.ToolEvents = append(dayLog.ToolEvents, reflection.ToolEvent{
			ToolKey:   toolKey,
			Success:   success,
			ErrorCode: errorCode,
		})
	}

	return dayLog
}
