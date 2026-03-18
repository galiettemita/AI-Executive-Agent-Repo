package consent

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ConsentPurpose identifies the processing purpose for which consent is granted.
type ConsentPurpose string

const (
	PurposeExecutiveAssistance ConsentPurpose = "executive_assistance"
	PurposeAnalytics           ConsentPurpose = "analytics"
	PurposeFineTuning          ConsentPurpose = "fine_tuning"
	PurposeMarketing           ConsentPurpose = "marketing"
)

// ValidPurposes is the set of valid consent purposes.
var ValidPurposes = map[ConsentPurpose]bool{
	PurposeExecutiveAssistance: true,
	PurposeAnalytics:           true,
	PurposeFineTuning:          true,
	PurposeMarketing:           true,
}

// LawfulBasis identifies the GDPR lawful basis for processing.
type LawfulBasis string

const (
	LawfulBasisConsent    LawfulBasis = "consent"
	LawfulBasisContract   LawfulBasis = "contract"
	LawfulBasisLegitimate LawfulBasis = "legitimate_interest"
)

// ValidBases is the set of valid lawful bases.
var ValidBases = map[LawfulBasis]bool{
	LawfulBasisConsent:    true,
	LawfulBasisContract:   true,
	LawfulBasisLegitimate: true,
}

// ConsentRecord is a single consent grant tracked in the registry.
type ConsentRecord struct {
	ID          uuid.UUID      `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	UserID      uuid.UUID      `json:"user_id"`
	Purpose     ConsentPurpose `json:"purpose"`
	LawfulBasis LawfulBasis    `json:"lawful_basis"`
	GrantedAt   time.Time      `json:"granted_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	RevokedAt   *time.Time     `json:"revoked_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// IsActive returns true if the consent has not been revoked and has not expired.
func (c ConsentRecord) IsActive() bool {
	if c.RevokedAt != nil {
		return false
	}
	if c.ExpiresAt != nil && c.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// GrantConsentRequest is the input for granting consent.
type GrantConsentRequest struct {
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	UserID      uuid.UUID      `json:"user_id"`
	Purpose     ConsentPurpose `json:"purpose"`
	LawfulBasis LawfulBasis    `json:"lawful_basis"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
}

// RevocationInput is the Temporal workflow input for consent revocation erasure.
type RevocationInput struct {
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	UserID      uuid.UUID      `json:"user_id"`
	Purpose     ConsentPurpose `json:"purpose"`
	RevokedAt   time.Time      `json:"revoked_at"`
}

// ErrConsentNotFound is returned when no active consent record exists.
var ErrConsentNotFound = fmt.Errorf("consent: no active consent record found")

// ErrConsentRequired indicates a tool call was blocked due to missing consent.
type ErrConsentRequired struct {
	Purpose      ConsentPurpose
	DataCategory string
}

func (e ErrConsentRequired) Error() string {
	return fmt.Sprintf("consent required for purpose=%s data_category=%s", e.Purpose, e.DataCategory)
}
