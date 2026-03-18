package soc2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SOC2ReportGenerator produces PDF compliance reports from evidence records.
type SOC2ReportGenerator struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewSOC2ReportGenerator creates a report generator.
func NewSOC2ReportGenerator(db *pgxpool.Pool, logger *slog.Logger) *SOC2ReportGenerator {
	return &SOC2ReportGenerator{db: db, logger: logger}
}

// GenerateReport produces a PDF compliance report for the given date range.
func (g *SOC2ReportGenerator) GenerateReport(ctx context.Context, startDate, endDate time.Time) ([]byte, error) {
	// Load evidence records.
	evidences, err := g.loadEvidence(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("load evidence: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)

	// Cover page.
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 24)
	pdf.Ln(40)
	pdf.CellFormat(0, 15, "Brevio AI Agent", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 18)
	pdf.CellFormat(0, 12, "SOC2 Type II Compliance Report", "", 1, "C", false, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Period: %s to %s",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Generated: %s", time.Now().Format(time.RFC3339)), "", 1, "C", false, 0, "")

	// Executive summary.
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Executive Summary", "", 1, "", false, 0, "")
	pdf.Ln(5)

	totalControls := len(evidences)
	passCount := 0
	soc2Count := 0
	isoCount := 0
	for _, ev := range evidences {
		if ev.Pass {
			passCount++
		}
		if ev.Framework == FrameworkSOC2 {
			soc2Count++
		} else {
			isoCount++
		}
	}

	passRate := 0.0
	if totalControls > 0 {
		passRate = float64(passCount) / float64(totalControls) * 100
	}

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, fmt.Sprintf("Total Controls Evaluated: %d", totalControls), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Controls Passed: %d", passCount), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Controls Failed: %d", totalControls-passCount), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("Pass Rate: %.1f%%", passRate), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("SOC2 Controls: %d", soc2Count), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("ISO 27001 Controls: %d", isoCount), "", 1, "", false, 0, "")

	// Per-control sections.
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Control Evidence Details", "", 1, "", false, 0, "")
	pdf.Ln(5)

	for _, ev := range evidences {
		pdf.SetFont("Helvetica", "B", 12)
		status := "PASS"
		if !ev.Pass {
			status = "FAIL"
		}
		pdf.CellFormat(0, 8, fmt.Sprintf("[%s] %s (%s) - %s",
			status, ev.ControlID, ev.Framework, ev.EvidenceType), "", 1, "", false, 0, "")

		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 6, fmt.Sprintf("Collected: %s", ev.CollectedAt.Format(time.RFC3339)), "", 1, "", false, 0, "")

		if ev.Details != nil {
			detailsJSON, _ := json.MarshalIndent(ev.Details, "", "  ")
			detailStr := truncate(string(detailsJSON), 500)
			pdf.MultiCell(0, 5, detailStr, "", "", false)
		}
		pdf.Ln(3)

		// Page break if we're running low on space.
		if pdf.GetY() > 250 {
			pdf.AddPage()
		}
	}

	// Appendix: ISO 27001 Annex A mapping.
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Appendix: ISO 27001 Annex A Mapping", "", 1, "", false, 0, "")
	pdf.Ln(5)

	annexMapping := []struct{ ID, Domain string }{
		{"A.9.1", "Access Control Policy"},
		{"A.9.2", "User Access Management"},
		{"A.9.3", "User Responsibilities"},
		{"A.9.4", "System and Application Access"},
		{"A.10.1", "Cryptography Policy"},
		{"A.12.1", "Operational Procedures"},
		{"A.12.3", "Information Backup"},
		{"A.12.4", "Logging"},
		{"A.12.6", "Vulnerability Management"},
		{"A.13.1", "Network Security"},
		{"A.14.2", "Security in Development"},
		{"A.16.1", "Security Incident Management"},
		{"A.17.1", "Business Continuity"},
		{"A.18.1", "Compliance with Legal"},
		{"A.18.2", "Information Security Reviews"},
	}

	pdf.SetFont("Helvetica", "", 10)
	for _, m := range annexMapping {
		pdf.CellFormat(30, 7, m.ID, "1", 0, "", false, 0, "")
		pdf.CellFormat(0, 7, m.Domain, "1", 1, "", false, 0, "")
	}

	// Write to buffer.
	var buf pdfBuffer
	if err := pdf.OutputAndClose(&buf); err != nil {
		return nil, fmt.Errorf("generate PDF: %w", err)
	}

	return buf.Bytes(), nil
}

type evidenceRow struct {
	ControlID    string
	Framework    string
	EvidenceType string
	CollectedAt  time.Time
	Pass         bool
	Details      map[string]interface{}
}

func (g *SOC2ReportGenerator) loadEvidence(ctx context.Context, startDate, endDate time.Time) ([]evidenceRow, error) {
	if g.db == nil {
		return nil, nil
	}

	rows, err := g.db.Query(ctx,
		`SELECT control_id, framework, evidence_type, collected_at, pass, details
		 FROM compliance_evidence
		 WHERE collected_at >= $1 AND collected_at <= $2
		 ORDER BY framework, control_id, collected_at DESC`,
		startDate, endDate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Deduplicate: keep only the latest evidence per (control_id, framework).
	seen := map[string]bool{}
	var results []evidenceRow
	for rows.Next() {
		var ev evidenceRow
		var detailsJSON []byte
		if err := rows.Scan(&ev.ControlID, &ev.Framework, &ev.EvidenceType,
			&ev.CollectedAt, &ev.Pass, &detailsJSON); err != nil {
			continue
		}
		key := ev.Framework + ":" + ev.ControlID
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &ev.Details)
		}
		results = append(results, ev)
	}

	return results, rows.Err()
}

// pdfBuffer implements io.WriteCloser for fpdf output.
type pdfBuffer struct {
	data []byte
}

func (b *pdfBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *pdfBuffer) Close() error { return nil }

func (b *pdfBuffer) Bytes() []byte { return b.data }
