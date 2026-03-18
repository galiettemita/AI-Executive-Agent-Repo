package consent

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// toolConsentRequirements maps data categories to the consent purpose required.
var toolConsentRequirements = map[string]ConsentPurpose{
	"financial": PurposeExecutiveAssistance,
	"health":    PurposeExecutiveAssistance,
	"analytics": PurposeAnalytics,
	"marketing": PurposeMarketing,
}

// PurposeLimitationMiddleware enforces that tool execution only proceeds
// when the user has active consent for the required purpose.
type PurposeLimitationMiddleware struct {
	registry *ConsentRegistry
	logger   *slog.Logger
}

// NewPurposeLimitationMiddleware creates a new middleware instance.
func NewPurposeLimitationMiddleware(registry *ConsentRegistry, logger *slog.Logger) *PurposeLimitationMiddleware {
	return &PurposeLimitationMiddleware{
		registry: registry,
		logger:   logger,
	}
}

// CheckToolConsent verifies the user has consented to the required purpose for
// the tool's data category. Records the data access in the audit log if allowed.
func (m *PurposeLimitationMiddleware) CheckToolConsent(
	ctx context.Context,
	workspaceID, userID uuid.UUID,
	toolKey string,
	dataCategory string,
) error {
	requiredPurpose := determinePurpose(dataCategory)

	hasConsent, err := m.registry.HasActiveConsent(ctx, workspaceID, userID, requiredPurpose)
	if err != nil {
		return err
	}

	if !hasConsent {
		m.logger.Warn("consent_check_blocked",
			"workspace_id", workspaceID,
			"user_id", userID,
			"tool_key", toolKey,
			"data_category", dataCategory,
			"required_purpose", requiredPurpose,
		)
		return ErrConsentRequired{
			Purpose:      requiredPurpose,
			DataCategory: dataCategory,
		}
	}

	// Log high-sensitivity tool access.
	if dataCategory == "financial" || dataCategory == "health" {
		m.logger.Warn("sensitive_tool_access_granted",
			"workspace_id", workspaceID,
			"user_id", userID,
			"tool_key", toolKey,
			"data_category", dataCategory,
		)
	}

	// Record data access in audit trail.
	consentID, findErr := m.registry.FindActiveConsentID(ctx, workspaceID, userID, requiredPurpose)
	if findErr == nil && consentID != uuid.Nil {
		if auditErr := m.registry.RecordDataAccess(ctx, consentID, toolKey, dataCategory); auditErr != nil {
			m.logger.Error("record_data_access_error", "error", auditErr)
		}
	}

	return nil
}

func determinePurpose(dataCategory string) ConsentPurpose {
	if purpose, ok := toolConsentRequirements[dataCategory]; ok {
		return purpose
	}
	return PurposeExecutiveAssistance
}
