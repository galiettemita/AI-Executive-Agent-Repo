package hipaa

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Error types for HIPAA policy enforcement.
var (
	ErrHIPAAAuthRequired       = fmt.Errorf("hipaa: HIPAA_AUTH consent required")
	ErrWorkspaceNotHIPAACovered = fmt.Errorf("hipaa: workspace is not a HIPAA covered entity")
	ErrBAARequired             = fmt.Errorf("hipaa: active Business Associate Agreement required")
	ErrEncryptionRequired      = fmt.Errorf("hipaa: encryption_at_rest and encryption_in_transit required")
)

// PHIAccessRequest describes a request to access PHI data.
type PHIAccessRequest struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	UserID      uuid.UUID `json:"user_id"`
	PHICategory string    `json:"phi_category"`
	Purpose     string    `json:"purpose"`
	ToolKey     string    `json:"tool_key"`
}

// HIPAABreachEvent describes a detected PHI breach.
type HIPAABreachEvent struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	UserID      uuid.UUID `json:"user_id"`
	PHICategory string    `json:"phi_category"`
	BreachType  string    `json:"breach_type"` // "error_log_exposure" | "ai_response_leak" | "unencrypted_storage"
	DetectedAt  time.Time `json:"detected_at"`
	Details     string    `json:"details"`
}

// PHI field names used for minimum necessary filtering.
var PHIFieldNames = []string{
	"diagnosis", "medication", "vital_signs", "icd_code",
	"medical_record_number", "insurance_id", "date_of_birth", "ssn",
}
