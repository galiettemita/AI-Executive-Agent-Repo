package executor

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/audit"
	"github.com/brevio/brevio/internal/control"
	"github.com/google/uuid"
)

// ReceiptValidator validates and consumes authorization receipts.
type ReceiptValidator interface {
	ValidateReceipt(receiptID, workspaceID, toolKey string) error
	ConsumeReceipt(ctx context.Context, receiptID string) error
}

// ProdService wraps Service with pgx-backed persistence and receipt enforcement.
// In production, all authoritative state flows through the database.
type ProdService struct {
	*Service
	repo     ToolExecutionRepository
	receipts ReceiptValidator
}

// NewProdService creates a production executor service with persistent storage
// and receipt enforcement.
func NewProdService(repo ToolExecutionRepository, receipts ReceiptValidator) *ProdService {
	return &ProdService{
		Service:  NewService(),
		repo:     repo,
		receipts: receipts,
	}
}

// Simulate validates SSRF, records the execution in the database with
// idempotency enforcement, and emits an audit entry.
func (ps *ProdService) Simulate(ctx context.Context, req ExecutionRequest) (ToolExecution, error) {
	if err := ps.Service.validateSSRF(req.TargetURL); err != nil {
		ps.persistAudit(ctx, "BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, err
	}

	exec, created, err := ps.recordExecutionPersistent(ctx, req, PhaseSimulate)
	if err != nil {
		return ToolExecution{}, err
	}
	if created {
		ps.persistAudit(ctx, "BREVIO.hands.tool.simulated.v1", exec.ID.String())
	}
	return exec, nil
}

// Commit validates the authorization receipt, records the execution in the
// database, increments side effects atomically, creates a trust receipt, and
// emits audit entries. Refuses execution without a valid receipt.
func (ps *ProdService) Commit(ctx context.Context, req ExecutionRequest, receiptID string) (ToolExecution, TrustReceipt, error) {
	// D3: deny-by-default — no execution without receipt.
	if err := ps.receipts.ValidateReceipt(receiptID, req.WorkspaceID, req.ToolKey); err != nil {
		ps.persistAudit(ctx, "BREVIO.trust.receipt.denied.v1", fmt.Sprintf("receipt=%s err=%s", receiptID, err.Error()))
		return ToolExecution{}, TrustReceipt{}, err
	}

	if err := ps.Service.validateSSRF(req.TargetURL); err != nil {
		ps.persistAudit(ctx, "BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, TrustReceipt{}, err
	}

	exec, created, err := ps.recordExecutionPersistent(ctx, req, PhaseCommit)
	if err != nil {
		return ToolExecution{}, TrustReceipt{}, err
	}

	// Idempotent: return existing receipt if execution already committed.
	if !created {
		existingReceipt, _ := ps.repo.GetReceiptByExecution(ctx, exec.ID)
		if existingReceipt != nil {
			return exec, *existingReceipt, nil
		}
		return exec, TrustReceipt{}, nil
	}

	// Consume the authorization receipt (one-time use).
	if err := ps.receipts.ConsumeReceipt(ctx, receiptID); err != nil {
		ps.persistAudit(ctx, "BREVIO.trust.receipt.consume_failed.v1", fmt.Sprintf("receipt=%s err=%s", receiptID, err.Error()))
		return ToolExecution{}, TrustReceipt{}, err
	}

	ps.persistAudit(ctx, "BREVIO.hands.tool.committed.v1", exec.ID.String())

	// Atomic side effect increment in DB.
	before, _, err := ps.repo.IncrementSideEffect(ctx, req.WorkspaceID, req.ToolKey)
	if err != nil {
		return ToolExecution{}, TrustReceipt{}, fmt.Errorf("increment side effect: %w", err)
	}

	receipt := TrustReceipt{
		ID:               uuid.Must(uuid.NewV7()),
		ToolExecutionID:  exec.ID,
		UndoInstructions: "Use compensating action for " + req.ToolKey,
		CreatedAt:        ps.Service.nowFunc(),
	}

	if err := ps.repo.InsertReceipt(ctx, &receipt); err != nil {
		return ToolExecution{}, TrustReceipt{}, fmt.Errorf("persist receipt: %w", err)
	}

	ps.persistAudit(ctx, "BREVIO.trust.receipt.created.v1", receipt.ID.String())
	ps.persistAudit(ctx, "BREVIO.trust.evidence.attached.v1", receipt.ID.String())
	ps.appendMutationAuditProd(req, exec, receipt, before, before+1)

	return exec, receipt, nil
}

// SideEffectCount retrieves the side effect count from the database.
func (ps *ProdService) SideEffectCount(ctx context.Context, workspaceID, toolKey string) (int, error) {
	return ps.repo.GetSideEffectCount(ctx, workspaceID, toolKey)
}

func (ps *ProdService) recordExecutionPersistent(ctx context.Context, req ExecutionRequest, phase ExecutionPhase) (ToolExecution, bool, error) {
	if req.WorkspaceID == "" || req.ToolKey == "" {
		return ToolExecution{}, false, fmt.Errorf("workspace_id and tool_key are required")
	}
	if req.IsMCP && req.MCPServerID == "" {
		return ToolExecution{}, false, fmt.Errorf("mcp_server_id is required for mcp execution")
	}
	provenance := req.ContentProvenance
	if provenance == "" {
		if req.IsMCP {
			provenance = "mcp_result"
		} else {
			provenance = "native_result"
		}
	}
	if provenance != "native_result" && provenance != "mcp_result" {
		return ToolExecution{}, false, fmt.Errorf("invalid content_provenance: %s", provenance)
	}

	logicalHash := logicalActionHash(req.WorkspaceID, req.ToolKey, req.Action)
	idempotencyKey := req.WorkspaceID + "::" + req.ToolKey + "::" + logicalHash + "::" + string(phase)

	exec := &ToolExecution{
		ID:                uuid.Must(uuid.NewV7()),
		Phase:             phase,
		WorkspaceID:       req.WorkspaceID,
		ToolKey:           req.ToolKey,
		LogicalAction:     req.Action,
		IdempotencyKey:    idempotencyKey,
		Provider:          req.Provider,
		IsMCP:             req.IsMCP,
		MCPServerID:       req.MCPServerID,
		ContentProvenance: provenance,
		PIIContent:        req.PIIContent,
		CreatedAt:         ps.Service.nowFunc(),
	}

	return ps.repo.InsertExecution(ctx, exec)
}

func (ps *ProdService) persistAudit(ctx context.Context, eventType, payload string) {
	payload = minimizeAuditPayload(payload)
	entryID := uuid.Must(uuid.NewV7())

	ps.Service.mu.Lock()
	prevHash := ps.Service.lastAuditHash
	entryHash := hashAudit(entryID.String() + eventType + payload + prevHash)
	ps.Service.lastAuditHash = entryHash
	ps.Service.mu.Unlock()

	entry := &AuditLogEntry{
		ID:        entryID,
		EventType: eventType,
		Payload:   payload,
		Hash:      entryHash,
		PrevHash:  prevHash,
		CreatedAt: ps.Service.nowFunc(),
	}
	_ = ps.repo.InsertAuditEntry(ctx, entry)
}

func (ps *ProdService) appendMutationAuditProd(req ExecutionRequest, exec ToolExecution, receipt TrustReceipt, beforeSideEffects, afterSideEffects int) {
	ps.Service.mu.Lock()
	mutationAudit := ps.Service.mutationAudit
	ps.Service.mu.Unlock()
	if mutationAudit == nil {
		return
	}
	mutationAudit.AppendMutation(audit.MutationInput{
		WorkspaceID: req.WorkspaceID,
		Action:      "hands.skill.execute.commit",
		Resource:    "skill:" + req.ToolKey,
		Before: map[string]any{
			"side_effect_count": beforeSideEffects,
		},
		After: map[string]any{
			"execution_id":      exec.ID.String(),
			"trust_receipt_id":  receipt.ID.String(),
			"side_effect_count": afterSideEffects,
		},
	})
}

// Ensure control.DurableReceiptService satisfies ReceiptValidator at compile time.
var _ ReceiptValidator = (*control.DurableReceiptService)(nil)
