package memory

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DecayConfig configures memory decay behaviour.
type DecayConfig struct {
	HalfLifeDays     float64  `json:"half_life_days"`
	MinRetention     float64  `json:"min_retention"`
	DecayFunction    string   `json:"decay_function"` // "exponential" or "linear"
	MinWeight        float64  `json:"min_weight"`
	ExemptCategories []string `json:"exempt_categories"`
}

// MemoryItem represents a single memory entry managed by the decay service.
type MemoryItem struct {
	ID               uuid.UUID
	WorkspaceID      string
	Content          string
	RelevanceScore   float64
	Category         string
	LastAccessedAt   time.Time
	CreatedAt        time.Time
	Confidence       float64 // 0.0-1.0; 0 treated as 1.0 for backwards compat
	RetrievalCount   int
	BaseHalfLifeDays float64 // 0 falls back to config.HalfLifeDays
}

// MemoryDecayService applies temporal decay to stored memories.
type MemoryDecayService struct {
	mu    sync.Mutex
	items []MemoryItem
	now   func() time.Time
}

// NewMemoryDecayService creates a new decay service.
func NewMemoryDecayService() *MemoryDecayService {
	return &MemoryDecayService{
		now: func() time.Time { return time.Now().UTC() },
	}
}

// AddItem adds a memory item to the service and returns it with an assigned ID.
func (d *MemoryDecayService) AddItem(item MemoryItem) MemoryItem {
	d.mu.Lock()
	defer d.mu.Unlock()
	if item.ID == uuid.Nil {
		item.ID = uuid.Must(uuid.NewV7())
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = d.now()
	}
	if item.LastAccessedAt.IsZero() {
		item.LastAccessedAt = item.CreatedAt
	}
	d.items = append(d.items, item)
	return item
}

// ComputeWeight calculates the weight of a memory using exponential decay.
// weight = 2^(-elapsed_days / half_life_days)
func ComputeWeight(createdAt time.Time, now time.Time, halfLifeDays float64) float64 {
	if halfLifeDays <= 0 {
		return 1.0
	}
	elapsedDays := now.Sub(createdAt).Hours() / 24
	if elapsedDays < 0 {
		return 1.0
	}
	return math.Pow(2, -elapsedDays/halfLifeDays)
}

// computeLinearWeight calculates weight using linear decay.
func computeLinearWeight(createdAt time.Time, now time.Time, halfLifeDays float64) float64 {
	if halfLifeDays <= 0 {
		return 1.0
	}
	elapsedDays := now.Sub(createdAt).Hours() / 24
	if elapsedDays < 0 {
		return 1.0
	}
	// Linear decay reaches 0 at 2*halfLifeDays
	w := 1.0 - (elapsedDays / (2 * halfLifeDays))
	if w < 0 {
		w = 0
	}
	return w
}

// AdjustedHalfLife returns the effective half-life for an item that has been
// successfully recalled retrievalCount times.
// Formula: effective = base × (1 + ln(retrievalCount + 1))
func AdjustedHalfLife(baseHalfLifeDays float64, retrievalCount int) float64 {
	if baseHalfLifeDays <= 0 {
		baseHalfLifeDays = 30.0
	}
	if retrievalCount < 0 {
		retrievalCount = 0
	}
	return baseHalfLifeDays * (1.0 + math.Log(float64(retrievalCount)+1))
}

// applyDecayToItem applies confidence-dampened, spacing-adjusted exponential decay.
// Pure function — no IO. Modifies item.RelevanceScore in-place.
func applyDecayToItem(item *MemoryItem, elapsedDays float64, configHalfLife float64) {
	baseHL := item.BaseHalfLifeDays
	if baseHL <= 0 {
		baseHL = configHalfLife
	}
	effectiveHL := AdjustedHalfLife(baseHL, item.RetrievalCount)

	rawDecay := math.Exp(-0.693 * elapsedDays / effectiveHL)

	conf := item.Confidence
	if conf <= 0 {
		conf = 1.0
	}
	item.RelevanceScore = item.RelevanceScore * math.Pow(rawDecay, 1.0/math.Max(0.1, conf))

	if item.RelevanceScore < 0.01 && item.Confidence < 0.10 {
		item.RelevanceScore = 0
	}
}

// ShouldForget returns true if the weight has dropped below the minimum threshold.
func ShouldForget(weight, minWeight float64) bool {
	return weight < minWeight
}

// ApplyDecay applies decay to all memories in the given workspace.
// Returns the count of decayed memories and any error.
func (d *MemoryDecayService) ApplyDecay(workspaceID string, config DecayConfig) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if workspaceID == "" {
		return 0, fmt.Errorf("workspace_id is required")
	}
	if config.HalfLifeDays <= 0 {
		return 0, fmt.Errorf("half_life_days must be positive")
	}
	if config.DecayFunction == "" {
		config.DecayFunction = "exponential"
	}

	exempt := map[string]struct{}{}
	for _, cat := range config.ExemptCategories {
		exempt[cat] = struct{}{}
	}

	now := d.now()
	decayedCount := 0

	for i := range d.items {
		item := &d.items[i]
		if item.WorkspaceID != workspaceID {
			continue
		}
		if _, ok := exempt[item.Category]; ok {
			continue
		}

		var weight float64
		switch config.DecayFunction {
		case "linear":
			weight = computeLinearWeight(item.LastAccessedAt, now, config.HalfLifeDays)
		default:
			weight = ComputeWeight(item.LastAccessedAt, now, config.HalfLifeDays)
		}

		if weight < item.RelevanceScore {
			item.RelevanceScore = weight
			decayedCount++
		}
	}

	return decayedCount, nil
}

// RefreshMemory resets the relevance score of a memory item to 1.0 and updates
// its last accessed time.
func (d *MemoryDecayService) RefreshMemory(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	parsed, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	for i := range d.items {
		if d.items[i].ID == parsed {
			d.items[i].RelevanceScore = 1.0
			d.items[i].LastAccessedAt = d.now()
			d.items[i].RetrievalCount++
			return nil
		}
	}
	return fmt.Errorf("memory item not found: %s", id)
}

// GetDecayedMemories returns all memories in a workspace whose relevance score
// is below the given threshold.
func (d *MemoryDecayService) GetDecayedMemories(workspaceID string, threshold float64) []MemoryItem {
	d.mu.Lock()
	defer d.mu.Unlock()

	var result []MemoryItem
	for _, item := range d.items {
		if item.WorkspaceID == workspaceID && item.RelevanceScore < threshold {
			result = append(result, item)
		}
	}
	return result
}

// PurgeDecayed removes all memories in a workspace whose relevance score is
// below the given threshold. Returns the number of purged items.
func (d *MemoryDecayService) PurgeDecayed(workspaceID string, threshold float64) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if workspaceID == "" {
		return 0, fmt.Errorf("workspace_id is required")
	}

	purged := 0
	remaining := d.items[:0]
	for _, item := range d.items {
		if item.WorkspaceID == workspaceID && item.RelevanceScore < threshold {
			purged++
			continue
		}
		remaining = append(remaining, item)
	}
	d.items = remaining
	return purged, nil
}

// TestExportedDefaultHalfLife is exported for testing only.
func TestExportedDefaultHalfLife(memType string) float64 {
	return defaultBaseHalfLifeDays(memType)
}

// NewTestDecayService creates a MemoryDecayService for unit tests.
func NewTestDecayService() *MemoryDecayService {
	return NewMemoryDecayService()
}

// TestApplyDecay is exported for testing the decay formula in isolation.
func (d *MemoryDecayService) TestApplyDecay(item *MemoryItem, elapsedDays float64) {
	applyDecayToItem(item, elapsedDays, 30.0)
}
