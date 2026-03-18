package soc2

import (
	"context"
	"fmt"
	"time"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ComplianceCronWorkflowID = "brevio-compliance-evidence-cron"
	ComplianceCronSchedule   = "0 3 * * *"
)

// ComplianceWorkflowInput carries the date for day-of-week determination.
type ComplianceWorkflowInput struct {
	RunDate string `json:"run_date,omitempty"`
}

// ComplianceEvidenceWorkflow runs all evidence collection.
// Daily at 03:00 UTC. Weekly checks (CC6.6, CC9.2, ISO 27001) run on Mondays.
func ComplianceEvidenceWorkflow(ctx workflow.Context, input ComplianceWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ComplianceEvidenceWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Determine if today is Monday (weekly run day).
	now := workflow.Now(ctx)
	isMonday := now.Weekday() == time.Monday

	// Daily controls.
	var failedControls []string

	var cc61 ControlEvidence
	if err := workflow.ExecuteActivity(actCtx, CollectCC61Activity).Get(ctx, &cc61); err != nil {
		logger.Error("CC6.1 collection failed", "error", err)
	} else if !cc61.Pass {
		failedControls = append(failedControls, cc61.ControlID)
	}

	var cc72 ControlEvidence
	if err := workflow.ExecuteActivity(actCtx, CollectCC72Activity).Get(ctx, &cc72); err != nil {
		logger.Error("CC7.2 collection failed", "error", err)
	} else if !cc72.Pass {
		failedControls = append(failedControls, cc72.ControlID)
	}

	var pi14 ControlEvidence
	if err := workflow.ExecuteActivity(actCtx, CollectPI14Activity).Get(ctx, &pi14); err != nil {
		logger.Error("PI1.4 collection failed", "error", err)
	} else if !pi14.Pass {
		failedControls = append(failedControls, pi14.ControlID)
	}

	// Weekly controls — only on Mondays.
	if isMonday {
		var cc66 ControlEvidence
		if err := workflow.ExecuteActivity(actCtx, CollectCC66Activity).Get(ctx, &cc66); err != nil {
			logger.Error("CC6.6 collection failed", "error", err)
		} else if !cc66.Pass {
			failedControls = append(failedControls, cc66.ControlID)
		}

		var cc92 ControlEvidence
		if err := workflow.ExecuteActivity(actCtx, CollectCC92Activity).Get(ctx, &cc92); err != nil {
			logger.Error("CC9.2 collection failed", "error", err)
		} else if !cc92.Pass {
			failedControls = append(failedControls, cc92.ControlID)
		}

		// ISO 27001 weekly collection.
		if err := workflow.ExecuteActivity(actCtx, CollectISO27001Activity).Get(ctx, nil); err != nil {
			logger.Error("ISO 27001 collection failed", "error", err)
		}
	}

	if len(failedControls) > 0 {
		logger.Warn("ComplianceEvidenceWorkflow: controls failed",
			"failed", failedControls,
		)
	}

	logger.Info("ComplianceEvidenceWorkflow complete",
		"is_monday", isMonday,
		"failed_controls", len(failedControls),
	)
	return nil
}

// Activity function stubs for Temporal registration.
func CollectCC61Activity(_ context.Context) (*ControlEvidence, error)  { return nil, nil }
func CollectCC66Activity(_ context.Context) (*ControlEvidence, error)  { return nil, nil }
func CollectCC72Activity(_ context.Context) (*ControlEvidence, error)  { return nil, nil }
func CollectCC92Activity(_ context.Context) (*ControlEvidence, error)  { return nil, nil }
func CollectPI14Activity(_ context.Context) (*ControlEvidence, error)  { return nil, nil }
func CollectISO27001Activity(_ context.Context) error                  { return nil }

// ComplianceActivities holds the collector for activity method binding.
type ComplianceActivities struct {
	SOC2Collector    *ComplianceEvidenceCollector
	ISO27001Collector interface {
		CollectAll(ctx context.Context) ([]*ControlEvidence, error)
	}
}

// CollectCC61 activity method.
func (a *ComplianceActivities) CollectCC61(ctx context.Context) (*ControlEvidence, error) {
	ev, err := a.SOC2Collector.CollectCC61(ctx)
	if err != nil {
		return nil, err
	}
	_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	return ev, nil
}

func (a *ComplianceActivities) CollectCC66(ctx context.Context) (*ControlEvidence, error) {
	ev, err := a.SOC2Collector.CollectCC66(ctx)
	if err != nil {
		return nil, err
	}
	_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	return ev, nil
}

func (a *ComplianceActivities) CollectCC72(ctx context.Context) (*ControlEvidence, error) {
	ev, err := a.SOC2Collector.CollectCC72(ctx)
	if err != nil {
		return nil, err
	}
	_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	return ev, nil
}

func (a *ComplianceActivities) CollectCC92(ctx context.Context) (*ControlEvidence, error) {
	ev, err := a.SOC2Collector.CollectCC92(ctx)
	if err != nil {
		return nil, err
	}
	_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	return ev, nil
}

func (a *ComplianceActivities) CollectPI14(ctx context.Context) (*ControlEvidence, error) {
	ev, err := a.SOC2Collector.CollectPI14(ctx)
	if err != nil {
		return nil, err
	}
	_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	return ev, nil
}

func (a *ComplianceActivities) CollectISO27001(ctx context.Context) error {
	if a.ISO27001Collector == nil {
		return nil
	}
	evidences, err := a.ISO27001Collector.CollectAll(ctx)
	if err != nil {
		return err
	}
	for _, ev := range evidences {
		_ = a.SOC2Collector.PersistEvidence(ctx, ev)
	}
	return nil
}

// ScheduleComplianceCron registers the daily compliance evidence cron with Temporal.
func ScheduleComplianceCron(tc temporalclient.Client, taskQueue string) error {
	opts := temporalclient.StartWorkflowOptions{
		ID:           ComplianceCronWorkflowID,
		TaskQueue:    taskQueue,
		CronSchedule: ComplianceCronSchedule,
	}

	_, err := tc.ExecuteWorkflow(context.Background(), opts,
		ComplianceEvidenceWorkflow, ComplianceWorkflowInput{})
	if err != nil {
		return fmt.Errorf("schedule compliance cron: %w", err)
	}
	return nil
}
