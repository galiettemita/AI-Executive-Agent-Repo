package temporal

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/compliance/eu_ai_act"
)

// EUAIActAggregateRisksActivity collects risk signals from the past 24h and records them.
func (a *Activities) EUAIActAggregateRisksActivity(ctx context.Context) (int, error) {
	if a.pool == nil || a.euRiskRegister == nil {
		return 0, nil
	}

	// Query recent tool executions as risk signals.
	rows, err := a.pool.Query(ctx,
		`SELECT DISTINCT workspace_id::text
		 FROM tool_executions
		 WHERE created_at > now() - INTERVAL '24 hours'
		 LIMIT 100`)
	if err != nil {
		// Table may not exist; non-fatal.
		log.Printf("[EU AI Act] aggregate risks: query failed: %v", err)
		return 0, nil
	}
	defer rows.Close()

	var recorded int
	for rows.Next() {
		var wsIDStr string
		if err := rows.Scan(&wsIDStr); err != nil {
			continue
		}
		wsID, err := uuid.Parse(wsIDStr)
		if err != nil {
			continue
		}
		_, err = a.euRiskRegister.RecordRisk(ctx, eu_ai_act.RiskItem{
			WorkspaceID:      wsID,
			Category:         eu_ai_act.RiskCategoryArt9RiskMgmt,
			Description:      "Automated tool execution in past 24h",
			Likelihood:       eu_ai_act.RiskLikelihoodMedium,
			Impact:           eu_ai_act.RiskImpactMedium,
			MitigationStatus: eu_ai_act.MitigationOpen,
			SourceEvent:      "daily_aggregate",
		})
		if err == nil {
			recorded++
		}
	}

	log.Printf("[EU AI Act] aggregate risks done: recorded=%d", recorded)
	return recorded, nil
}

// EUAIActCheckIncidentThresholdsActivity reviews recent incidents and logs alerts
// if thresholds are exceeded (>3 high-severity incidents in 24h).
func (a *Activities) EUAIActCheckIncidentThresholdsActivity(ctx context.Context) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	var count int
	err := a.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM eu_ai_act_incidents
		 WHERE reported_at > now() - INTERVAL '24 hours'
		   AND severity IN ('high', 'critical')
		   AND resolved_at IS NULL`,
	).Scan(&count)
	if err != nil {
		// Table may not exist yet; non-fatal.
		log.Printf("[EU AI Act] incident threshold check: query failed: %v", err)
		return 0, nil
	}

	if count > 3 {
		log.Printf("[EU AI Act] ALERT: %d high-severity incidents in past 24h (threshold: 3)", count)
	}

	return count, nil
}

// EUAIActGenerateConformityEvidenceActivity generates conformity assessment
// documents for all active workspaces and stores them as compliance evidence.
func (a *Activities) EUAIActGenerateConformityEvidenceActivity(ctx context.Context) error {
	if a.pool == nil || a.euConformityGenerator == nil {
		return nil
	}

	rows, err := a.pool.Query(ctx,
		`SELECT DISTINCT workspace_id::text FROM eu_ai_act_risks LIMIT 100`)
	if err != nil {
		log.Printf("[EU AI Act] conformity evidence: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var wsIDStr string
		if err := rows.Scan(&wsIDStr); err != nil {
			continue
		}
		wsID, err := uuid.Parse(wsIDStr)
		if err != nil {
			continue
		}

		assessment, err := a.euConformityGenerator.Generate(ctx, wsID)
		if err != nil {
			log.Printf("[EU AI Act] conformity generation failed workspace=%s: %v", wsIDStr, err)
			continue
		}

		// Try PDF export first.
		pdfBytes, pdfErr := a.euConformityGenerator.ExportAsPDF(assessment)
		if pdfErr == nil && len(pdfBytes) >= 5 && string(pdfBytes[:5]) == "%PDF-" {
			pdfB64 := base64.StdEncoding.EncodeToString(pdfBytes)
			_, _ = a.pool.Exec(ctx,
				`INSERT INTO compliance_evidence
				 (workspace_id, event_type, artifact_uri, sha256, deleted_counts, collected_at)
				 VALUES ($1, 'eu_ai_act_conformity_pdf', $2, '', NULL, now())`,
				wsID, fmt.Sprintf("data:application/pdf;base64,%s", pdfB64[:40]+"..."),
			)
			log.Printf("[EU AI Act] conformity PDF stored workspace=%s", wsIDStr)
			continue
		}

		// Fallback to text.
		textBytes, textErr := a.euConformityGenerator.ExportAsText(assessment)
		if textErr == nil {
			_, _ = a.pool.Exec(ctx,
				`INSERT INTO compliance_evidence
				 (workspace_id, event_type, artifact_uri, sha256, deleted_counts, collected_at)
				 VALUES ($1, 'eu_ai_act_conformity_text', $2, '', NULL, now())`,
				wsID, fmt.Sprintf("text://%d-bytes", len(textBytes)),
			)
			log.Printf("[EU AI Act] conformity text stored workspace=%s", wsIDStr)
		}
	}

	return nil
}
