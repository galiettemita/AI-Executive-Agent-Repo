package cai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConstitutionalPrincipleDiscovery analyzes failure patterns to propose
// new Constitutional AI principles.
type ConstitutionalPrincipleDiscovery struct {
	db        *pgxpool.Pool
	llmClient LLMClient
	logger    *slog.Logger
}

// NewConstitutionalPrincipleDiscovery creates a discovery service.
func NewConstitutionalPrincipleDiscovery(db *pgxpool.Pool, llm LLMClient, logger *slog.Logger) *ConstitutionalPrincipleDiscovery {
	return &ConstitutionalPrincipleDiscovery{db: db, llmClient: llm, logger: logger}
}

// DiscoverPrinciples analyzes 30-day failures and proposes new principles.
func (d *ConstitutionalPrincipleDiscovery) DiscoverPrinciples(ctx context.Context) ([]ProposedPrinciple, error) {
	violations, err := d.loadRecentViolations(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("load violations: %w", err)
	}

	existingPrinciples, err := d.loadActivePrinciples(ctx)
	if err != nil {
		return nil, fmt.Errorf("load principles: %w", err)
	}

	if len(violations) == 0 {
		d.logger.Info("principle_discovery_no_violations")
		return nil, nil
	}

	// Build LLM prompt.
	var violationSummary strings.Builder
	for i, v := range violations {
		fmt.Fprintf(&violationSummary, "%d. [%s] %s", i+1, v.PrincipleID, v.ViolationType)
		if v.UserCorrection != "" {
			fmt.Fprintf(&violationSummary, " (correction: %s)", v.UserCorrection)
		}
		violationSummary.WriteString("\n")
	}

	var principleTexts strings.Builder
	for _, p := range existingPrinciples {
		fmt.Fprintf(&principleTexts, "- %s: %s\n", p.PrincipleID, p.Text)
	}

	prompt := fmt.Sprintf(
		`Analyze these AI assistant failures. Identify patterns that suggest missing Constitutional AI principles.

Violations:
%s

Existing principles:
%s

Identify 1-3 distinct failure patterns NOT covered by existing principles.
For each new principle candidate, return JSON:
{"principles": [{"description": "principle text", "failure_examples": ["example 1"], "coverage_rate": 0.15, "conflict_with_existing": []}]}
Return ONLY the JSON, no preamble.`,
		violationSummary.String(), principleTexts.String(),
	)

	resp, err := d.llmClient.Complete(ctx, "You are an AI safety researcher.", prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM discovery: %w", err)
	}

	// Parse response.
	var result struct {
		Principles []struct {
			Description          string   `json:"description"`
			FailureExamples      []string `json:"failure_examples"`
			CoverageRate         float64  `json:"coverage_rate"`
			ConflictWithExisting []string `json:"conflict_with_existing"`
		} `json:"principles"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		d.logger.Warn("principle_discovery_parse_error", "response", resp, "error", err)
		return nil, nil
	}

	var proposals []ProposedPrinciple
	for _, p := range result.Principles {
		if p.CoverageRate < 0.05 {
			continue // below 5% coverage threshold
		}

		if d.db != nil {
			examplesJSON, _ := json.Marshal(p.FailureExamples)
			conflictJSON, _ := json.Marshal(p.ConflictWithExisting)

			_, dbErr := d.db.Exec(ctx,
				`INSERT INTO proposed_principles (description, failure_examples, coverage_rate, conflict_with_existing, status)
				 VALUES ($1, $2, $3, $4, 'draft')`,
				p.Description, examplesJSON, p.CoverageRate, conflictJSON,
			)
			if dbErr != nil {
				d.logger.Error("insert_proposed_principle_error", "error", dbErr)
				continue
			}
		}

		proposals = append(proposals, ProposedPrinciple{
			Description:          p.Description,
			FailureExamples:      p.FailureExamples,
			CoverageRate:         p.CoverageRate,
			ConflictWithExisting: p.ConflictWithExisting,
			Status:               "draft",
		})
	}

	d.logger.Info("principle_discovery_complete", "proposals", len(proposals))
	return proposals, nil
}

// GetProposedPrinciples returns all draft/admin_review proposals.
func (d *ConstitutionalPrincipleDiscovery) GetProposedPrinciples(ctx context.Context) ([]ProposedPrinciple, error) {
	if d.db == nil {
		return nil, nil
	}

	rows, err := d.db.Query(ctx,
		`SELECT id, description, failure_examples, coverage_rate, status, proposed_at
		 FROM proposed_principles WHERE status IN ('draft', 'admin_review')
		 ORDER BY proposed_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proposals []ProposedPrinciple
	for rows.Next() {
		var p ProposedPrinciple
		var examplesJSON []byte
		if err := rows.Scan(&p.ID, &p.Description, &examplesJSON, &p.CoverageRate, &p.Status, &p.ProposedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(examplesJSON, &p.FailureExamples)
		proposals = append(proposals, p)
	}
	return proposals, nil
}

func (d *ConstitutionalPrincipleDiscovery) loadRecentViolations(ctx context.Context, limit int) ([]Violation, error) {
	if d.db == nil {
		return nil, nil
	}

	rows, err := d.db.Query(ctx,
		`SELECT principle_id, violation_type, COALESCE(user_correction, '')
		 FROM constitutional_violations
		 WHERE violated_at > NOW() - INTERVAL '30 days'
		 ORDER BY violated_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var violations []Violation
	for rows.Next() {
		var v Violation
		if err := rows.Scan(&v.PrincipleID, &v.ViolationType, &v.UserCorrection); err != nil {
			continue
		}
		violations = append(violations, v)
	}
	return violations, nil
}

func (d *ConstitutionalPrincipleDiscovery) loadActivePrinciples(ctx context.Context) ([]ConstitutionalPrinciple, error) {
	if d.db == nil {
		return nil, nil
	}

	rows, err := d.db.Query(ctx,
		`SELECT principle_id, text FROM constitutional_principles WHERE status = 'active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var principles []ConstitutionalPrinciple
	for rows.Next() {
		var p ConstitutionalPrinciple
		if err := rows.Scan(&p.PrincipleID, &p.Text); err != nil {
			continue
		}
		principles = append(principles, p)
	}
	return principles, nil
}

// RecordViolation inserts a violation record into the database.
func RecordViolation(ctx context.Context, db *pgxpool.Pool, principleID, violationType, userCorrection string, workspaceID, requestID *string) error {
	if db == nil {
		return nil
	}

	_, err := db.Exec(ctx,
		`INSERT INTO constitutional_violations (principle_id, violation_type, user_correction, workspace_id, request_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		principleID, violationType, userCorrection, workspaceID, requestID,
	)
	return err
}
