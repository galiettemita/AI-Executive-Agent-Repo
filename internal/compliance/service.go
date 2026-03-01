package compliance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Framework struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Key         string `json:"key"`
	Status      string `json:"status"`
	VersionInt  int    `json:"version_int"`
}

type Evidence struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	FrameworkID string `json:"framework_id"`
	EventType   string `json:"event_type"`
	ArtifactURI string `json:"artifact_uri"`
	SHA256      string `json:"sha256"`
	CollectedAt string `json:"collected_at"`
}

type DSRRequest struct {
	ID            string `json:"id"`
	RequestID     string `json:"request_id"`
	WorkspaceID   string `json:"workspace_id"`
	UserID        string `json:"user_id"`
	SubjectUserID string `json:"subject_user_id"`
	RequestType   string `json:"request_type"`
	Status        string `json:"status"`
	DeadlineDate  string `json:"deadline_date"`
	DeadlineAt    string `json:"deadline_at"`
	CreatedAt     string `json:"created_at"`
	SLAAtRisk     bool   `json:"sla_at_risk"`
}

type DeletionReport struct {
	ID                 string `json:"id"`
	RequestID          string `json:"request_id"`
	WorkspaceID        string `json:"workspace_id"`
	SubjectUserID      string `json:"subject_user_id"`
	DatabasePurged     bool   `json:"database_purged"`
	CachePurged        bool   `json:"cache_purged"`
	ConnectorRevoked   bool   `json:"connector_revoked"`
	MCPOAuthRevoked    bool   `json:"mcp_oauth_revoked"`
	BackupRotationDays int    `json:"backup_rotation_days"`
	Irreversible       bool   `json:"irreversible"`
	CompletedAt        string `json:"completed_at"`
}

type RetentionPolicy struct {
	PolicyID        string
	Name            string
	RetentionPeriod time.Duration
	AppliesTo       []string
	ExpiryAction    string
}

type Service struct {
	mu         sync.RWMutex
	nextID     int
	frameworks map[string]Framework
	evidence   []Evidence
	dsr        map[string]DSRRequest
	deletions  map[string]DeletionReport
	now        func() time.Time
}

func DefaultRetentionPolicies() map[string]RetentionPolicy {
	year := 365 * 24 * time.Hour
	return map[string]RetentionPolicy{
		"RP-001": {
			PolicyID:        "RP-001",
			Name:            "Standard",
			RetentionPeriod: 2 * year,
			AppliesTo:       []string{"PUBLIC", "PRIVATE"},
			ExpiryAction:    "soft_delete_then_hard_purge_30d",
		},
		"RP-002": {
			PolicyID:        "RP-002",
			Name:            "Extended",
			RetentionPeriod: 7 * year,
			AppliesTo:       []string{"FINANCIAL"},
			ExpiryAction:    "soft_delete",
		},
		"RP-003": {
			PolicyID:        "RP-003",
			Name:            "Compliance",
			RetentionPeriod: 7 * year,
			AppliesTo:       []string{"AUDIT", "COMPLIANCE"},
			ExpiryAction:    "archive_glacier_never_hard_delete",
		},
		"RP-004": {
			PolicyID:        "RP-004",
			Name:            "Sensitive Short",
			RetentionPeriod: 1 * year,
			AppliesTo:       []string{"SENSITIVE", "HEALTH", "SECRETS"},
			ExpiryAction:    "secure_wipe_then_delete",
		},
		"RP-005": {
			PolicyID:        "RP-005",
			Name:            "Ephemeral",
			RetentionPeriod: 30 * 24 * time.Hour,
			AppliesTo:       []string{"TRANSCRIPTION", "CACHE"},
			ExpiryAction:    "hard_delete",
		},
		"RP-006": {
			PolicyID:        "RP-006",
			Name:            "Indefinite",
			RetentionPeriod: 0,
			AppliesTo:       []string{"USER_ACCOUNT", "WORKSPACE_CONFIG"},
			ExpiryAction:    "explicit_user_action_only",
		},
	}
}

func DefaultRetentionPolicyForDataClass(dataClass string) string {
	switch strings.ToUpper(strings.TrimSpace(dataClass)) {
	case "PUBLIC", "PRIVATE":
		return "RP-001"
	case "FINANCIAL":
		return "RP-002"
	case "SENSITIVE", "HEALTH", "SECRETS":
		return "RP-004"
	default:
		return "RP-001"
	}
}

func NewService() *Service {
	return &Service{
		nextID:     1,
		frameworks: map[string]Framework{},
		evidence:   []Evidence{},
		dsr:        map[string]DSRRequest{},
		deletions:  map[string]DeletionReport{},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) UpsertFramework(framework Framework) Framework {
	s.mu.Lock()
	defer s.mu.Unlock()

	if framework.ID == "" {
		framework.ID = fmt.Sprintf("framework_%06d", s.nextID)
		s.nextID++
	}
	framework.WorkspaceID = normalizeWorkspaceID(framework.WorkspaceID)
	framework.Key = normalizeFrameworkKey(framework.Key)
	if framework.Status == "" {
		framework.Status = "active"
	}
	if framework.VersionInt == 0 {
		framework.VersionInt = 1
	}
	s.frameworks[framework.ID] = framework
	return framework
}

func (s *Service) ListFrameworks(workspaceID string) []Framework {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Framework, 0, len(s.frameworks))
	for _, framework := range s.frameworks {
		if framework.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, framework)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) AddEvidence(evidence Evidence) Evidence {
	return s.addEvidence(evidence, false)
}

func (s *Service) AddEvidenceStrict(evidence Evidence) (Evidence, error) {
	stored := s.addEvidence(evidence, true)
	if !strings.HasPrefix(stored.SHA256, "sha256:") {
		return Evidence{}, fmt.Errorf("EVIDENCE_HASH_MISSING")
	}
	return stored, nil
}

func (s *Service) addEvidence(evidence Evidence, requireProvidedHash bool) Evidence {
	s.mu.Lock()
	defer s.mu.Unlock()

	evidence.ID = fmt.Sprintf("evidence_%06d", s.nextID)
	s.nextID++
	evidence.WorkspaceID = normalizeWorkspaceID(evidence.WorkspaceID)
	if evidence.EventType == "" {
		evidence.EventType = "BREVIO.compliance.evidence_collected.v1"
	}
	if evidence.CollectedAt == "" {
		evidence.CollectedAt = s.now().Format(time.RFC3339)
	}
	if normalized, ok := normalizeSHA256(evidence.SHA256); ok {
		evidence.SHA256 = normalized
	} else if requireProvidedHash {
		evidence.SHA256 = ""
	} else {
		evidence.SHA256 = buildEvidenceSHA256(evidence)
	}
	s.evidence = append(s.evidence, evidence)
	return evidence
}

func (s *Service) ListEvidence(workspaceID string) []Evidence {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Evidence, 0, len(s.evidence))
	for _, evidence := range s.evidence {
		if evidence.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, evidence)
	}
	return out
}

func (s *Service) CreateDSR(request DSRRequest) DSRRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	request.ID = fmt.Sprintf("dsr_%06d", s.nextID)
	s.nextID++
	request.RequestID = request.ID
	request.WorkspaceID = normalizeWorkspaceID(request.WorkspaceID)
	if request.RequestType == "" {
		request.RequestType = "access"
	}
	if request.Status == "" {
		request.Status = "received"
	}
	request.SubjectUserID = firstNonEmpty(request.SubjectUserID, request.UserID)
	request.CreatedAt = now.Format(time.RFC3339)
	deadline := now.Add(dsrDeadlineDuration(request.RequestType))
	request.DeadlineAt = deadline.Format(time.RFC3339)
	request.DeadlineDate = request.DeadlineAt
	request.SLAAtRisk = false
	s.dsr[request.ID] = request
	return request
}

func (s *Service) GetDSR(id string) (DSRRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	request, ok := s.dsr[id]
	if !ok {
		return DSRRequest{}, false
	}
	return s.withSLAStatus(request, s.now()), true
}

func (s *Service) UpdateDSR(id string, update DSRRequest) (DSRRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.dsr[id]
	if !ok {
		return DSRRequest{}, false
	}
	if update.Status != "" {
		current.Status = update.Status
	}
	if update.DeadlineAt != "" {
		current.DeadlineAt = update.DeadlineAt
		current.DeadlineDate = update.DeadlineAt
	}
	if update.DeadlineDate != "" {
		current.DeadlineDate = update.DeadlineDate
		current.DeadlineAt = update.DeadlineDate
	}
	if update.RequestType != "" {
		current.RequestType = update.RequestType
	}
	current.SLAAtRisk = isDSRAtRisk(current, s.now())
	if isDeletionCompleted(current) {
		s.deletions[current.RequestID] = buildDeletionReport(current, s.nextID, s.now())
		s.nextID++
	}
	s.dsr[id] = current
	return current, true
}

func (s *Service) ListDSR(workspaceID string) []DSRRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]DSRRequest, 0, len(s.dsr))
	for _, request := range s.dsr {
		if request.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, s.withSLAStatus(request, s.now()))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ListDSRAtRisk(workspaceID string) []DSRRequest {
	all := s.ListDSR(workspaceID)
	out := make([]DSRRequest, 0, len(all))
	for _, request := range all {
		if request.SLAAtRisk {
			out = append(out, request)
		}
	}
	return out
}

func (s *Service) GetDeletionReport(requestID string) (DeletionReport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	report, ok := s.deletions[requestID]
	if !ok {
		return DeletionReport{}, false
	}
	return report, true
}

func (s *Service) ListDeletionReports(workspaceID string) []DeletionReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]DeletionReport, 0, len(s.deletions))
	for _, report := range s.deletions {
		if report.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, report)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) withSLAStatus(request DSRRequest, now time.Time) DSRRequest {
	request.SLAAtRisk = isDSRAtRisk(request, now)
	return request
}

func isDSRAtRisk(request DSRRequest, now time.Time) bool {
	status := strings.ToLower(strings.TrimSpace(request.Status))
	if status == "completed" || status == "closed" {
		return false
	}
	deadline := parseDeadline(request)
	if deadline.IsZero() {
		return true
	}
	remaining := deadline.Sub(now)
	return remaining <= 5*24*time.Hour
}

func isDeletionCompleted(request DSRRequest) bool {
	if !strings.EqualFold(strings.TrimSpace(request.RequestType), "deletion") {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(request.Status))
	return status == "completed" || status == "closed"
}

func buildDeletionReport(request DSRRequest, nextID int, now time.Time) DeletionReport {
	return DeletionReport{
		ID:                 fmt.Sprintf("deletion_%06d", nextID),
		RequestID:          request.RequestID,
		WorkspaceID:        request.WorkspaceID,
		SubjectUserID:      request.SubjectUserID,
		DatabasePurged:     true,
		CachePurged:        true,
		ConnectorRevoked:   true,
		MCPOAuthRevoked:    true,
		BackupRotationDays: 30,
		Irreversible:       true,
		CompletedAt:        now.UTC().Format(time.RFC3339),
	}
}

func parseDeadline(request DSRRequest) time.Time {
	for _, candidate := range []string{request.DeadlineAt, request.DeadlineDate} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, candidate); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func normalizeFrameworkKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "soc2", "gdpr", "ccpa":
		return key
	default:
		return "soc2"
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func dsrDeadlineDuration(requestType string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(requestType)) {
	case "portability":
		return 45 * 24 * time.Hour
	case "deletion", "access", "rectification":
		return 30 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeSHA256(value string) (string, bool) {
	candidate := strings.ToLower(strings.TrimSpace(value))
	candidate = strings.TrimPrefix(candidate, "sha256:")
	if len(candidate) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(candidate); err != nil {
		return "", false
	}
	return "sha256:" + candidate, true
}

func buildEvidenceSHA256(evidence Evidence) string {
	joined := strings.Join([]string{
		evidence.WorkspaceID,
		evidence.FrameworkID,
		evidence.EventType,
		evidence.ArtifactURI,
		evidence.CollectedAt,
	}, "::")
	sum := sha256.Sum256([]byte(joined))
	return "sha256:" + hex.EncodeToString(sum[:])
}
