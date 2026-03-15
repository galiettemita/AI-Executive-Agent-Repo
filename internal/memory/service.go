package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	StatusProposed          = "proposed"
	StatusNeedsConfirmation = "needs_confirmation"
	StatusActive            = "active"
	StatusSuperseded        = "superseded"
	StatusDeleted           = "deleted"
)

var validMemoryTypes = map[string]struct{}{
	"semantic":     {},
	"episodic":     {},
	"procedural":  {},
	"preference":   {},
	"rule":         {},
	"contact_fact": {},
	"task_fact":    {},
	"daily_log":    {},
	"heartbeat":    {},
}

var validDataClasses = map[string]struct{}{
	"public":       {},
	"internal":     {},
	"confidential": {},
	"restricted":   {},
}

var validSensitivityLabels = map[string]struct{}{
	"none":      {},
	"low":       {},
	"moderate":  {},
	"high":      {},
	"regulated": {},
}

var validContentTrust = map[string]struct{}{
	"trusted":   {},
	"untrusted": {},
	"mixed":     {},
}

type Item struct {
	ID                uuid.UUID
	WorkspaceID       string
	UserID            string
	MemoryType        string
	Status            string
	Body              string
	DataClass         string
	SensitivityLabel  string
	RetentionPolicyID string
	AllowedProcessors []string
	ContentTrust      string
	EmbeddingVersion  int
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Confidence is the certainty score 0.0-1.0.
	Confidence float64 `json:"confidence" db:"confidence"`

	// RetrievalCount is incremented on each successful retrieval (Ebbinghaus spacing).
	RetrievalCount int `json:"retrieval_count" db:"retrieval_count"`

	// BaseHalfLifeDays is the starting decay half-life. Immutable after initial write.
	BaseHalfLifeDays float64 `json:"base_half_life_days" db:"base_half_life_days"`

	// Embedding is the vector representation for semantic retrieval.
	Embedding []float32 `json:"-" db:"embedding"`

	// RelevanceScore is the retrieval-time similarity score. NOT persisted.
	RelevanceScore float64 `json:"relevance_score,omitempty" db:"-"`

	// LinkedItemIDs holds IDs of associatively linked items. Hydrated at read time.
	LinkedItemIDs []string `json:"linked_item_ids,omitempty" db:"-"`

	// LinkedItemStrengths holds cosine similarity for each linked item (parallel array).
	LinkedItemStrengths []float64 `json:"linked_item_strengths,omitempty" db:"-"`

	// ContradictsItemID is the ID of the item this one supersedes.
	ContradictsItemID *string `json:"contradicts_item_id,omitempty" db:"contradicts_item_id"`

	// IsContradicted is true when a newer item supersedes this one.
	IsContradicted bool `json:"is_contradicted,omitempty" db:"is_contradicted"`

	// ContradictionConfidence is the certainty of the contradiction (0.0-1.0).
	ContradictionConfidence float64 `json:"contradiction_confidence,omitempty" db:"contradiction_confidence"`
}

type WriteRequest struct {
	WorkspaceID       string
	UserID            string
	MemoryType        string
	Body              string
	DataClass         string
	SensitivityLabel  string
	RetentionPolicyID string
	AllowedProcessors []string
	ContentTrust      string
	ExpiresAt         *time.Time

	// Confidence is the certainty of this memory item.
	Confidence float64

	// BaseHalfLifeDays overrides the type-based default. 0 = use type default.
	BaseHalfLifeDays float64

	// OPA policy fields:
	WorkspaceTier      string  // "free", "pro", "business", "enterprise"
	TTLOverrideDays    float64 // 0 = use system default
	MemoryWriteBlocked bool    // legacy hard-block flag

	// ContradictsItemID optionally specifies an item this write supersedes.
	ContradictsItemID *string
}

// PolicyViolationError is returned when OPA denies a memory write.
type PolicyViolationError struct {
	Reasons []string // raw deny messages from OPA
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("memory write denied by policy: [%s]",
		strings.Join(e.Reasons, "; "))
}

// MetricLabels extracts the code prefix from each deny reason.
func (e *PolicyViolationError) MetricLabels() []string {
	labels := make([]string, 0, len(e.Reasons))
	for _, reason := range e.Reasons {
		if idx := strings.Index(reason, ":"); idx > 0 {
			labels = append(labels, strings.TrimSpace(reason[:idx]))
		}
	}
	return labels
}

// BuildOPAInput constructs the OPA input document from a WriteRequest.
func BuildOPAInput(req WriteRequest) map[string]any {
	processors := req.AllowedProcessors
	if processors == nil {
		processors = []string{}
	}
	contradictsExisting := req.ContradictsItemID != nil
	contradictsID := ""
	if req.ContradictsItemID != nil {
		contradictsID = *req.ContradictsItemID
	}
	return map[string]any{
		"workspace_id":         req.WorkspaceID,
		"user_id":              req.UserID,
		"memory_type":          req.MemoryType,
		"data_class":           req.DataClass,
		"sensitivity_label":    req.SensitivityLabel,
		"retention_policy_id":  req.RetentionPolicyID,
		"allowed_processors":   processors,
		"workspace_tier":       req.WorkspaceTier,
		"memory_write_blocked": req.MemoryWriteBlocked,
		"content_byte_size":    len(req.Body),
		"ttl_override_days":    req.TTLOverrideDays,
		"contradicts_existing": contradictsExisting,
		"contradicts_item_id":  contradictsID,
	}
}

type WritePolicy struct {
	RequireConfirmationForTypes map[string]struct{}
	BlockedDataClasses          map[string]struct{}
}

type Service struct {
	mu             sync.Mutex
	exclusionRules map[string][]string
	writePolicies  map[string]WritePolicy
	items          map[uuid.UUID]Item
	itemOrder      []uuid.UUID
	linkSvc        *LinkService // nil = auto-linking disabled
	kgSvc          KGExtractor  // nil = KG extraction disabled
	contradictionDetector *ContradictionDetector // nil = no contradiction detection
}

func NewService() *Service {
	return &Service{
		exclusionRules: map[string][]string{},
		writePolicies:  map[string]WritePolicy{},
		items:          map[uuid.UUID]Item{},
		itemOrder:      []uuid.UUID{},
	}
}

// defaultBaseHalfLifeDays returns the starting decay half-life for a memory type.
func defaultBaseHalfLifeDays(memType string) float64 {
	switch memType {
	case "rule", "preference":
		return 730.0
	case "procedural":
		return 180.0
	case "semantic":
		return 90.0
	case "fact", "episodic", "contact_fact", "task_fact":
		return 30.0
	case "daily_log":
		return 7.0
	case "heartbeat", "transient":
		return 0.1667
	default:
		return 30.0
	}
}

// defaultConfidence returns the certainty for a WriteRequest.
func defaultConfidence(req WriteRequest) float64 {
	if req.Confidence > 0 {
		return req.Confidence
	}
	return 1.0
}

// SetLinkService injects the link service for auto-linking after Write.
func (s *Service) SetLinkService(ls *LinkService) {
	s.linkSvc = ls
}

// KGExtractor is the interface for async knowledge graph triple extraction.
// Implemented by kg.Service. Defined here to avoid circular imports.
type KGExtractor interface {
	ExtractAndStore(ctx context.Context, req KGExtractionRequest)
}

// KGExtractionRequest carries the data needed for KG triple extraction.
type KGExtractionRequest struct {
	WorkspaceID string
	TurnID      string
	Content     string
	Role        string
}

// SetContradictionDetector injects the contradiction detector.
func (s *Service) SetContradictionDetector(d *ContradictionDetector) {
	s.contradictionDetector = d
}

// SetKGService injects the knowledge graph service for async extraction after Write.
func (s *Service) SetKGService(kg KGExtractor) {
	s.kgSvc = kg
}

func exclusionKey(workspaceID, userID string) string {
	return workspaceID + "::" + userID
}

func (s *Service) SetWritePolicy(workspaceID string, policy WritePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy.RequireConfirmationForTypes == nil {
		policy.RequireConfirmationForTypes = map[string]struct{}{}
	}
	if policy.BlockedDataClasses == nil {
		policy.BlockedDataClasses = map[string]struct{}{}
	}
	s.writePolicies[workspaceID] = policy
}

func (s *Service) AddExclusionRule(workspaceID, userID, rule string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exclusionRules[exclusionKey(workspaceID, userID)] = append(s.exclusionRules[exclusionKey(workspaceID, userID)], strings.ToLower(rule))
}

func (s *Service) Write(workspaceID, userID, memoryType, body string) (Item, error) {
	return s.WriteWithRequest(WriteRequest{
		WorkspaceID:       workspaceID,
		UserID:            userID,
		MemoryType:        memoryType,
		Body:              body,
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"brain", "control", "executor"},
		ContentTrust:      "mixed",
	})
}

func (s *Service) WriteWithRequest(req WriteRequest) (Item, error) {
	if req.WorkspaceID == "" || req.UserID == "" {
		return Item{}, fmt.Errorf("workspace_id and user_id required")
	}
	if strings.TrimSpace(req.Body) == "" {
		return Item{}, fmt.Errorf("body is required")
	}
	if _, ok := validMemoryTypes[req.MemoryType]; !ok {
		return Item{}, fmt.Errorf("invalid memory_type: %s", req.MemoryType)
	}
	if _, ok := validDataClasses[req.DataClass]; !ok {
		return Item{}, fmt.Errorf("invalid data_class: %s", req.DataClass)
	}
	if _, ok := validSensitivityLabels[req.SensitivityLabel]; !ok {
		return Item{}, fmt.Errorf("invalid sensitivity_label: %s", req.SensitivityLabel)
	}
	if _, ok := validContentTrust[req.ContentTrust]; !ok {
		return Item{}, fmt.Errorf("invalid content_trust: %s", req.ContentTrust)
	}
	if len(req.AllowedProcessors) == 0 {
		return Item{}, fmt.Errorf("allowed_processors must be non-empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rule := range s.exclusionRules[exclusionKey(req.WorkspaceID, req.UserID)] {
		if strings.Contains(strings.ToLower(req.Body), rule) {
			return Item{}, fmt.Errorf("memory write blocked by exclusion rule")
		}
	}

	policy := s.writePolicies[req.WorkspaceID]
	if _, blocked := policy.BlockedDataClasses[req.DataClass]; blocked {
		return Item{}, fmt.Errorf("memory write blocked by data_class policy")
	}

	status := StatusProposed
	if _, requires := policy.RequireConfirmationForTypes[req.MemoryType]; requires {
		status = StatusNeedsConfirmation
	}

	conf := defaultConfidence(req)
	halfLife := req.BaseHalfLifeDays
	if halfLife <= 0 {
		halfLife = defaultBaseHalfLifeDays(req.MemoryType)
	}

	item := Item{
		ID:                uuid.Must(uuid.NewV7()),
		WorkspaceID:       req.WorkspaceID,
		UserID:            req.UserID,
		MemoryType:        req.MemoryType,
		Status:            status,
		Body:              strings.TrimSpace(req.Body),
		DataClass:         req.DataClass,
		SensitivityLabel:  req.SensitivityLabel,
		RetentionPolicyID: req.RetentionPolicyID,
		AllowedProcessors: append([]string(nil), req.AllowedProcessors...),
		ContentTrust:      req.ContentTrust,
		EmbeddingVersion:  1,
		ExpiresAt:         req.ExpiresAt,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		Confidence:        conf,
		RetrievalCount:    0,
		BaseHalfLifeDays:  halfLife,
	}
	// Semantic deduplication: check for exact-match duplicates using ShouldMergeDuplicate.
	// Case-sensitive exact match only — case-insensitive dedup happens in Consolidate().
	trimmedBody := strings.TrimSpace(req.Body)
	activeCount := len(s.items)
	for _, existing := range s.items {
		if existing.WorkspaceID != req.WorkspaceID || existing.Status == StatusDeleted || existing.Status == StatusSuperseded {
			continue
		}
		if strings.TrimSpace(existing.Body) == trimmedBody {
			sameType := existing.MemoryType == req.MemoryType
			if ShouldMergeDuplicate(1.0, true, sameType, activeCount) {
				existing.Body = trimmedBody
				existing.UpdatedAt = time.Now().UTC()
				existing.RetrievalCount++
				if conf > 0 {
					existing.Confidence = conf
				}
				s.items[existing.ID] = existing
				return existing, nil
			}
		}
	}

	s.items[item.ID] = item
	s.itemOrder = append(s.itemOrder, item.ID)

	// Auto-link asynchronously — never block the write path.
	if s.linkSvc != nil {
		go func(it Item) {
			s.linkSvc.AutoLink(context.Background(), it)
		}(item)
	}

	// KG triple extraction — async, never blocks write path.
	if s.kgSvc != nil && item.Body != "" {
		go s.kgSvc.ExtractAndStore(context.Background(), KGExtractionRequest{
			WorkspaceID: item.WorkspaceID,
			TurnID:      item.ID.String(),
			Content:     item.Body,
			Role:        "user",
		})
	}

	// Contradiction detection — async. Only auto-detect if caller hasn't asserted.
	if s.contradictionDetector != nil && req.ContradictsItemID == nil && item.Body != "" {
		go s.contradictionDetector.DetectAndMark(context.Background(), item)
	}

	return item, nil
}

func (s *Service) TransitionStatus(itemID uuid.UUID, nextStatus string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return Item{}, fmt.Errorf("memory item not found")
	}
	if !isValidStatusTransition(item.Status, nextStatus) {
		return Item{}, fmt.Errorf("invalid status transition: %s -> %s", item.Status, nextStatus)
	}
	item.Status = nextStatus
	item.UpdatedAt = time.Now().UTC()
	s.items[itemID] = item
	return item, nil
}

func isValidStatusTransition(current, next string) bool {
	allowed := map[string]map[string]struct{}{
		StatusProposed: {
			StatusNeedsConfirmation: {},
			StatusActive:            {},
			StatusDeleted:           {},
		},
		StatusNeedsConfirmation: {
			StatusActive:  {},
			StatusDeleted: {},
		},
		StatusActive: {
			StatusSuperseded: {},
			StatusDeleted:    {},
		},
		StatusSuperseded: {
			StatusDeleted: {},
		},
	}
	if _, ok := allowed[current]; !ok {
		return false
	}
	_, ok := allowed[current][next]
	return ok
}

func (s *Service) GetItem(itemID uuid.UUID) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[itemID]
	return item, ok
}

func (s *Service) Retrieve(workspaceID string) []Item {
	return s.retrieveFiltered(workspaceID, nil)
}

func (s *Service) RetrieveWithTrust(workspaceID string, allowedTrust []string) []Item {
	trustFilter := map[string]struct{}{}
	for _, trust := range allowedTrust {
		trustFilter[trust] = struct{}{}
	}
	return s.retrieveFiltered(workspaceID, trustFilter)
}

func (s *Service) retrieveFiltered(workspaceID string, trustFilter map[string]struct{}) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Item{}
	for _, itemID := range s.itemOrder {
		item := s.items[itemID]
		if item.WorkspaceID != workspaceID {
			continue
		}
		if trustFilter != nil {
			if _, ok := trustFilter[item.ContentTrust]; !ok {
				continue
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Service) Consolidate(workspaceID string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	seen := map[string]uuid.UUID{}
	consolidated := []Item{}

	for _, itemID := range s.itemOrder {
		item := s.items[itemID]
		if item.WorkspaceID != workspaceID {
			continue
		}
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			item.Status = StatusDeleted
			item.UpdatedAt = now
			s.items[itemID] = item
			continue
		}
		if item.Status == StatusDeleted {
			continue
		}

		normalized := strings.TrimSpace(strings.ToLower(item.MemoryType + "::" + item.Body))
		if canonicalID, exists := seen[normalized]; exists {
			item.Status = StatusSuperseded
			item.UpdatedAt = now
			s.items[itemID] = item

			canonical := s.items[canonicalID]
			canonical.EmbeddingVersion++
			canonical.UpdatedAt = now
			s.items[canonicalID] = canonical
			continue
		}
		seen[normalized] = itemID
		consolidated = append(consolidated, item)
	}

	sort.Slice(consolidated, func(i, j int) bool {
		return consolidated[i].CreatedAt.Before(consolidated[j].CreatedAt)
	})
	return consolidated
}
