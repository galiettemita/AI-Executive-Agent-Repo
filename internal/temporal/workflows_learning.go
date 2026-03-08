package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// LearningConsolidationInput is the input for the learning consolidation workflow.
type LearningConsolidationInput struct {
	WorkspaceID string `json:"workspace_id"`
	BatchSize   int    `json:"batch_size"`
}

// LearningConsolidationResult is the output of the learning consolidation workflow.
type LearningConsolidationResult struct {
	LessonsProcessed  int `json:"lessons_processed"`
	ConflictsDetected int `json:"conflicts_detected"`
	ConflictsResolved int `json:"conflicts_resolved"`
	RulesProposed     int `json:"rules_proposed"`
}

// LearningConsolidationWorkflow periodically consolidates learned lessons,
// detects conflicts, and proposes rules.
func LearningConsolidationWorkflow(ctx workflow.Context, input LearningConsolidationInput) (*LearningConsolidationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("LearningConsolidationWorkflow started", "workspaceID", input.WorkspaceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Cluster corrections into lesson candidates
	var clusterResult ClusterCorrectionsResult
	err := workflow.ExecuteActivity(ctx, ClusterCorrectionsActivity, ClusterCorrectionsInput{
		WorkspaceID: input.WorkspaceID,
		BatchSize:   input.BatchSize,
	}).Get(ctx, &clusterResult)
	if err != nil {
		return nil, err
	}

	// Step 2: Detect conflicts among lessons
	var detectResult DetectConflictsResult
	err = workflow.ExecuteActivity(ctx, DetectConflictsActivity, DetectConflictsInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &detectResult)
	if err != nil {
		return nil, err
	}

	// Step 3: Auto-resolve redundant conflicts
	resolved := 0
	for _, conflictID := range detectResult.RedundantConflictIDs {
		var resolveResult ResolveConflictResult
		err = workflow.ExecuteActivity(ctx, ResolveConflictActivity, ResolveConflictInput{
			ConflictID: conflictID,
			Resolution: "keep_a",
		}).Get(ctx, &resolveResult)
		if err == nil && resolveResult.Success {
			resolved++
		}
	}

	// Step 4: Propose rules from confirmed lessons
	var proposeResult ProposeRulesResult
	err = workflow.ExecuteActivity(ctx, ProposeRulesActivity, ProposeRulesInput{
		WorkspaceID: input.WorkspaceID,
	}).Get(ctx, &proposeResult)
	if err != nil {
		logger.Warn("rule proposal failed", "error", err)
	}

	return &LearningConsolidationResult{
		LessonsProcessed:  clusterResult.LessonsCreated,
		ConflictsDetected: detectResult.TotalConflicts,
		ConflictsResolved: resolved,
		RulesProposed:     proposeResult.RulesProposed,
	}, nil
}

// Learning activity types

// ClusterCorrectionsInput is the input for ClusterCorrectionsActivity.
type ClusterCorrectionsInput struct {
	WorkspaceID string `json:"workspace_id"`
	BatchSize   int    `json:"batch_size"`
}

// ClusterCorrectionsResult is the result of ClusterCorrectionsActivity.
type ClusterCorrectionsResult struct {
	LessonsCreated int `json:"lessons_created"`
}

// DetectConflictsInput is the input for DetectConflictsActivity.
type DetectConflictsInput struct {
	WorkspaceID string `json:"workspace_id"`
}

// DetectConflictsResult is the result of DetectConflictsActivity.
type DetectConflictsResult struct {
	TotalConflicts       int      `json:"total_conflicts"`
	RedundantConflictIDs []string `json:"redundant_conflict_ids"`
}

// ResolveConflictInput is the input for ResolveConflictActivity.
type ResolveConflictInput struct {
	ConflictID string `json:"conflict_id"`
	Resolution string `json:"resolution"`
}

// ResolveConflictResult is the result of ResolveConflictActivity.
type ResolveConflictResult struct {
	Success bool `json:"success"`
}

// ProposeRulesInput is the input for ProposeRulesActivity.
type ProposeRulesInput struct {
	WorkspaceID string `json:"workspace_id"`
}

// ProposeRulesResult is the result of ProposeRulesActivity.
type ProposeRulesResult struct {
	RulesProposed int `json:"rules_proposed"`
}
