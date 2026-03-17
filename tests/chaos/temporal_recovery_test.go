//go:build chaos

package chaos

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func chaosTestWorkflow(ctx workflow.Context) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 5},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var result string
	err := workflow.ExecuteActivity(ctx, chaosTestActivity).Get(ctx, &result)
	return result, err
}

func chaosTestActivity(_ context.Context) (string, error) {
	time.Sleep(2 * time.Second)
	return "ok", nil
}

func TestChaos_TemporalWorkerRecovery(t *testing.T) {
	temporalHost := os.Getenv("TEMPORAL_HOST")
	if temporalHost == "" {
		t.Skip("TEMPORAL_HOST not set — skipping chaos test (requires live stack)")
	}

	c, err := client.NewClient(client.Options{HostPort: temporalHost})
	if err != nil {
		t.Fatalf("failed to create Temporal client: %v", err)
	}
	defer c.Close()

	wfID := fmt.Sprintf("chaos-temporal-%d", time.Now().UnixNano())

	w1 := worker.New(c, "chaos-task-queue", worker.Options{})
	w1.RegisterWorkflow(chaosTestWorkflow)
	w1.RegisterActivity(chaosTestActivity)
	if err := w1.Start(); err != nil {
		t.Fatalf("worker 1 failed to start: %v", err)
	}

	run, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{ID: wfID, TaskQueue: "chaos-task-queue"},
		chaosTestWorkflow)
	if err != nil {
		t.Fatalf("failed to start workflow: %v", err)
	}
	t.Logf("Workflow started: ID=%s RunID=%s", run.GetID(), run.GetRunID())

	time.Sleep(500 * time.Millisecond)

	w1.Stop()
	t.Logf("Worker killed at %s (os.Kill — in-process via worker.Stop())",
		time.Now().Format(time.RFC3339))

	time.Sleep(5 * time.Second)

	w2 := worker.New(c, "chaos-task-queue", worker.Options{})
	w2.RegisterWorkflow(chaosTestWorkflow)
	w2.RegisterActivity(chaosTestActivity)
	if err := w2.Start(); err != nil {
		t.Fatalf("new worker failed to start: %v", err)
	}
	defer w2.Stop()
	t.Logf("New worker started — Temporal should resume workflow via auto-retry")

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, descErr := c.DescribeWorkflowExecution(
			context.Background(), run.GetID(), run.GetRunID())
		if descErr != nil {
			t.Logf("DescribeWorkflowExecution error (retrying): %v", descErr)
			time.Sleep(2 * time.Second)
			continue
		}
		status := resp.WorkflowExecutionInfo.Status
		switch status {
		case enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:
			t.Logf("Workflow COMPLETED after worker recovery — Temporal auto-retry verified")
			return
		case enums.WORKFLOW_EXECUTION_STATUS_FAILED,
			enums.WORKFLOW_EXECUTION_STATUS_TERMINATED,
			enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
			t.Fatalf("Workflow reached terminal failure state: %s — expected COMPLETED", status)
		}
		t.Logf("Workflow status: %s — waiting...", status)
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("Workflow did not complete within 60s after worker restart — " +
		"Temporal auto-retry failed")
}
