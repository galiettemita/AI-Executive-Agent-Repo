package eu_ai_act

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConformityAssessment represents the EU AI Act conformity document.
type ConformityAssessment struct {
	GeneratedAt        time.Time
	SystemName         string
	SystemVersion      string
	IntendedPurpose    string
	CapabilityBoundary string
	RiskClass          string
	RiskItems          []RiskItem
	IncidentCount      int
	DataGovernance     []DataGovernanceEntry
	WorkspaceID        uuid.UUID
}

// ConformityAssessmentGenerator generates conformity assessment documents.
type ConformityAssessmentGenerator struct {
	pool         *pgxpool.Pool
	riskRegister *RiskRegister
	incidentLog  *IncidentLog
	dataGovLog   *DataGovernanceLog
}

// NewConformityAssessmentGenerator creates a generator. Returns error if any dep is nil.
func NewConformityAssessmentGenerator(pool *pgxpool.Pool, rr *RiskRegister,
	il *IncidentLog, dg *DataGovernanceLog) (*ConformityAssessmentGenerator, error) {
	if pool == nil || rr == nil || il == nil || dg == nil {
		return nil, fmt.Errorf("eu_ai_act.NewConformityAssessmentGenerator: all dependencies required")
	}
	return &ConformityAssessmentGenerator{pool: pool, riskRegister: rr, incidentLog: il, dataGovLog: dg}, nil
}

// Generate produces a ConformityAssessment for the given workspace.
func (g *ConformityAssessmentGenerator) Generate(ctx context.Context, workspaceID uuid.UUID) (*ConformityAssessment, error) {
	risks, err := g.riskRegister.ListRisks(ctx, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("ConformityAssessmentGenerator.Generate: risks: %w", err)
	}
	incidents, err := g.incidentLog.ListIncidents(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("ConformityAssessmentGenerator.Generate: incidents: %w", err)
	}
	dgEntries, err := g.dataGovLog.ListEntries(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("ConformityAssessmentGenerator.Generate: data gov: %w", err)
	}

	return &ConformityAssessment{
		GeneratedAt:        time.Now().UTC(),
		SystemName:         "Brevio AI Executive Assistant",
		SystemVersion:      "1.0",
		IntendedPurpose:    "AI-powered executive assistant operating over WhatsApp/iMessage, automating calendar, email, and productivity workflows.",
		CapabilityBoundary: "Operates within user-granted tool permissions. Cannot initiate financial transactions or legal commitments without explicit human approval.",
		RiskClass:          "High-Risk AI system — automated decision-making in professional/employment context (Annex III, Category 4).",
		RiskItems:          risks,
		IncidentCount:      len(incidents),
		DataGovernance:     dgEntries,
		WorkspaceID:        workspaceID,
	}, nil
}

// ExportAsText renders the conformity assessment as a plain text document.
func (g *ConformityAssessmentGenerator) ExportAsText(assessment *ConformityAssessment) ([]byte, error) {
	const tmpl = `EU AI ACT CONFORMITY ASSESSMENT
================================
Generated: {{ .GeneratedAt.Format "2006-01-02T15:04:05Z" }}
System: {{ .SystemName }} v{{ .SystemVersion }}

INTENDED PURPOSE
{{ .IntendedPurpose }}

CAPABILITY BOUNDARY
{{ .CapabilityBoundary }}

RISK CLASSIFICATION
{{ .RiskClass }}

RISK REGISTER SUMMARY (Art. 9)
Total risks: {{ len .RiskItems }}
{{ range .RiskItems }}- [{{ .Category }}] {{ .Description }} | likelihood={{ .Likelihood }} impact={{ .Impact }} status={{ .MitigationStatus }}
{{ end }}
INCIDENT LOG SUMMARY (Art. 73)
Total incidents: {{ .IncidentCount }}

DATA GOVERNANCE (Art. 10)
Total datasets logged: {{ len .DataGovernance }}
{{ range .DataGovernance }}- {{ .DatasetName }}: {{ .Provenance }}
{{ end }}`

	t, err := template.New("conformity").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("ExportAsText: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, assessment); err != nil {
		return nil, fmt.Errorf("ExportAsText: execute: %w", err)
	}
	return buf.Bytes(), nil
}

// ExportAsPDF renders the conformity assessment as a PDF document.
// The returned bytes start with the "%PDF-" magic header.
func (g *ConformityAssessmentGenerator) ExportAsPDF(assessment *ConformityAssessment) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "EU AI Act Conformity Assessment", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Metadata
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6,
		fmt.Sprintf("Generated: %s   System: %s v%s",
			assessment.GeneratedAt.Format("2006-01-02T15:04:05Z"),
			assessment.SystemName,
			assessment.SystemVersion,
		), "", 1, "L", false, 0, "")
	pdf.Ln(6)

	writeSection := func(title, body string) {
		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(0, 8, title, "B", 1, "L", false, 0, "")
		pdf.Ln(2)
		pdf.SetFont("Arial", "", 10)
		pdf.MultiCell(0, 6, body, "", "L", false)
		pdf.Ln(4)
	}

	writeSection("Intended Purpose", assessment.IntendedPurpose)
	writeSection("Capability Boundary", assessment.CapabilityBoundary)
	writeSection("Risk Classification", assessment.RiskClass)

	// Risk Register
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Risk Register Summary - Art. 9 (%d risks)", len(assessment.RiskItems)),
		"B", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 9)
	for _, r := range assessment.RiskItems {
		line := fmt.Sprintf("[%s] %s | likelihood=%s impact=%s status=%s",
			r.Category, r.Description, r.Likelihood, r.Impact, r.MitigationStatus)
		pdf.MultiCell(0, 5, line, "", "L", false)
	}
	pdf.Ln(4)

	writeSection("Incident Log Summary - Art. 73",
		fmt.Sprintf("Total serious incidents reported: %d", assessment.IncidentCount))

	// Data Governance
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Data Governance Log - Art. 10 (%d datasets)", len(assessment.DataGovernance)),
		"B", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 9)
	for _, dg := range assessment.DataGovernance {
		pdf.MultiCell(0, 5, fmt.Sprintf("%s: %s", dg.DatasetName, dg.Provenance), "", "L", false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("ExportAsPDF: %w", err)
	}

	pdfBytes := buf.Bytes()
	if len(pdfBytes) < 5 || string(pdfBytes[:5]) != "%PDF-" {
		return nil, fmt.Errorf("ExportAsPDF: output is not a valid PDF")
	}
	return pdfBytes, nil
}
