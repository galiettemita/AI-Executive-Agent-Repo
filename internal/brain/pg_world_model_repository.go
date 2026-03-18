package brain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const worldModelCacheTTL = 5 * time.Minute

type wmCacheKey struct {
	workspaceID uuid.UUID
	subject     string
}

type wmCacheEntry struct {
	facts     []WorldFact
	expiresAt time.Time
}

// PgWorldModelRepository implements WorldModelRepository backed by PostgreSQL
// with a 5-minute write-through in-memory read cache.
type PgWorldModelRepository struct {
	pool  *pgxpool.Pool
	mu    sync.RWMutex
	cache map[wmCacheKey]wmCacheEntry
}

// NewPgWorldModelRepository creates a new repository. Returns an error if pool is nil.
func NewPgWorldModelRepository(pool *pgxpool.Pool) (*PgWorldModelRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("brain.NewPgWorldModelRepository: pool must not be nil")
	}
	return &PgWorldModelRepository{
		pool:  pool,
		cache: make(map[wmCacheKey]wmCacheEntry),
	}, nil
}

// AddFact upserts a world model fact. On conflict (workspace_id, subject, predicate),
// the newer value wins.
func (r *PgWorldModelRepository) AddFact(ctx context.Context, workspaceID uuid.UUID,
	subject, predicate, value, source string, confidence float64, expiresAt time.Time) (WorldFact, error) {

	fact := WorldFact{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID.String(),
		Subject:     subject,
		Predicate:   predicate,
		Value:       value,
		Source:      source,
		Confidence:  confidence,
		LearnedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().UTC(),
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO world_model_facts (id, workspace_id, subject, predicate, value, source, confidence, learned_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (workspace_id, subject, predicate) DO UPDATE SET
			value = EXCLUDED.value,
			source = EXCLUDED.source,
			confidence = EXCLUDED.confidence,
			learned_at = EXCLUDED.learned_at,
			expires_at = EXCLUDED.expires_at`,
		fact.ID, workspaceID, fact.Subject, fact.Predicate, fact.Value,
		fact.Source, fact.Confidence, fact.LearnedAt, fact.ExpiresAt, fact.CreatedAt,
	)
	if err != nil {
		return WorldFact{}, fmt.Errorf("world_model add_fact: %w", err)
	}

	r.invalidateWorkspace(workspaceID)
	return fact, nil
}

// GetFacts returns all non-expired facts for a workspace filtered by subject.
func (r *PgWorldModelRepository) GetFacts(ctx context.Context, workspaceID uuid.UUID, subject string) ([]WorldFact, error) {
	key := wmCacheKey{workspaceID: workspaceID, subject: subject}
	if cached, ok := r.getCached(key); ok {
		return cached, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, subject, predicate, value, source, confidence, learned_at, expires_at, created_at
		FROM world_model_facts
		WHERE workspace_id = $1 AND subject = $2 AND expires_at > now()
		ORDER BY learned_at DESC`, workspaceID, subject)
	if err != nil {
		return nil, fmt.Errorf("world_model get_facts: %w", err)
	}
	defer rows.Close()

	facts, err := r.scanFacts(rows)
	if err != nil {
		return nil, err
	}

	r.putCache(key, facts)
	return facts, nil
}

// GetAllFacts returns all non-expired facts for a workspace.
func (r *PgWorldModelRepository) GetAllFacts(ctx context.Context, workspaceID uuid.UUID) ([]WorldFact, error) {
	key := wmCacheKey{workspaceID: workspaceID, subject: ""}
	if cached, ok := r.getCached(key); ok {
		return cached, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, workspace_id, subject, predicate, value, source, confidence, learned_at, expires_at, created_at
		FROM world_model_facts
		WHERE workspace_id = $1 AND expires_at > now()
		ORDER BY learned_at DESC`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("world_model get_all_facts: %w", err)
	}
	defer rows.Close()

	facts, err := r.scanFacts(rows)
	if err != nil {
		return nil, err
	}

	r.putCache(key, facts)
	return facts, nil
}

// ExpireFacts hard-deletes facts past their expires_at for a workspace.
func (r *PgWorldModelRepository) ExpireFacts(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM world_model_facts WHERE workspace_id = $1 AND expires_at <= now()`,
		workspaceID)
	if err != nil {
		return 0, fmt.Errorf("world_model expire_facts: %w", err)
	}
	r.invalidateWorkspace(workspaceID)
	return int(tag.RowsAffected()), nil
}

func (r *PgWorldModelRepository) invalidateWorkspace(workspaceID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.cache {
		if k.workspaceID == workspaceID {
			delete(r.cache, k)
		}
	}
}

func (r *PgWorldModelRepository) getCached(key wmCacheKey) ([]WorldFact, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.facts, true
}

func (r *PgWorldModelRepository) putCache(key wmCacheKey, facts []WorldFact) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = wmCacheEntry{
		facts:     facts,
		expiresAt: time.Now().Add(worldModelCacheTTL),
	}
}

type factRows interface {
	Next() bool
	Scan(dest ...any) error
}

func (r *PgWorldModelRepository) scanFacts(rows factRows) ([]WorldFact, error) {
	var facts []WorldFact
	for rows.Next() {
		var f WorldFact
		var wsID uuid.UUID
		if err := rows.Scan(&f.ID, &wsID, &f.Subject, &f.Predicate, &f.Value,
			&f.Source, &f.Confidence, &f.LearnedAt, &f.ExpiresAt, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("world_model scan: %w", err)
		}
		f.WorkspaceID = wsID.String()
		facts = append(facts, f)
	}
	if facts == nil {
		facts = []WorldFact{}
	}
	return facts, nil
}
