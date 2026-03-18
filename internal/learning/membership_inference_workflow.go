package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// MembershipInferenceCronWorkflowID is the stable Temporal workflow ID.
	MembershipInferenceCronWorkflowID = "brevio-membership-inference-cron"

	// MembershipInferenceCronSchedule runs quarterly: 1st of every 3rd month at 03:00 UTC.
	MembershipInferenceCronSchedule = "0 3 1 */3 *"
)

// MembershipInferenceWorkflowInput provides the workspace to audit.
type MembershipInferenceWorkflowInput struct {
	WorkspaceID string `json:"workspace_id"`
}

// MembershipInferenceWorkflow runs a membership inference audit for a workspace.
func MembershipInferenceWorkflow(ctx workflow.Context, input MembershipInferenceWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("MembershipInferenceWorkflow started", "workspace_id", input.WorkspaceID)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	var result AuditResult
	if err := workflow.ExecuteActivity(actCtx, RunMembershipInferenceActivity, input).Get(ctx, &result); err != nil {
		return fmt.Errorf("membership inference audit: %w", err)
	}

	logger.Info("MembershipInferenceWorkflow complete",
		"workspace_id", input.WorkspaceID,
		"auc", result.AUC,
		"alert_fired", result.AlertFired,
	)
	return nil
}

// RunMembershipInferenceActivity is the activity function placeholder for Temporal registration.
func RunMembershipInferenceActivity(_ context.Context, _ MembershipInferenceWorkflowInput) (*AuditResult, error) {
	return nil, nil
}

// MembershipInferenceActivities holds the audit runner for activity method binding.
type MembershipInferenceActivities struct {
	Audit *MembershipInferenceAudit
}

// RunAuditActivity executes the membership inference audit.
func (a *MembershipInferenceActivities) RunAuditActivity(ctx context.Context, input MembershipInferenceWorkflowInput) (*AuditResult, error) {
	wsID, err := uuid.Parse(input.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}
	return a.Audit.RunAudit(ctx, wsID)
}

// ScheduleMembershipInferenceCron registers the quarterly cron with Temporal.
func ScheduleMembershipInferenceCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           MembershipInferenceCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: MembershipInferenceCronSchedule,
	}

	_, err := tc.ExecuteWorkflow(context.Background(), opts,
		MembershipInferenceWorkflow,
		MembershipInferenceWorkflowInput{WorkspaceID: "all"},
	)
	if err != nil {
		return fmt.Errorf("schedule membership inference cron: %w", err)
	}
	return nil
}
