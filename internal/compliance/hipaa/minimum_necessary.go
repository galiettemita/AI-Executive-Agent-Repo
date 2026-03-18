package hipaa

import (
	"log/slog"
)

// MinimumNecessaryFilter enforces HIPAA's minimum necessary standard
// by stripping PHI fields from responses that were not explicitly requested.
type MinimumNecessaryFilter struct {
	logger *slog.Logger
}

// NewMinimumNecessaryFilter creates a filter instance.
func NewMinimumNecessaryFilter(logger *slog.Logger) *MinimumNecessaryFilter {
	return &MinimumNecessaryFilter{logger: logger}
}

// phiFieldSet is the set of field names considered PHI.
var phiFieldSet = map[string]bool{
	"diagnosis":              true,
	"medication":             true,
	"vital_signs":            true,
	"icd_code":               true,
	"medical_record_number":  true,
	"insurance_id":           true,
	"date_of_birth":          true,
	"ssn":                    true,
}

// FilterResponse removes PHI fields from the response that were not explicitly
// requested, enforcing the HIPAA minimum necessary standard.
func (f *MinimumNecessaryFilter) FilterResponse(response map[string]interface{}, requestedFields []string) map[string]interface{} {
	requestedSet := make(map[string]bool, len(requestedFields))
	for _, field := range requestedFields {
		requestedSet[field] = true
	}

	filtered := make(map[string]interface{}, len(response))
	for key, value := range response {
		if phiFieldSet[key] && !requestedSet[key] {
			f.logger.Info("phi_field_redacted_minimum_necessary", "field", key)
			continue
		}
		filtered[key] = value
	}

	return filtered
}
