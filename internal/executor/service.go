package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ExecutionPhase string

const (
	PhaseSimulate ExecutionPhase = "simulate"
	PhaseCommit   ExecutionPhase = "commit"
)

type ExecutionRequest struct {
	WorkspaceID string
	ToolKey     string
	Action      string
	Provider    string
	TargetURL   string
}

type ToolExecution struct {
	ID             uuid.UUID
	Phase          ExecutionPhase
	WorkspaceID    string
	ToolKey        string
	LogicalAction  string
	IdempotencyKey string
	CreatedAt      time.Time
}

type TrustReceipt struct {
	ID               uuid.UUID
	ToolExecutionID  uuid.UUID
	UndoInstructions string
	CreatedAt        time.Time
}

type AuditLogEntry struct {
	ID        uuid.UUID
	EventType string
	Payload   string
	Hash      string
	PrevHash  string
	CreatedAt time.Time
}

type CircuitState struct {
	OpenUntil time.Time
	Failures  []time.Time
}

type Service struct {
	mu            sync.Mutex
	sideEffects   map[string]int
	executions    []ToolExecution
	receipts      []TrustReceipt
	audit         []AuditLogEntry
	lastAuditHash string
	idempotency   map[string]ToolExecution
	circuits      map[string]CircuitState
	nowFunc       func() time.Time
}

func NewService() *Service {
	return &Service{
		sideEffects: map[string]int{},
		executions:  []ToolExecution{},
		receipts:    []TrustReceipt{},
		audit:       []AuditLogEntry{},
		idempotency: map[string]ToolExecution{},
		circuits:    map[string]CircuitState{},
		nowFunc:     func() time.Time { return time.Now().UTC() },
	}
}

func logicalActionHash(workspaceID, toolKey, action string) string {
	sum := sha256.Sum256([]byte(workspaceID + "::" + toolKey + "::" + action))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Simulate(req ExecutionRequest) (ToolExecution, error) {
	if err := validateSSRF(req.TargetURL); err != nil {
		s.emitAudit("BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, err
	}
	return s.recordExecution(req, PhaseSimulate)
}

func (s *Service) Commit(req ExecutionRequest) (ToolExecution, TrustReceipt, error) {
	if err := validateSSRF(req.TargetURL); err != nil {
		s.emitAudit("BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, TrustReceipt{}, err
	}

	exec, err := s.recordExecution(req, PhaseCommit)
	if err != nil {
		return ToolExecution{}, TrustReceipt{}, err
	}

	s.mu.Lock()
	s.sideEffects[req.WorkspaceID+"::"+req.ToolKey]++
	s.mu.Unlock()

	receipt := TrustReceipt{
		ID:               uuid.Must(uuid.NewV7()),
		ToolExecutionID:  exec.ID,
		UndoInstructions: "Use compensating action for " + req.ToolKey,
		CreatedAt:        s.nowFunc(),
	}

	s.mu.Lock()
	s.receipts = append(s.receipts, receipt)
	s.mu.Unlock()
	s.emitAudit("BREVIO.trust.receipt.created.v1", receipt.ID.String())

	return exec, receipt, nil
}

func (s *Service) recordExecution(req ExecutionRequest, phase ExecutionPhase) (ToolExecution, error) {
	if req.WorkspaceID == "" || req.ToolKey == "" {
		return ToolExecution{}, fmt.Errorf("workspace_id and tool_key are required")
	}
	logicalHash := logicalActionHash(req.WorkspaceID, req.ToolKey, req.Action)
	idempotencyKey := req.WorkspaceID + "::" + req.ToolKey + "::" + logicalHash + "::" + string(phase)

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotency[idempotencyKey]; ok {
		return existing, nil
	}

	exec := ToolExecution{
		ID:             uuid.Must(uuid.NewV7()),
		Phase:          phase,
		WorkspaceID:    req.WorkspaceID,
		ToolKey:        req.ToolKey,
		LogicalAction:  req.Action,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      s.nowFunc(),
	}
	s.executions = append(s.executions, exec)
	s.idempotency[idempotencyKey] = exec
	return exec, nil
}

func (s *Service) emitAudit(eventType, payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entryID := uuid.Must(uuid.NewV7())
	entryHash := hashAudit(entryID.String() + eventType + payload + s.lastAuditHash)
	entry := AuditLogEntry{
		ID:        entryID,
		EventType: eventType,
		Payload:   payload,
		Hash:      entryHash,
		PrevHash:  s.lastAuditHash,
		CreatedAt: s.nowFunc(),
	}
	s.audit = append(s.audit, entry)
	s.lastAuditHash = entryHash
}

func hashAudit(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

var blockedPrefixes = []string{"169.254.169.254", "127.", "::1"}

func validateSSRF(target string) error {
	if target == "" {
		return nil
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target url: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(host, prefix) {
			return fmt.Errorf("blocked host: %s", host)
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}
	if addr.IsLoopback() {
		return fmt.Errorf("blocked loopback address: %s", host)
	}
	return nil
}

func circuitKey(workspaceID, provider string) string {
	return workspaceID + "::" + provider
}

func (s *Service) RecordProviderFailure(workspaceID, provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	key := circuitKey(workspaceID, provider)
	state := s.circuits[key]
	fresh := make([]time.Time, 0, len(state.Failures)+1)
	for _, ts := range state.Failures {
		if now.Sub(ts) <= 60*time.Second {
			fresh = append(fresh, ts)
		}
	}
	fresh = append(fresh, now)
	state.Failures = fresh
	if len(fresh) >= 5 {
		state.OpenUntil = now.Add(300 * time.Second)
	}
	s.circuits[key] = state
}

func (s *Service) CircuitOpen(workspaceID, provider string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.circuits[circuitKey(workspaceID, provider)]
	if state.OpenUntil.IsZero() {
		return false
	}
	if s.nowFunc().After(state.OpenUntil) {
		state.OpenUntil = time.Time{}
		state.Failures = nil
		s.circuits[circuitKey(workspaceID, provider)] = state
		return false
	}
	return true
}

func (s *Service) SideEffectCount(workspaceID, toolKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sideEffects[workspaceID+"::"+toolKey]
}

func (s *Service) TrustReceiptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.receipts)
}

func (s *Service) AuditCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.audit)
}
