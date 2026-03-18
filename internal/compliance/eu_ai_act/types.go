package eu_ai_act

import (
	"time"

	"github.com/google/uuid"
)

// RiskCategory identifies which EU AI Act article a risk item belongs to.
type RiskCategory string

const (
	RiskCategoryArt9RiskMgmt  RiskCategory = "Art9_RiskMgmt"
	RiskCategoryArt10DataGov  RiskCategory = "Art10_DataGov"
	RiskCategoryArt73Incident RiskCategory = "Art73_Incident"
)

type RiskLikelihood string
type RiskImpact string
type MitigationStatus string

const (
	RiskLikelihoodLow    RiskLikelihood = "low"
	RiskLikelihoodMedium RiskLikelihood = "medium"
	RiskLikelihoodHigh   RiskLikelihood = "high"

	RiskImpactLow    RiskImpact = "low"
	RiskImpactMedium RiskImpact = "medium"
	RiskImpactHigh   RiskImpact = "high"

	MitigationOpen        MitigationStatus = "open"
	MitigationMitigated   MitigationStatus = "mitigated"
	MitigationAccepted    MitigationStatus = "accepted"
	MitigationTransferred MitigationStatus = "transferred"
)

// RiskItem is a single entry in the EU AI Act risk register (Art. 9).
type RiskItem struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	Category         RiskCategory
	Description      string
	Likelihood       RiskLikelihood
	Impact           RiskImpact
	MitigationStatus MitigationStatus
	MitigationNotes  string
	ReviewDate       time.Time
	SourceEvent      string
	SourceRef        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DataGovernanceEntry is a single Art. 10 data governance log record.
type DataGovernanceEntry struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	DatasetName    string
	Provenance     string
	QualityScore   *float64
	BiasIndicators map[string]interface{}
	DPOPairRef     *uuid.UUID
	LoggedAt       time.Time
	CreatedAt      time.Time
}

// IncidentEntry is a single Art. 73 incident log record.
type IncidentEntry struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	IncidentType string
	TriggerMetric string
	Severity     string
	Description  string
	DSRRequestID *uuid.UUID
	ResolvedAt   *time.Time
	ReportedAt   time.Time
	CreatedAt    time.Time
}
