package crdt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PersistentVectorClockStore defines the persistence interface for vector
// clocks used by the CRDT memory subsystem.
type PersistentVectorClockStore interface {
	Save(ctx context.Context, key string, clock VectorClock) error
	Load(ctx context.Context, key string) (*VectorClock, error)
	Delete(ctx context.Context, key string) error
	ListKeys(ctx context.Context, prefix string) ([]string, error)
}

// MergeResult captures the outcome of merging a persisted clock with an
// incoming clock.
type MergeResult struct {
	Merged           VectorClock `json:"merged"`
	ConflictDetected bool        `json:"conflict_detected"`
	Resolution       string      `json:"resolution"`
}

// PostgresPersistence implements PersistentVectorClockStore backed by a
// PostgreSQL table: memory_vector_clocks (key TEXT PRIMARY KEY, clock_data JSONB, updated_at TIMESTAMPTZ).
type PostgresPersistence struct {
	pool *pgxpool.Pool
}

// NewPostgresPersistence creates a PostgresPersistence with the given pgx
// connection pool. Callers should ensure the memory_vector_clocks table exists.
func NewPostgresPersistence(pool *pgxpool.Pool) *PostgresPersistence {
	return &PostgresPersistence{pool: pool}
}

// EnsureTable creates the memory_vector_clocks table if it does not exist.
func (p *PostgresPersistence) EnsureTable(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS memory_vector_clocks (
		key TEXT PRIMARY KEY,
		clock_data JSONB NOT NULL DEFAULT '{}',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`
	_, err := p.pool.Exec(ctx, query)
	return err
}

func (p *PostgresPersistence) Save(ctx context.Context, key string, clock VectorClock) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key is required")
	}
	data, err := json.Marshal(clock)
	if err != nil {
		return fmt.Errorf("marshal clock: %w", err)
	}
	query := `INSERT INTO memory_vector_clocks (key, clock_data, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET clock_data = $2, updated_at = NOW()`
	_, err = p.pool.Exec(ctx, query, key, data)
	if err != nil {
		return fmt.Errorf("save clock %q: %w", key, err)
	}
	return nil
}

func (p *PostgresPersistence) Load(ctx context.Context, key string) (*VectorClock, error) {
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("key is required")
	}
	var data []byte
	query := `SELECT clock_data FROM memory_vector_clocks WHERE key = $1`
	err := p.pool.QueryRow(ctx, query, key).Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("load clock %q: %w", key, err)
	}
	var clock VectorClock
	if err := json.Unmarshal(data, &clock); err != nil {
		return nil, fmt.Errorf("unmarshal clock %q: %w", key, err)
	}
	return &clock, nil
}

func (p *PostgresPersistence) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key is required")
	}
	query := `DELETE FROM memory_vector_clocks WHERE key = $1`
	_, err := p.pool.Exec(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete clock %q: %w", key, err)
	}
	return nil
}

func (p *PostgresPersistence) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	query := `SELECT key FROM memory_vector_clocks WHERE key LIKE $1 ORDER BY key`
	rows, err := p.pool.Query(ctx, query, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("list keys with prefix %q: %w", prefix, err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate keys: %w", err)
	}
	return keys, nil
}

// InMemoryPersistence implements PersistentVectorClockStore for testing.
type InMemoryPersistence struct {
	mu    sync.RWMutex
	store map[string]VectorClock
	now   func() time.Time
}

// NewInMemoryPersistence creates an InMemoryPersistence store.
func NewInMemoryPersistence() *InMemoryPersistence {
	return &InMemoryPersistence{
		store: make(map[string]VectorClock),
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (m *InMemoryPersistence) Save(_ context.Context, key string, clock VectorClock) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = copyClock(clock)
	return nil
}

func (m *InMemoryPersistence) Load(_ context.Context, key string) (*VectorClock, error) {
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("key is required")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	clock, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("clock %q not found", key)
	}
	c := copyClock(clock)
	return &c, nil
}

func (m *InMemoryPersistence) Delete(_ context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

func (m *InMemoryPersistence) ListKeys(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for key := range m.store {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// MergeWithPersistence loads the existing clock for the given key from the
// store, merges it with the incoming clock, detects conflicts (concurrent
// writes), persists the merged result, and returns the outcome.
func MergeWithPersistence(ctx context.Context, store PersistentVectorClockStore, key string, incoming VectorClock) (*MergeResult, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("key is required")
	}

	existing, err := store.Load(ctx, key)
	if err != nil {
		// Key does not exist yet — simply save the incoming clock.
		if saveErr := store.Save(ctx, key, incoming); saveErr != nil {
			return nil, fmt.Errorf("save new clock: %w", saveErr)
		}
		return &MergeResult{
			Merged:           copyClock(incoming),
			ConflictDetected: false,
			Resolution:       "new_key",
		}, nil
	}

	relation := compareClocks(*existing, incoming)
	merged := mergeClock(*existing, incoming)

	conflictDetected := false
	resolution := "fast_forward"

	switch relation {
	case relationConcurrent:
		conflictDetected = true
		resolution = "concurrent_merge"
	case relationEqual:
		resolution = "no_change"
	case relationLocalDominates:
		resolution = "local_dominates"
	case relationRemoteDominates:
		resolution = "fast_forward"
	}

	if err := store.Save(ctx, key, merged); err != nil {
		return nil, fmt.Errorf("save merged clock: %w", err)
	}

	return &MergeResult{
		Merged:           merged,
		ConflictDetected: conflictDetected,
		Resolution:       resolution,
	}, nil
}
