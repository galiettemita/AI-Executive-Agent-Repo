package eu_ai_act

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRiskRegister_NilPool_ReturnsError(t *testing.T) {
	_, err := NewRiskRegister(nil)
	assert.Error(t, err)
}

func TestNewDataGovernanceLog_NilPool_ReturnsError(t *testing.T) {
	_, err := NewDataGovernanceLog(nil)
	assert.Error(t, err)
}

func TestNewIncidentLog_NilPool_ReturnsError(t *testing.T) {
	_, err := NewIncidentLog(nil)
	assert.Error(t, err)
}

func TestNewConformityAssessmentGenerator_NilDeps_ReturnsError(t *testing.T) {
	_, err := NewConformityAssessmentGenerator(nil, nil, nil, nil)
	assert.Error(t, err)
}

func TestRiskItem_Defaults(t *testing.T) {
	item := RiskItem{
		WorkspaceID: uuid.New(),
		Category:    RiskCategoryArt9RiskMgmt,
		Description: "test risk",
		Likelihood:  RiskLikelihoodMedium,
		Impact:      RiskImpactMedium,
	}
	assert.Equal(t, RiskCategoryArt9RiskMgmt, item.Category)
	assert.Equal(t, RiskLikelihoodMedium, item.Likelihood)
}

func TestExportAsText_RendersAllSections(t *testing.T) {
	gen := &ConformityAssessmentGenerator{}
	assessment := &ConformityAssessment{
		GeneratedAt:        time.Now().UTC(),
		SystemName:         "TestSystem",
		SystemVersion:      "0.1",
		IntendedPurpose:    "Testing",
		CapabilityBoundary: "None",
		RiskClass:          "Low-Risk",
		RiskItems: []RiskItem{
			{Category: RiskCategoryArt9RiskMgmt, Description: "test risk", Likelihood: RiskLikelihoodLow, Impact: RiskImpactLow, MitigationStatus: MitigationOpen},
		},
		IncidentCount:  2,
		DataGovernance: []DataGovernanceEntry{{DatasetName: "ds1", Provenance: "test"}},
		WorkspaceID:    uuid.New(),
	}

	text, err := gen.ExportAsText(assessment)
	require.NoError(t, err)
	s := string(text)
	assert.Contains(t, s, "EU AI ACT CONFORMITY ASSESSMENT")
	assert.Contains(t, s, "TestSystem")
	assert.Contains(t, s, "RISK REGISTER SUMMARY")
	assert.Contains(t, s, "test risk")
	assert.Contains(t, s, "INCIDENT LOG SUMMARY")
	assert.Contains(t, s, "Total incidents: 2")
	assert.Contains(t, s, "DATA GOVERNANCE")
	assert.Contains(t, s, "ds1")
}

func TestExportAsPDF_ReturnsPDFMagicHeader(t *testing.T) {
	gen := &ConformityAssessmentGenerator{}
	assessment := &ConformityAssessment{
		GeneratedAt:        time.Now().UTC(),
		SystemName:         "TestSystem",
		SystemVersion:      "0.1",
		IntendedPurpose:    "Testing",
		CapabilityBoundary: "None",
		RiskClass:          "Low-Risk",
		RiskItems:          []RiskItem{},
		IncidentCount:      0,
		DataGovernance:     []DataGovernanceEntry{},
		WorkspaceID:        uuid.New(),
	}

	pdfBytes, err := gen.ExportAsPDF(assessment)
	require.NoError(t, err)
	require.True(t, len(pdfBytes) >= 5)
	assert.Equal(t, "%PDF-", string(pdfBytes[:5]))
}
