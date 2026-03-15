package kg

import (
	"context"
	"fmt"
	"time"
)

// Repository manages all database operations for the knowledge graph.
type Repository struct {
	db     RepoDB
	logger Logger
}

// RepoDB is the minimal database interface for the KG repository.
type RepoDB interface {
	ExecContext(ctx context.Context, query string, args ...any) error
	QueryContext(ctx context.Context, query string, args ...any) (RepoRows, error)
}

// RepoRows is the minimal rows interface.
type RepoRows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

func NewRepository(db RepoDB, logger Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// UpsertTriple stores or updates a triple with deduplication.
func (r *Repository) UpsertTriple(ctx context.Context, t Triple) error {
	query := `
		INSERT INTO memory_knowledge_graph
			(id, workspace_id, subject, predicate, object,
			 subject_type, object_type, confidence, source_turn_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		ON CONFLICT (workspace_id, lower(trim(subject)), predicate, lower(trim(object)))
		DO UPDATE SET
			confidence     = GREATEST(excluded.confidence, memory_knowledge_graph.confidence),
			source_turn_id = CASE
				WHEN excluded.confidence >= memory_knowledge_graph.confidence
				THEN excluded.source_turn_id
				ELSE memory_knowledge_graph.source_turn_id
			END,
			updated_at = NOW()
		WHERE excluded.confidence >= 0.70
	`
	return r.db.ExecContext(ctx, query,
		t.ID, t.WorkspaceID, t.Subject, t.Predicate, t.Object,
		t.SubjectType, t.ObjectType, t.Confidence, t.SourceTurnID,
		time.Now().UTC(),
	)
}

// UpdateSubjectEmbedding stores the pgvector embedding for a subject entity.
func (r *Repository) UpdateSubjectEmbedding(ctx context.Context, workspaceID, subject string, embedding []float32) error {
	return r.db.ExecContext(ctx, `
		UPDATE memory_knowledge_graph
		SET subject_embedding = $1, subject_embedded_at = NOW()
		WHERE workspace_id = $2
		  AND lower(trim(subject)) = lower(trim($3))
	`, embedding, workspaceID, subject)
}

// UpdateObjectEmbedding stores the pgvector embedding for an object entity.
func (r *Repository) UpdateObjectEmbedding(ctx context.Context, workspaceID, object string, embedding []float32) error {
	return r.db.ExecContext(ctx, `
		UPDATE memory_knowledge_graph
		SET object_embedding = $1, object_embedded_at = NOW()
		WHERE workspace_id = $2
		  AND lower(trim(object)) = lower(trim($3))
	`, embedding, workspaceID, object)
}

// FindSeedEntities returns entity names whose embeddings are most similar to queryVec.
func (r *Repository) FindSeedEntities(
	ctx context.Context,
	workspaceID string,
	queryVec []float32,
	k int,
) ([]string, error) {
	query := `
		SELECT DISTINCT entity FROM (
			SELECT subject AS entity,
				   (1 - (subject_embedding <=> $2::vector)) AS sim
			FROM memory_knowledge_graph
			WHERE workspace_id = $1
			  AND subject_embedding IS NOT NULL
			  AND confidence >= 0.7
			ORDER BY subject_embedding <=> $2::vector
			LIMIT $3

			UNION

			SELECT object AS entity,
				   (1 - (object_embedding <=> $2::vector)) AS sim
			FROM memory_knowledge_graph
			WHERE workspace_id = $1
			  AND object_embedding IS NOT NULL
			  AND confidence >= 0.7
			ORDER BY object_embedding <=> $2::vector
			LIMIT $3
		) combined
		ORDER BY sim DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID, queryVec, k)
	if err != nil {
		return nil, fmt.Errorf("kg find seeds: %w", err)
	}
	defer rows.Close()

	var entities []string
	for rows.Next() {
		var entity string
		if err := rows.Scan(&entity); err != nil {
			continue
		}
		entities = append(entities, entity)
	}
	return entities, rows.Err()
}

// GetTriplesForEntity returns all triples where entityName appears as subject OR object.
func (r *Repository) GetTriplesForEntity(
	ctx context.Context,
	workspaceID, entityName string,
) ([]Triple, error) {
	query := `
		SELECT id, workspace_id, subject, predicate, object,
			   COALESCE(subject_type, '') AS subject_type,
			   COALESCE(object_type, '') AS object_type,
			   confidence,
			   COALESCE(source_turn_id, '') AS source_turn_id,
			   created_at
		FROM memory_knowledge_graph
		WHERE workspace_id = $1
		  AND confidence >= 0.70
		  AND (lower(trim(subject)) = lower(trim($2))
			   OR lower(trim(object)) = lower(trim($2)))
		ORDER BY confidence DESC
		LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID, entityName)
	if err != nil {
		return nil, fmt.Errorf("kg get triples for entity: %w", err)
	}
	defer rows.Close()

	var triples []Triple
	for rows.Next() {
		var t Triple
		if err := rows.Scan(
			&t.ID, &t.WorkspaceID, &t.Subject, &t.Predicate, &t.Object,
			&t.SubjectType, &t.ObjectType, &t.Confidence, &t.SourceTurnID, &t.CreatedAt,
		); err != nil {
			r.logger.Warn("kg: scan error", "error", err)
			continue
		}
		triples = append(triples, t)
	}
	return triples, rows.Err()
}
