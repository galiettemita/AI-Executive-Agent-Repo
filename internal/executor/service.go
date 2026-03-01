package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
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
	WorkspaceID       string
	ToolKey           string
	Action            string
	Provider          string
	TargetURL         string
	IsMCP             bool
	MCPServerID       string
	ContentProvenance string
	PIIContent        bool
}

type ToolExecution struct {
	ID                uuid.UUID
	Phase             ExecutionPhase
	WorkspaceID       string
	ToolKey           string
	LogicalAction     string
	IdempotencyKey    string
	Provider          string
	IsMCP             bool
	MCPServerID       string
	ContentProvenance string
	PIIContent        bool
	CreatedAt         time.Time
}

type TrustReceipt struct {
	ID               uuid.UUID
	ToolExecutionID  uuid.UUID
	UndoInstructions string
	CreatedAt        time.Time
}

type SynthesisEvidenceReceipt struct {
	ID            uuid.UUID
	WorkspaceID   string
	IngressTurnID string
	CreatedAt     time.Time
}

type SynthesisEvidenceItem struct {
	ReceiptID uuid.UUID
	Evidence  string
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

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Service struct {
	mu            sync.Mutex
	sideEffects   map[string]int
	executions    []ToolExecution
	receipts      []TrustReceipt
	receiptByExec map[uuid.UUID]TrustReceipt
	synthesis     []SynthesisEvidenceReceipt
	synthItems    []SynthesisEvidenceItem
	audit         []AuditLogEntry
	lastAuditHash string
	idempotency   map[string]ToolExecution
	circuits      map[string]CircuitState
	l1Cache       map[string]string
	l2Cache       map[string]string
	l3Cache       map[string]string
	resolver      ipResolver
	nowFunc       func() time.Time
}

func NewService() *Service {
	return &Service{
		sideEffects:   map[string]int{},
		executions:    []ToolExecution{},
		receipts:      []TrustReceipt{},
		receiptByExec: map[uuid.UUID]TrustReceipt{},
		synthesis:     []SynthesisEvidenceReceipt{},
		synthItems:    []SynthesisEvidenceItem{},
		audit:         []AuditLogEntry{},
		idempotency:   map[string]ToolExecution{},
		circuits:      map[string]CircuitState{},
		l1Cache:       map[string]string{},
		l2Cache:       map[string]string{},
		l3Cache:       map[string]string{},
		resolver:      net.DefaultResolver,
		nowFunc:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetNowFunc(nowFunc func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nowFunc == nil {
		s.nowFunc = func() time.Time { return time.Now().UTC() }
		return
	}
	s.nowFunc = nowFunc
}

func logicalActionHash(workspaceID, toolKey, action string) string {
	sum := sha256.Sum256([]byte(workspaceID + "::" + toolKey + "::" + action))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Simulate(req ExecutionRequest) (ToolExecution, error) {
	if err := s.validateSSRF(req.TargetURL); err != nil {
		s.emitAudit("BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, err
	}
	exec, created, err := s.recordExecution(req, PhaseSimulate)
	if err != nil {
		return ToolExecution{}, err
	}
	if created {
		s.emitAudit("BREVIO.hands.tool.simulated.v1", exec.ID.String())
	}
	return exec, nil
}

func (s *Service) Commit(req ExecutionRequest) (ToolExecution, TrustReceipt, error) {
	if err := s.validateSSRF(req.TargetURL); err != nil {
		s.emitAudit("BREVIO.security.ssrf.blocked.v1", err.Error())
		return ToolExecution{}, TrustReceipt{}, err
	}

	exec, created, err := s.recordExecution(req, PhaseCommit)
	if err != nil {
		return ToolExecution{}, TrustReceipt{}, err
	}

	if !created {
		s.mu.Lock()
		receipt, ok := s.receiptByExec[exec.ID]
		s.mu.Unlock()
		if ok {
			return exec, receipt, nil
		}
		return exec, TrustReceipt{}, nil
	}

	s.emitAudit("BREVIO.hands.tool.committed.v1", exec.ID.String())

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
	s.receiptByExec[exec.ID] = receipt
	s.mu.Unlock()
	s.emitAudit("BREVIO.trust.receipt.created.v1", receipt.ID.String())
	s.emitAudit("BREVIO.trust.evidence.attached.v1", receipt.ID.String())

	return exec, receipt, nil
}

func (s *Service) recordExecution(req ExecutionRequest, phase ExecutionPhase) (ToolExecution, bool, error) {
	if req.WorkspaceID == "" || req.ToolKey == "" {
		return ToolExecution{}, false, fmt.Errorf("workspace_id and tool_key are required")
	}
	if req.IsMCP && strings.TrimSpace(req.MCPServerID) == "" {
		return ToolExecution{}, false, fmt.Errorf("mcp_server_id is required for mcp execution")
	}
	provenance := strings.TrimSpace(req.ContentProvenance)
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

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotency[idempotencyKey]; ok {
		return existing, false, nil
	}

	exec := ToolExecution{
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
		CreatedAt:         s.nowFunc(),
	}
	s.executions = append(s.executions, exec)
	s.idempotency[idempotencyKey] = exec
	return exec, true, nil
}

func (s *Service) emitAudit(eventType, payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload = minimizeAuditPayload(payload)
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

var blockedCIDRStrings = []string{
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"100.64.0.0/10",
	"198.18.0.0/15",
	"0.0.0.0/8",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"fd00::/8",
}

var blockedCIDRs = func() []netip.Prefix {
	out := make([]netip.Prefix, 0, len(blockedCIDRStrings))
	for _, cidr := range blockedCIDRStrings {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		out = append(out, prefix)
	}
	return out
}()

func (s *Service) validateSSRF(target string) error {
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
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("blocked host: %s", host)
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return validateBlockedIP(host, ip)
	}

	resolver := s.resolver
	if resolver == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	resolved, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		// Fail open on DNS resolution failure; hostname checks above still apply.
		return nil
	}
	for _, ipAddr := range resolved {
		if err := validateBlockedIP(host, ipAddr.IP); err != nil {
			return err
		}
	}
	return nil
}

func validateBlockedIP(host string, ip net.IP) error {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return fmt.Errorf("blocked loopback address: %s", host)
	}
	if addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return fmt.Errorf("blocked private address: %s", host)
	}
	for _, prefix := range blockedCIDRs {
		if prefix.Contains(addr) {
			if addr.String() == "169.254.169.254" {
				return fmt.Errorf("blocked metadata address: %s", host)
			}
			return fmt.Errorf("blocked private address: %s", host)
		}
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

func (s *Service) EmitSynthesisEvidence(workspaceID, ingressTurnID string, evidenceItems []string) (SynthesisEvidenceReceipt, []SynthesisEvidenceItem, error) {
	if workspaceID == "" {
		return SynthesisEvidenceReceipt{}, nil, fmt.Errorf("workspace_id is required")
	}
	if ingressTurnID == "" {
		return SynthesisEvidenceReceipt{}, nil, fmt.Errorf("ingress_turn_id is required")
	}
	if len(evidenceItems) == 0 {
		return SynthesisEvidenceReceipt{}, nil, fmt.Errorf("at least one evidence item is required")
	}
	receipt := SynthesisEvidenceReceipt{
		ID:            uuid.Must(uuid.NewV7()),
		WorkspaceID:   workspaceID,
		IngressTurnID: ingressTurnID,
		CreatedAt:     s.nowFunc(),
	}
	items := make([]SynthesisEvidenceItem, 0, len(evidenceItems))
	for _, evidence := range evidenceItems {
		items = append(items, SynthesisEvidenceItem{
			ReceiptID: receipt.ID,
			Evidence:  strings.TrimSpace(evidence),
		})
	}
	s.mu.Lock()
	s.synthesis = append(s.synthesis, receipt)
	s.synthItems = append(s.synthItems, items...)
	s.mu.Unlock()
	s.emitAudit("BREVIO.trust.synthesis_evidence.created.v1", receipt.ID.String())
	return receipt, items, nil
}

func (s *Service) SynthesisReceiptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.synthesis)
}

func (s *Service) CachePut(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.l1Cache[key] = value
	s.l2Cache[key] = value
	s.l3Cache[key] = value
}

func (s *Service) CacheGet(key string) (value string, hit bool, layer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value, ok := s.l1Cache[key]; ok {
		return value, true, "L1"
	}
	if value, ok := s.l2Cache[key]; ok {
		s.l1Cache[key] = value
		return value, true, "L2"
	}
	if value, ok := s.l3Cache[key]; ok {
		s.l2Cache[key] = value
		s.l1Cache[key] = value
		return value, true, "L3"
	}
	return "", false, ""
}

func (s *Service) ExecuteWithCircuitProtection(req ExecutionRequest) (string, error) {
	if s.CircuitOpen(req.WorkspaceID, req.Provider) {
		return "fallback_response", nil
	}
	if _, _, err := s.Commit(req); err != nil {
		s.RecordProviderFailure(req.WorkspaceID, req.Provider)
		return "", err
	}
	return "committed", nil
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

func (s *Service) AuditEntries() []AuditLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditLogEntry, len(s.audit))
	copy(out, s.audit)
	return out
}

func (s *Service) Executions() []ToolExecution {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ToolExecution, len(s.executions))
	copy(out, s.executions)
	return out
}

var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
var bearerPattern = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._-]+`)

func minimizeAuditPayload(payload string) string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return trimmed
	}
	trimmed = emailPattern.ReplaceAllString(trimmed, "[redacted_email]")
	trimmed = bearerPattern.ReplaceAllString(trimmed, "Bearer [redacted_token]")
	if len(trimmed) > 256 {
		return trimmed[:256]
	}
	return trimmed
}
