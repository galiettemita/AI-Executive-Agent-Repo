package pii

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PIIField describes a single PII field and its classification.
type PIIField struct {
	FieldPath        string `json:"field_path"`
	DataClass        string `json:"data_class"`        // email | phone | ssn | name | address | financial
	SensitivityLevel string `json:"sensitivity_level"` // low | medium | high | critical
}

// PIIEncryptionPolicy defines workspace-level PII handling rules.
type PIIEncryptionPolicy struct {
	WorkspaceID  string     `json:"workspace_id"`
	Fields       []PIIField `json:"fields"`
	RetentionDays int       `json:"retention_days"`
	AutoRedact   bool       `json:"auto_redact"`
}

// DetectedPII represents a single PII occurrence found by ScanForPII.
type DetectedPII struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	StartIndex int     `json:"start_index"`
	EndIndex   int     `json:"end_index"`
	Confidence float64 `json:"confidence"`
}

// encryptedRecord tracks an encrypted field value and when it was stored.
type encryptedRecord struct {
	Ciphertext  CipherRecord
	EncryptedAt time.Time
}

// PIIEncryptionService provides field-level PII encryption, scanning, and
// retention enforcement on top of the existing AES encryption Service.
type PIIEncryptionService struct {
	mu       sync.RWMutex
	piiSvc   *Service // underlying AES encryption service
	policies map[string]PIIEncryptionPolicy
	records  map[string]map[string]encryptedRecord // workspaceID -> fieldPath -> record
	now      func() time.Time
}

// NewPIIEncryptionService creates a new PIIEncryptionService backed by the
// given pii.Service for actual AES-GCM encryption.
func NewPIIEncryptionService(piiSvc *Service) *PIIEncryptionService {
	return &PIIEncryptionService{
		piiSvc:   piiSvc,
		policies: make(map[string]PIIEncryptionPolicy),
		records:  make(map[string]map[string]encryptedRecord),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// SetNowFunc overrides the clock (useful for testing retention).
func (s *PIIEncryptionService) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn == nil {
		s.now = func() time.Time { return time.Now().UTC() }
		return
	}
	s.now = fn
}

// SetPolicy stores a workspace-level PII encryption policy.
func (s *PIIEncryptionService) SetPolicy(workspaceID string, policy PIIEncryptionPolicy) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if policy.RetentionDays < 0 {
		return fmt.Errorf("retention_days must be >= 0")
	}
	policy.WorkspaceID = workspaceID
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[workspaceID] = policy
	return nil
}

// GetPolicy retrieves the PII encryption policy for a workspace.
func (s *PIIEncryptionService) GetPolicy(workspaceID string) (*PIIEncryptionPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no PII encryption policy for workspace %q", workspaceID)
	}
	return &p, nil
}

// EncryptField encrypts plaintext for a specific field under the workspace's
// policy and stores the record for retention tracking.
func (s *PIIEncryptionService) EncryptField(_ context.Context, workspaceID, fieldPath string, plaintext []byte) ([]byte, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(fieldPath) == "" {
		return nil, fmt.Errorf("field_path is required")
	}

	record, err := s.piiSvc.EncryptField(fieldPath, string(plaintext))
	if err != nil {
		return nil, fmt.Errorf("encrypting field %q: %w", fieldPath, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[workspaceID] == nil {
		s.records[workspaceID] = make(map[string]encryptedRecord)
	}
	s.records[workspaceID][fieldPath] = encryptedRecord{
		Ciphertext:  record,
		EncryptedAt: s.now(),
	}

	return []byte(record.Ciphertext), nil
}

// DecryptField decrypts a field value, returning an error if retention has
// expired.
func (s *PIIEncryptionService) DecryptField(_ context.Context, workspaceID, fieldPath string, ciphertext []byte) ([]byte, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	s.mu.RLock()
	policy, hasPolicy := s.policies[workspaceID]
	rec, hasRecord := s.records[workspaceID][fieldPath]
	now := s.now()
	s.mu.RUnlock()

	// Check retention.
	if hasPolicy && policy.RetentionDays > 0 && hasRecord {
		expiry := rec.EncryptedAt.AddDate(0, 0, policy.RetentionDays)
		if now.After(expiry) {
			return nil, fmt.Errorf("field %q retention expired for workspace %q", fieldPath, workspaceID)
		}
	}

	if hasRecord {
		plaintext, err := s.piiSvc.DecryptField(rec.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypting field %q: %w", fieldPath, err)
		}
		return []byte(plaintext), nil
	}

	// Fallback: try to decrypt with the raw ciphertext provided.
	_ = ciphertext
	return nil, fmt.Errorf("no encrypted record found for field %q in workspace %q", fieldPath, workspaceID)
}

// EnforceRetention purges all encrypted field records that have exceeded
// their workspace's retention period.
func (s *PIIEncryptionService) EnforceRetention(workspaceID string) (int, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return 0, fmt.Errorf("workspace_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	policy, ok := s.policies[workspaceID]
	if !ok {
		return 0, nil
	}
	if policy.RetentionDays <= 0 {
		return 0, nil
	}

	records := s.records[workspaceID]
	if records == nil {
		return 0, nil
	}

	now := s.now()
	purged := 0
	for field, rec := range records {
		expiry := rec.EncryptedAt.AddDate(0, 0, policy.RetentionDays)
		if now.After(expiry) {
			delete(records, field)
			purged++
		}
	}
	return purged, nil
}

// -----------------------------------------------------------------------
// PII Detection & Redaction
// -----------------------------------------------------------------------

var (
	piiEmailRe = regexp.MustCompile(`(?i)\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`)
	piiPhoneRe = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}\b`)
	piiSSNRe   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
)

// ScanForPII scans text for email addresses, phone numbers, and SSNs.
func ScanForPII(text string) []DetectedPII {
	var results []DetectedPII

	for _, loc := range piiSSNRe.FindAllStringIndex(text, -1) {
		results = append(results, DetectedPII{
			Type:       "ssn",
			Value:      text[loc[0]:loc[1]],
			StartIndex: loc[0],
			EndIndex:   loc[1],
			Confidence: 0.95,
		})
	}
	for _, loc := range piiEmailRe.FindAllStringIndex(text, -1) {
		results = append(results, DetectedPII{
			Type:       "email",
			Value:      text[loc[0]:loc[1]],
			StartIndex: loc[0],
			EndIndex:   loc[1],
			Confidence: 0.90,
		})
	}
	for _, loc := range piiPhoneRe.FindAllStringIndex(text, -1) {
		results = append(results, DetectedPII{
			Type:       "phone",
			Value:      text[loc[0]:loc[1]],
			StartIndex: loc[0],
			EndIndex:   loc[1],
			Confidence: 0.80,
		})
	}
	return results
}

// RedactPII replaces detected PII in text with [REDACTED:<type>] markers.
func RedactPII(text string) (string, []DetectedPII) {
	detected := ScanForPII(text)
	if len(detected) == 0 {
		return text, nil
	}

	// De-duplicate overlapping detections: keep the higher-confidence one.
	// Sort by StartIndex ascending first.
	sortDetections(detected)
	deduped := make([]DetectedPII, 0, len(detected))
	for _, d := range detected {
		if len(deduped) > 0 {
			last := &deduped[len(deduped)-1]
			// If this detection overlaps with the previous one, keep the
			// higher-confidence one.
			if d.StartIndex < last.EndIndex {
				if d.Confidence > last.Confidence {
					*last = d
				}
				continue
			}
		}
		deduped = append(deduped, d)
	}

	// Process replacements from right to left so indices stay valid.
	result := text
	for i := len(deduped) - 1; i >= 0; i-- {
		d := deduped[i]
		marker := fmt.Sprintf("[REDACTED:%s]", d.Type)
		result = result[:d.StartIndex] + marker + result[d.EndIndex:]
	}
	return result, detected
}

func sortDetections(d []DetectedPII) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j].StartIndex < d[j-1].StartIndex; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}
