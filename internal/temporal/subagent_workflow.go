package temporal

import (
	"fmt"
	"strings"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/subagent"
)

// SubAgentOrchestratorWorkflow fans out a multi-domain plan to parallel child
// MessageProcessingWorkflows, collects their results, and returns merged context.
func SubAgentOrchestratorWorkflow(ctx workflow.Context, in subagent.OrchestratorInput) (*subagent.OrchestratorResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SubAgentOrchestratorWorkflow started",
		"workspace_id", in.WorkspaceID, "tool_count", len(in.ToolKeys))

	baseAO := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporalSDK.RetryPolicy{
			MaximumAttempts: 2, InitialInterval: 2 * time.Second, BackoffCoefficient: 2.0,
		},
	}
	baseCtx := workflow.WithActivityOptions(ctx, baseAO)

	// Step 1: Autonomy Gate — requires A3+
	var autonomyResult subagent.CheckAutonomyResult
	if err := workflow.ExecuteActivity(baseCtx, "CheckSubAgentAutonomyActivity",
		subagent.CheckAutonomyInput{WorkspaceID: in.WorkspaceID, RequiredTier: "A3"},
	).Get(ctx, &autonomyResult); err != nil {
		return nil, fmt.Errorf("SubAgentOrchestrator: autonomy check failed: %w", err)
	}

	if !autonomyResult.Permitted {
		return &subagent.OrchestratorResult{
			TerminalState: fmt.Sprintf("SEQUENTIAL_FALLBACK:tier=%s", autonomyResult.CurrentTier),
		}, nil
	}

	// Step 2: Task Decomposition
	var decomp subagent.DecompositionResult
	if err := workflow.ExecuteActivity(baseCtx, "DecomposeSubTasksActivity",
		subagent.DecomposeInput{Intent: in.Intent, ToolKeys: in.ToolKeys},
	).Get(ctx, &decomp); err != nil {
		return nil, fmt.Errorf("SubAgentOrchestrator: decompose failed: %w", err)
	}

	if !decomp.CanParallelize || len(decomp.SubTasks) < 2 {
		return &subagent.OrchestratorResult{
			TerminalState: "SEQUENTIAL_PREFERRED:" + decomp.Reason,
		}, nil
	}

	// Step 3: Fan-Out by Priority Bucket
	var allResults []subagent.SubTaskResult
	launched := 0

	buckets := subagent.SplitByPriority(decomp.SubTasks)
	for bucketIdx, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}

		type pendingTask struct {
			subTaskID string
			domain    string
			future    workflow.ChildWorkflowFuture
		}
		pending := make([]pendingTask, 0, len(bucket))

		for taskIdx, subTask := range bucket {
			childWorkflowID := fmt.Sprintf("%s-orch-b%d-t%d", in.MessageID, bucketIdx, taskIdx)
			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID:               childWorkflowID,
				TaskQueue:                "brevio-main",
				WorkflowExecutionTimeout: 5 * time.Minute,
				RetryPolicy:              &temporalSDK.RetryPolicy{MaximumAttempts: 1},
			})

			childInput := MessageProcessingWorkflowInput{
				MessageID: childWorkflowID, WorkspaceID: in.WorkspaceID,
				ChannelType: "internal", RawPayload: subTask.Intent,
				IdempotencyKey: childWorkflowID, Tier: in.Tier,
			}

			future := workflow.ExecuteChildWorkflow(childCtx, MessageProcessingWorkflow, childInput)
			pending = append(pending, pendingTask{subTaskID: subTask.ID, domain: string(subTask.Domain), future: future})
			launched++
		}

		// Fan-in: wait for all tasks in this bucket before the next
		for _, p := range pending {
			var childResult MessageProcessingWorkflowResult
			err := p.future.Get(ctx, &childResult)

			res := subagent.SubTaskResult{SubTaskID: p.subTaskID, Domain: p.domain}
			if err != nil {
				res.TerminalState = "FAILED"
				res.Error = err.Error()
			} else {
				res.TerminalState = childResult.TerminalState
				res.ResponsePayload = childResult.ResponsePayload
			}
			allResults = append(allResults, res)
		}
	}

	// Step 4: Merge results
	complete, failed := 0, 0
	for _, r := range allResults {
		if r.TerminalState == "FAILED" {
			failed++
		} else {
			complete++
		}
	}

	return &subagent.OrchestratorResult{
		SubTasksLaunched: launched, SubTasksComplete: complete, SubTasksFailed: failed,
		MergedContext: buildMergedContext(allResults), Results: allResults, TerminalState: "COMPLETED",
	}, nil
}

func buildMergedContext(results []subagent.SubTaskResult) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("PARALLEL SUB-AGENT RESULTS:\n")
	for _, r := range results {
		if r.TerminalState == "FAILED" {
			sb.WriteString(fmt.Sprintf("[%s/%s] FAILED: %s\n", r.Domain, r.SubTaskID, r.Error))
		} else if r.ResponsePayload != "" {
			sb.WriteString(fmt.Sprintf("[%s/%s] %s\n", r.Domain, r.SubTaskID, r.ResponsePayload))
		}
	}
	return sb.String()
}
