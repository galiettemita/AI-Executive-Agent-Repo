package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	raptorMinClusterSize    = 3
	raptorMaxEpisodesPerRun = 200
)

// RAPTORConsolidationLLM defines the LLM contract for cluster summarization.
type RAPTORConsolidationLLM interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// RAPTORConsolidationEmbedder provides embedding for clustering.
type RAPTORConsolidationEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// RAPTORConsolidationRepo defines DB operations needed for consolidation.
type RAPTORConsolidationRepo interface {
	GetUnconsolidatedEpisodes(ctx context.Context, workspaceID string, limit int) ([]Item, error)
	MarkConsolidated(ctx context.Context, itemIDs []uuid.UUID, clusterID uuid.UUID) error
	InsertConsolidationSummary(ctx context.Context, clusterID uuid.UUID, summaryItemID uuid.UUID, episodeCount int, start, end time.Time) error
}

// RAPTORLogger is the logging interface for the consolidator.
type RAPTORLogger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// RAPTORConsolidator implements two-level hierarchical memory consolidation.
type RAPTORConsolidator struct {
	embedder RAPTORConsolidationEmbedder
	llm      RAPTORConsolidationLLM
	repo     RAPTORConsolidationRepo
	memorySvc *Service
	logger   RAPTORLogger
}

func NewRAPTORConsolidator(
	embedder RAPTORConsolidationEmbedder,
	llm RAPTORConsolidationLLM,
	repo RAPTORConsolidationRepo,
	memorySvc *Service,
	logger RAPTORLogger,
) *RAPTORConsolidator {
	return &RAPTORConsolidator{embedder: embedder, llm: llm, repo: repo, memorySvc: memorySvc, logger: logger}
}

const raptorSummarizationSystemPrompt = `You are a memory consolidation system for an AI executive assistant.
Summarize the following related memory episodes into a single coherent paragraph.

PRESERVE WITHOUT EXCEPTION:
- Exact dates, named entities (people, companies, projects)
- Key decisions made, action items, and open questions
- Source attribution where meaningful

EXCLUDE: filler phrases, redundant information, vague statements.
FORMAT: 2-4 prose sentences. Begin with the date range.
OUTPUT: ONLY the summary. No preamble, no meta-commentary.`

// ConsolidateWorkspace runs one RAPTOR consolidation cycle for a workspace.
func (r *RAPTORConsolidator) ConsolidateWorkspace(ctx context.Context, workspaceID string) error {
	episodes, err := r.repo.GetUnconsolidatedEpisodes(ctx, workspaceID, raptorMaxEpisodesPerRun)
	if err != nil {
		return fmt.Errorf("raptor: fetch episodes: %w", err)
	}
	if len(episodes) < raptorMinClusterSize {
		r.logger.Info("raptor: insufficient episodes to consolidate",
			"workspace_id", workspaceID, "count", len(episodes))
		return nil
	}

	texts := make([]string, len(episodes))
	for i, ep := range episodes {
		texts[i] = ep.Body
	}
	embeddings, err := r.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("raptor: embed episodes: %w", err)
	}

	clusters := kMeansCluster(episodes, embeddings, 100)

	r.logger.Info("raptor: k-means clustering complete",
		"workspace_id", workspaceID,
		"episodes", len(episodes),
		"k_clusters", len(clusters))

	for _, cluster := range clusters {
		if len(cluster) < raptorMinClusterSize {
			continue
		}
		if err := r.consolidateCluster(ctx, workspaceID, cluster); err != nil {
			r.logger.Error("raptor: cluster consolidation failed",
				"size", len(cluster), "error", err)
		}
	}

	return nil
}

func (r *RAPTORConsolidator) consolidateCluster(ctx context.Context, workspaceID string, cluster []Item) error {
	var sb strings.Builder
	for i, ep := range cluster {
		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n",
			i+1, ep.CreatedAt.Format("2006-01-02"), ep.Body))
	}

	summary, err := r.llm.Complete(ctx, raptorSummarizationSystemPrompt,
		fmt.Sprintf("Consolidate these %d related episodes:\n\n%s", len(cluster), sb.String()))
	if err != nil {
		return fmt.Errorf("LLM summarization: %w", err)
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("LLM returned empty summary")
	}

	clusterID := uuid.New()

	sort.Slice(cluster, func(i, j int) bool {
		return cluster[i].CreatedAt.Before(cluster[j].CreatedAt)
	})
	periodStart := cluster[0].CreatedAt
	periodEnd := cluster[len(cluster)-1].CreatedAt

	// Store summary as a Level-1 semantic memory via the service
	summaryItem, err := r.memorySvc.Write(workspaceID, cluster[0].UserID, "semantic", summary)
	if err != nil {
		return fmt.Errorf("insert summary item: %w", err)
	}

	// Mark source episodes as consolidated (NOT deleted)
	ids := make([]uuid.UUID, len(cluster))
	for i, ep := range cluster {
		ids[i] = ep.ID
	}
	if err := r.repo.MarkConsolidated(ctx, ids, clusterID); err != nil {
		r.logger.Error("raptor: mark consolidated failed (non-fatal)",
			"cluster_id", clusterID, "error", err)
	}

	_ = r.repo.InsertConsolidationSummary(ctx, clusterID, summaryItem.ID, len(cluster), periodStart, periodEnd)

	r.logger.Info("raptor: cluster consolidated",
		"cluster_id", clusterID,
		"episodes", len(cluster),
		"summary_item_id", summaryItem.ID)
	return nil
}

// kMeansCluster groups episodes into k clusters using Lloyd's k-means algorithm.
// k = max(2, round(sqrt(n/10))), per audit §5 UPGRADE-06.
// Deterministic: seeds centroids from first k episodes.
func kMeansCluster(episodes []Item, embeddings [][]float32, maxIter int) [][]Item {
	n := len(episodes)
	if n == 0 {
		return nil
	}

	k := ComputeK(n)
	if maxIter <= 0 {
		maxIter = 100
	}

	dims := len(embeddings[0])

	// Seed centroids deterministically from first k episodes
	centroids := make([][]float64, k)
	for i := 0; i < k; i++ {
		centroids[i] = f32ToF64(embeddings[i])
	}

	assignments := make([]int, n)

	for iter := 0; iter < maxIter; iter++ {
		changed := false

		// Assignment step
		for i, emb := range embeddings {
			best := 0
			bestSim := -2.0
			embF64 := f32ToF64(emb)
			for c, centroid := range centroids {
				sim := cosSimF64(embF64, centroid)
				if sim > bestSim {
					bestSim = sim
					best = c
				}
			}
			if assignments[i] != best {
				assignments[i] = best
				changed = true
			}
		}

		if !changed {
			break
		}

		// Update step
		newCentroids := make([][]float64, k)
		counts := make([]int, k)
		for c := 0; c < k; c++ {
			newCentroids[c] = make([]float64, dims)
		}
		for i, emb := range embeddings {
			c := assignments[i]
			for d, v := range emb {
				newCentroids[c][d] += float64(v)
			}
			counts[c]++
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for d := range newCentroids[c] {
					newCentroids[c][d] /= float64(counts[c])
				}
			}
		}

		// Convergence check
		maxDelta := 0.0
		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				continue
			}
			delta := eucDistF64(centroids[c], newCentroids[c])
			if delta > maxDelta {
				maxDelta = delta
			}
		}
		centroids = newCentroids
		if maxDelta < 1e-6 {
			break
		}
	}

	// Build output, skip empty clusters
	clusters := make([][]Item, k)
	for i, ep := range episodes {
		c := assignments[i]
		clusters[c] = append(clusters[c], ep)
	}

	result := clusters[:0]
	for _, cluster := range clusters {
		if len(cluster) > 0 {
			result = append(result, cluster)
		}
	}
	return result
}

// ComputeK returns the k value for k-means: max(2, round(sqrt(n/10))), capped at n.
func ComputeK(n int) int {
	k := int(math.Round(math.Sqrt(float64(n) / 10.0)))
	if k < 2 {
		k = 2
	}
	if k > n {
		k = n
	}
	return k
}

func f32ToF64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = float64(x)
	}
	return out
}

func cosSimF64(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

func eucDistF64(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// EpochConfig configures how Level-2 epoch summaries are generated.
type EpochConfig struct {
	PeriodDays          int // time window for one epoch (default: 30)
	MinClusterSummaries int // minimum L1 summaries required (default: 3)
}

func DefaultEpochConfig() EpochConfig {
	return EpochConfig{PeriodDays: 30, MinClusterSummaries: 3}
}

// EpochSummaryRepo provides the data access for epoch generation.
type EpochSummaryRepo interface {
	GetLevel1SummariesSince(ctx context.Context, workspaceID string, since time.Time) ([]Item, error)
	GetEpochSummaryForPeriod(ctx context.Context, workspaceID string, start, end time.Time) (*Item, error)
}

// EpochLLMSummarizer generates epoch-level summaries from L1 cluster summaries.
type EpochLLMSummarizer interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// GenerateEpochSummaries creates Level-2 summaries from Level-1 cluster summaries.
// Idempotent: safe to run repeatedly for the same period.
func (r *RAPTORConsolidator) GenerateEpochSummaries(
	ctx context.Context,
	workspaceID string,
	cfg EpochConfig,
	epochRepo EpochSummaryRepo,
	epochLLM EpochLLMSummarizer,
) error {
	if cfg.PeriodDays <= 0 {
		cfg.PeriodDays = 30
	}
	if cfg.MinClusterSummaries <= 0 {
		cfg.MinClusterSummaries = 3
	}

	periodStart := time.Now().UTC().AddDate(0, 0, -cfg.PeriodDays)
	periodEnd := time.Now().UTC()

	clusterSummaries, err := epochRepo.GetLevel1SummariesSince(ctx, workspaceID, periodStart)
	if err != nil {
		return fmt.Errorf("epoch: fetch L1 summaries: %w", err)
	}

	if len(clusterSummaries) < cfg.MinClusterSummaries {
		r.logger.Info("epoch: not enough L1 summaries",
			"workspace_id", workspaceID,
			"count", len(clusterSummaries),
			"minimum", cfg.MinClusterSummaries)
		return nil
	}

	// Idempotency check
	existing, err := epochRepo.GetEpochSummaryForPeriod(ctx, workspaceID, periodStart, periodEnd)
	if err != nil {
		return fmt.Errorf("epoch: check existing: %w", err)
	}
	if existing != nil {
		r.logger.Info("epoch: summary already exists for period",
			"workspace_id", workspaceID)
		return nil
	}

	// Build LLM prompt
	var sb strings.Builder
	for i, cs := range clusterSummaries {
		sb.WriteString(fmt.Sprintf("[Cluster %d - %s]: %s\n\n",
			i+1, cs.CreatedAt.Format("Jan 2"), cs.Body))
	}

	systemPrompt := fmt.Sprintf(`You are creating a high-level epoch summary for an AI executive assistant.
The period is %s to %s (%d days). You have %d cluster-level summaries.

Generate a single coherent epoch summary:
1. MAJOR THEMES: 2-4 dominant focus areas
2. KEY OUTCOMES: Decisions finalized, milestones reached
3. ONGOING THREADS: Active but unresolved items
4. KEY PEOPLE & PROJECTS: Most frequent named entities

FORMAT: 2-sentence overview, then "Key outcomes:" and "Ongoing:" bullet points.
Maximum 300 words. Output ONLY the summary.`,
		periodStart.Format("Jan 2, 2006"),
		periodEnd.Format("Jan 2, 2006"),
		cfg.PeriodDays,
		len(clusterSummaries))

	summary, err := epochLLM.Complete(ctx, systemPrompt, sb.String())
	if err != nil {
		return fmt.Errorf("epoch: LLM summarization failed: %w", err)
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("epoch: LLM returned empty summary")
	}

	// Store as Level-2 memory via the service
	if r.memorySvc != nil {
		item, writeErr := r.memorySvc.Write(workspaceID,
			clusterSummaries[0].UserID, "semantic", summary)
		if writeErr != nil {
			return fmt.Errorf("epoch: store summary: %w", writeErr)
		}
		_ = item
	}

	r.logger.Info("epoch: Level-2 summary generated",
		"workspace_id", workspaceID,
		"period_start", periodStart.Format("2006-01-02"),
		"source_l1_count", len(clusterSummaries))
	return nil
}
