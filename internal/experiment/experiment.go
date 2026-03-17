package experiment

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Experiment is a running A/B test comparing two system prompts.
type Experiment struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	ControlPrompt string    `json:"control_prompt"`
	VariantPrompt string    `json:"variant_prompt"`
	Metric        string    `json:"metric"`
	TargetPValue  float64   `json:"target_p_value"`
	MinSamples    int       `json:"min_samples"`
	CreatedAt     time.Time `json:"created_at"`
}

// ExperimentAssignment maps a workspace to a variant.
type ExperimentAssignment struct {
	WorkspaceID  string `json:"workspace_id"`
	ExperimentID string `json:"experiment_id"`
	Variant      string `json:"variant"`
}

// ExperimentRouter assigns workspaces to experiment variants.
type ExperimentRouter struct {
	pool *pgxpool.Pool
}

func NewExperimentRouter(pool *pgxpool.Pool) *ExperimentRouter {
	return &ExperimentRouter{pool: pool}
}

// GetActiveExperiment returns the first running experiment or nil.
func (r *ExperimentRouter) GetActiveExperiment(ctx context.Context) (*Experiment, error) {
	if r.pool == nil {
		return nil, nil
	}
	var exp Experiment
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, status, control_prompt, variant_prompt,
		       metric, target_p_value, min_samples, created_at
		FROM experiments WHERE status = 'running'
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&exp.ID, &exp.Name, &exp.Description, &exp.Status,
		&exp.ControlPrompt, &exp.VariantPrompt, &exp.Metric,
		&exp.TargetPValue, &exp.MinSamples, &exp.CreatedAt)
	if err != nil {
		return nil, nil
	}
	return &exp, nil
}

// AssignVariant deterministically assigns a workspace to a variant using SHA-256.
func (r *ExperimentRouter) AssignVariant(ctx context.Context, experimentID, workspaceID string) (*ExperimentAssignment, error) {
	if r.pool == nil {
		return &ExperimentAssignment{WorkspaceID: workspaceID, ExperimentID: experimentID, Variant: DeterministicVariant(experimentID, workspaceID)}, nil
	}
	var existing ExperimentAssignment
	err := r.pool.QueryRow(ctx, `
		SELECT workspace_id, experiment_id, variant FROM experiment_assignments
		WHERE workspace_id = $1::uuid AND experiment_id = $2::uuid
	`, workspaceID, experimentID).Scan(&existing.WorkspaceID, &existing.ExperimentID, &existing.Variant)
	if err == nil {
		return &existing, nil
	}

	variant := DeterministicVariant(experimentID, workspaceID)
	assignment := ExperimentAssignment{WorkspaceID: workspaceID, ExperimentID: experimentID, Variant: variant}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO experiment_assignments (workspace_id, experiment_id, variant)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (workspace_id, experiment_id) DO NOTHING
	`, workspaceID, experimentID, variant)
	if err != nil {
		return nil, fmt.Errorf("assign_variant: %w", err)
	}
	return &assignment, nil
}

// DeterministicVariant returns "control" or "variant" based on a hash of the IDs.
func DeterministicVariant(experimentID, workspaceID string) string {
	h := sha256.Sum256([]byte(experimentID + workspaceID))
	bucket := binary.BigEndian.Uint64(h[:8]) % 100
	if bucket >= 50 {
		return "variant"
	}
	return "control"
}

// GetPromptForWorkspace returns the system prompt variant for the given workspace.
func (r *ExperimentRouter) GetPromptForWorkspace(ctx context.Context, workspaceID, defaultPrompt string) (prompt, variant string, err error) {
	exp, err := r.GetActiveExperiment(ctx)
	if err != nil || exp == nil {
		return defaultPrompt, "control", nil
	}
	assignment, err := r.AssignVariant(ctx, exp.ID, workspaceID)
	if err != nil {
		return defaultPrompt, "control", nil
	}
	if assignment.Variant == "variant" {
		return exp.VariantPrompt, "variant", nil
	}
	return exp.ControlPrompt, "control", nil
}

// CreateExperiment creates a new experiment in draft status.
func (r *ExperimentRouter) CreateExperiment(ctx context.Context, exp Experiment) (string, error) {
	id := uuid.New().String()
	if r.pool == nil {
		return id, nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO experiments (id, name, description, status, control_prompt, variant_prompt, metric, target_p_value, min_samples)
		VALUES ($1,$2,$3,'draft',$4,$5,$6,$7,$8)
	`, id, exp.Name, exp.Description, exp.ControlPrompt, exp.VariantPrompt, exp.Metric, exp.TargetPValue, exp.MinSamples)
	return id, err
}

// ListExperiments returns all experiments.
func (r *ExperimentRouter) ListExperiments(ctx context.Context) ([]Experiment, error) {
	if r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, status, control_prompt, variant_prompt, metric, target_p_value, min_samples, created_at
		FROM experiments ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exps []Experiment
	for rows.Next() {
		var e Experiment
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.Status, &e.ControlPrompt, &e.VariantPrompt, &e.Metric, &e.TargetPValue, &e.MinSamples, &e.CreatedAt); err != nil {
			continue
		}
		exps = append(exps, e)
	}
	return exps, nil
}

// StartExperiment transitions an experiment to 'running'.
func (r *ExperimentRouter) StartExperiment(ctx context.Context, experimentID string) error {
	if r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE experiments SET status = 'running' WHERE id = $1 AND status = 'draft'`, experimentID)
	return err
}
