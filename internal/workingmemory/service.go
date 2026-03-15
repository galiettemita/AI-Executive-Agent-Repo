package workingmemory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Logger is the minimal logging contract.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Service is the application-level interface to the Working Memory tier.
type Service struct {
	repo   *Repository
	logger Logger
}

func NewService(repo *Repository, logger Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// GetOrCreate returns the working memory for a task.
// Creates a fresh item if none exists.
func (s *Service) GetOrCreate(ctx context.Context, workspaceID, taskID, userID string) (*Item, error) {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil {
		return nil, fmt.Errorf("working_memory.GetOrCreate: %w", err)
	}
	if item != nil {
		return item, nil
	}

	item = &Item{
		TaskID:           taskID,
		WorkspaceID:      workspaceID,
		UserID:           userID,
		ScratchPad:       make(map[string]any),
		PendingToolCalls: make(map[string]PendingToolCall),
		TTL:              DefaultTTL,
	}
	if err := s.repo.Upsert(ctx, item); err != nil {
		return nil, fmt.Errorf("working_memory.GetOrCreate: create: %w", err)
	}
	return item, nil
}

// MergeScratchPad merges updates into the scratch pad.
func (s *Service) MergeScratchPad(ctx context.Context, workspaceID, taskID string, updates map[string]any) error {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil {
		return fmt.Errorf("working_memory.MergeScratchPad: %w", err)
	}
	if item == nil {
		return fmt.Errorf("working_memory.MergeScratchPad: no item for task %s/%s", workspaceID, taskID)
	}
	if item.ScratchPad == nil {
		item.ScratchPad = make(map[string]any)
	}
	for k, v := range updates {
		item.ScratchPad[k] = v
	}
	return s.repo.Upsert(ctx, item)
}

// SetStage updates the current workflow stage and refreshes ContextSummary.
func (s *Service) SetStage(ctx context.Context, workspaceID, taskID, stage string) error {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil || item == nil {
		return fmt.Errorf("working_memory.SetStage: item not found for %s/%s", workspaceID, taskID)
	}
	item.Stage = stage
	item.ContextSummary = fmt.Sprintf("Active task is currently: %s", stage)
	return s.repo.Upsert(ctx, item)
}

// AddPendingToolCall records a dispatched tool call.
func (s *Service) AddPendingToolCall(ctx context.Context, workspaceID, taskID string, call PendingToolCall) error {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil || item == nil {
		return fmt.Errorf("working_memory.AddPendingToolCall: item not found")
	}
	if item.PendingToolCalls == nil {
		item.PendingToolCalls = make(map[string]PendingToolCall)
	}
	call.IssuedAt = time.Now().UTC()
	item.PendingToolCalls[call.ToolCallID] = call
	return s.repo.Upsert(ctx, item)
}

// ResolvePendingToolCall removes a resolved tool call. Idempotent.
func (s *Service) ResolvePendingToolCall(ctx context.Context, workspaceID, taskID, toolCallID string) error {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil || item == nil {
		return nil
	}
	delete(item.PendingToolCalls, toolCallID)
	return s.repo.Upsert(ctx, item)
}

// BindWorkflow links this item to a Temporal workflow and extends TTL.
func (s *Service) BindWorkflow(ctx context.Context, workspaceID, taskID, workflowID, runID string) error {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil || item == nil {
		return fmt.Errorf("working_memory.BindWorkflow: item not found")
	}
	item.WorkflowID = workflowID
	item.WorkflowRunID = runID
	item.TTL = WorkflowTTL
	return s.repo.Upsert(ctx, item)
}

// Complete evicts working memory when a task finishes. Best-effort.
func (s *Service) Complete(ctx context.Context, workspaceID, taskID string) {
	if err := s.repo.Evict(ctx, workspaceID, taskID); err != nil {
		s.logger.Error("working_memory.Complete: eviction failed",
			"workspace_id", workspaceID, "task_id", taskID, "error", err)
	}
}

// BuildContextSnippet returns formatted text for the context assembly working_memory slot.
func (s *Service) BuildContextSnippet(ctx context.Context, workspaceID, taskID string) (string, error) {
	item, err := s.repo.Get(ctx, workspaceID, taskID)
	if err != nil {
		return "", fmt.Errorf("working_memory.BuildContextSnippet: %w", err)
	}
	if item == nil {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("[Working Memory — Active Task State]\n")

	if item.Stage != "" {
		b.WriteString("Stage: " + item.Stage + "\n")
	}

	if len(item.PendingToolCalls) > 0 {
		names := make([]string, 0, len(item.PendingToolCalls))
		for _, tc := range item.PendingToolCalls {
			names = append(names, tc.ToolName)
		}
		b.WriteString("Pending: " + strings.Join(names, ", ") + "\n")
	}

	for k, v := range item.ScratchPad {
		if str, ok := v.(string); ok && str != "" {
			b.WriteString(k + ": " + str + "\n")
		}
	}

	result := b.String()
	if result == "[Working Memory — Active Task State]\n" {
		return "", nil
	}
	return result, nil
}
