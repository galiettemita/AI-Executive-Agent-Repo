package temporal

import (
	"context"
	"fmt"
	"time"

	contextlayer "github.com/brevio/brevio/internal/context"
	"github.com/brevio/brevio/internal/executor"
	"github.com/brevio/brevio/internal/fastpath"
	"github.com/brevio/brevio/internal/memory"
	"github.com/brevio/brevio/internal/rag"
)

// V10.2 P8 Activity Input/Output types.

type ApplyMemoryDecayInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	DecayFunction string  `json:"decay_function"`
	HalfLifeDays  float64 `json:"half_life_days"`
	MinWeight     float64 `json:"min_weight"`
	Purge         bool    `json:"purge"`
}

type ApplyMemoryDecayResult struct {
	ItemsDecayed int  `json:"items_decayed"`
	ItemsPurged  int  `json:"items_purged"`
	Persisted    bool `json:"persisted"`
}

type DetectLessonConflictInput struct {
	WorkspaceID      string `json:"workspace_id"`
	ExistingLessonID string `json:"existing_lesson_id"`
	IncomingLessonID string `json:"incoming_lesson_id"`
	ConflictType     string `json:"conflict_type"`
}

type DetectLessonConflictResult struct {
	ConflictRecorded bool   `json:"conflict_recorded"`
	Resolution       string `json:"resolution"`
}

type ResolveLessonConflictInput struct {
	ConflictID       string `json:"conflict_id"`
	Resolution       string `json:"resolution"`
	ResolvedBy       string `json:"resolved_by"`
	ResolutionDetail string `json:"resolution_detail"`
}

type ResolveLessonConflictResult struct {
	Resolved bool `json:"resolved"`
}

type EmbedAndChunkInput struct {
	WorkspaceID  string `json:"workspace_id"`
	CollectionID string `json:"collection_id"`
	Text         string `json:"text"`
}

type EmbedAndChunkResult struct {
	ChunkCount      int  `json:"chunk_count"`
	Dimensions      int  `json:"dimensions"`
	ChunksPersisted int  `json:"chunks_persisted"`
	SpecPersisted   bool `json:"spec_persisted"`
}

type RankWithFreshnessInput struct {
	WorkspaceID    string    `json:"workspace_id"`
	SemanticScore  float64   `json:"semantic_score"`
	DocumentAge    int64     `json:"document_age_seconds"`
	TemporalLambda float64   `json:"temporal_lambda"`
	MaxAgeDays     int       `json:"max_age_days"`
}

type RankWithFreshnessResult struct {
	CombinedScore float64 `json:"combined_score"`
}

type PersistCompressionInput struct {
	WorkspaceID       string   `json:"workspace_id"`
	SessionID         string   `json:"session_id"`
	OriginalTurnCount int      `json:"original_turn_count"`
	CompressedCount   int      `json:"compressed_count"`
	EntityRefs        []string `json:"entity_refs"`
	SummaryText       string   `json:"summary_text"`
	TokenSavings      int      `json:"token_savings"`
}

type PersistCompressionResult struct {
	Persisted bool `json:"persisted"`
}

type EnforceContextBudgetInput struct {
	WorkspaceID             string `json:"workspace_id"`
	IngressTurnID           string `json:"ingress_turn_id"`
	PromptRequestedTokens   int    `json:"prompt_requested_tokens"`
	RAGRequestedTokens      int    `json:"rag_requested_tokens"`
	HistoryRequestedTokens  int    `json:"history_requested_tokens"`
}

type EnforceContextBudgetResult struct {
	AllocatedPromptTokens  int  `json:"allocated_prompt_tokens"`
	AllocatedRAGTokens     int  `json:"allocated_rag_tokens"`
	AllocatedHistoryTokens int  `json:"allocated_history_tokens"`
	Overflowed             bool `json:"overflowed"`
	Persisted              bool `json:"persisted"`
}

type EvaluateLatencyBudgetInput struct {
	WorkspaceID     string  `json:"workspace_id"`
	WorkflowRunID   string  `json:"workflow_run_id"`
	BudgetMs        float64 `json:"budget_ms"`
	ElapsedMs       float64 `json:"elapsed_ms"`
	EstimatedNextMs float64 `json:"estimated_next_ms"`
}

type EvaluateLatencyBudgetResult struct {
	ShouldProceed     bool    `json:"should_proceed"`
	Reason            string  `json:"reason"`
	RemainingBudgetMs float64 `json:"remaining_budget_ms"`
	Persisted         bool    `json:"persisted"`
}

type WarmFastPathCacheInput struct {
	WorkspaceID string `json:"workspace_id"`
	MaxRoutes   int    `json:"max_routes"`
}

type WarmFastPathCacheResult struct {
	RoutesWarmed  int  `json:"routes_warmed"`
	AnswersCached int  `json:"answers_cached"`
	RateLimited   bool `json:"rate_limited"`
}

// ApplyMemoryDecayActivity runs memory decay and persists results to DB.
func (a *Activities) ApplyMemoryDecayActivity(ctx context.Context, input ApplyMemoryDecayInput) (*ApplyMemoryDecayResult, error) {
	config := memory.DecayConfig{
		HalfLifeDays:  input.HalfLifeDays,
		MinRetention:  0.1,
		DecayFunction: input.DecayFunction,
		MinWeight:     input.MinWeight,
	}
	if config.DecayFunction == "" {
		config.DecayFunction = "exponential"
	}
	if config.HalfLifeDays <= 0 {
		config.HalfLifeDays = 30
	}
	if config.MinWeight <= 0 {
		config.MinWeight = 0.05
	}

	svc := memory.NewMemoryDecayService()
	decayed, err := svc.ApplyDecay(input.WorkspaceID, config)
	if err != nil {
		return nil, fmt.Errorf("apply memory decay: %w", err)
	}

	purged := 0
	if input.Purge {
		purged, err = svc.PurgeDecayed(input.WorkspaceID, config.MinWeight)
		if err != nil {
			return nil, fmt.Errorf("purge decayed: %w", err)
		}
	}

	result := &ApplyMemoryDecayResult{
		ItemsDecayed: decayed,
		ItemsPurged:  purged,
	}

	if a.pool != nil && a.decayRepo != nil {
		logErr := a.decayRepo.RecordDecaySweep(ctx, memory.DecayLogRow{
			WorkspaceID:   input.WorkspaceID,
			DecayFunction: config.DecayFunction,
			HalfLifeDays:  config.HalfLifeDays,
			ItemsDecayed:  decayed,
			ItemsPurged:   purged,
			MinWeight:     config.MinWeight,
			SweptAt:       time.Now().UTC(),
		})
		result.Persisted = logErr == nil
	}

	return result, nil
}

// DetectLessonConflictActivity records a lesson conflict in the DB.
func (a *Activities) DetectLessonConflictActivity(ctx context.Context, input DetectLessonConflictInput) (*DetectLessonConflictResult, error) {
	if a.pool == nil || a.conflictRepo == nil {
		return &DetectLessonConflictResult{ConflictRecorded: false, Resolution: "manual_review"}, nil
	}

	err := a.conflictRepo.RecordConflict(ctx, memory.LessonConflictRow{
		WorkspaceID:      input.WorkspaceID,
		ExistingLessonID: input.ExistingLessonID,
		IncomingLessonID: input.IncomingLessonID,
		ConflictType:     input.ConflictType,
		Resolution:       "manual_review",
	})
	return &DetectLessonConflictResult{
		ConflictRecorded: err == nil,
		Resolution:       "manual_review",
	}, err
}

// ResolveLessonConflictActivity resolves a lesson conflict.
func (a *Activities) ResolveLessonConflictActivity(ctx context.Context, input ResolveLessonConflictInput) (*ResolveLessonConflictResult, error) {
	if a.pool == nil || a.conflictRepo == nil {
		return &ResolveLessonConflictResult{Resolved: false}, nil
	}

	err := a.conflictRepo.ResolveConflict(ctx, input.ConflictID, input.Resolution, input.ResolvedBy, input.ResolutionDetail)
	return &ResolveLessonConflictResult{Resolved: err == nil}, err
}

// EmbedAndChunkActivity embeds text and persists chunk spec.
func (a *Activities) EmbedAndChunkActivity(ctx context.Context, input EmbedAndChunkInput) (*EmbedAndChunkResult, error) {
	// Use deterministic provider in degraded mode, OpenAI in production.
	var provider rag.EmbeddingProvider
	if a.embeddingProvider != nil {
		provider = a.embeddingProvider
	} else {
		provider = rag.NewDeterministicEmbeddingProvider(1536)
	}

	// Chunk the text (fixed_token strategy).
	chunkSize := 512
	overlap := 64
	chunks := chunkText(input.Text, chunkSize, overlap)

	// Embed chunks.
	svc := rag.NewEmbeddingService(provider)
	embeddings, err := svc.BatchEmbed(ctx, chunks, 64)
	if err != nil {
		return nil, fmt.Errorf("embed chunks: %w", err)
	}

	// Persist chunks with embeddings to pgvector store when available.
	chunksPersisted := 0
	if a.vectorStore != nil {
		for i, chunkText := range chunks {
			if i >= len(embeddings) || len(embeddings[i]) == 0 {
				continue
			}
			chunkID := fmt.Sprintf("%s:%s:%d", input.WorkspaceID, input.CollectionID, i)
			upsertErr := a.vectorStore.UpsertChunk(ctx, rag.ChunkWithEmbedding{
				ChunkID:      chunkID,
				WorkspaceID:  input.WorkspaceID,
				CollectionID: input.CollectionID,
				Content:      chunkText,
				Embedding:    embeddings[i],
			})
			if upsertErr == nil {
				chunksPersisted++
			}
		}
	}

	result := &EmbedAndChunkResult{
		ChunkCount:      len(chunks),
		Dimensions:      provider.Dimensions(),
		ChunksPersisted: chunksPersisted,
	}

	if a.pool != nil && a.chunkSpecRepo != nil {
		specErr := a.chunkSpecRepo.UpsertChunkSpec(ctx, rag.ChunkSpecRow{
			WorkspaceID:    input.WorkspaceID,
			CollectionID:   input.CollectionID,
			ChunkStrategy:  "fixed_token",
			ChunkSize:      chunkSize,
			ChunkOverlap:   overlap,
			EmbeddingModel: "text-embedding-3-small",
			Dimensions:     provider.Dimensions(),
		})
		result.SpecPersisted = specErr == nil
	}

	return result, nil
}

// chunkText splits text into overlapping chunks of approximately chunkSize tokens.
func chunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 0
	}
	// Approximate: 1 token ≈ 4 chars.
	chunkChars := chunkSize * 4
	overlapChars := overlap * 4

	if len(text) <= chunkChars {
		return []string{text}
	}

	var chunks []string
	for start := 0; start < len(text); {
		end := start + chunkChars
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		start = end - overlapChars
		if start <= 0 && end >= len(text) {
			break
		}
		if start >= len(text) {
			break
		}
	}
	return chunks
}

// RankWithFreshnessActivity applies freshness scoring to a retrieval result.
func (a *Activities) RankWithFreshnessActivity(ctx context.Context, input RankWithFreshnessInput) (*RankWithFreshnessResult, error) {
	scorer := rag.NewFreshnessScorer()
	config := rag.FreshnessConfig{
		TemporalLambda: input.TemporalLambda,
		MaxAgeDays:     input.MaxAgeDays,
	}
	if config.TemporalLambda <= 0 {
		config.TemporalLambda = 0.7
	}
	if config.MaxAgeDays <= 0 {
		config.MaxAgeDays = 30
	}

	docAge := time.Duration(input.DocumentAge) * time.Second
	combined := scorer.ScoreWithFreshness(input.SemanticScore, docAge, config)

	return &RankWithFreshnessResult{CombinedScore: combined}, nil
}

// PersistCompressionActivity persists a compression artifact to the DB.
func (a *Activities) PersistCompressionActivity(ctx context.Context, input PersistCompressionInput) (*PersistCompressionResult, error) {
	if a.pool == nil || a.compressionRepo == nil {
		return &PersistCompressionResult{Persisted: false}, nil
	}

	err := a.compressionRepo.RecordCompression(ctx, contextlayer.CompressionArtifactRow{
		WorkspaceID:       input.WorkspaceID,
		SessionID:         input.SessionID,
		OriginalTurnCount: input.OriginalTurnCount,
		CompressedCount:   input.CompressedCount,
		EntityRefs:        input.EntityRefs,
		SummaryText:       input.SummaryText,
		TokenSavings:      input.TokenSavings,
	})
	return &PersistCompressionResult{Persisted: err == nil}, err
}

// EnforceContextBudgetActivity enforces context budget and persists allocation.
func (a *Activities) EnforceContextBudgetActivity(ctx context.Context, input EnforceContextBudgetInput) (*EnforceContextBudgetResult, error) {
	svc := contextlayer.NewService()

	// Ensure budget exists.
	if _, ok := svc.GetBudget(input.WorkspaceID); !ok {
		svc.UpsertBudgetConfig(input.WorkspaceID, "T2", 32000, 256, 512, "active")
	}

	report, allocErr := svc.AllocateContext(
		input.WorkspaceID, input.IngressTurnID,
		input.PromptRequestedTokens, input.RAGRequestedTokens, input.HistoryRequestedTokens,
	)

	result := &EnforceContextBudgetResult{
		AllocatedPromptTokens:  report.AllocatedPromptTokens,
		AllocatedRAGTokens:     report.AllocatedRAGTokens,
		AllocatedHistoryTokens: report.AllocatedHistoryTokens,
		Overflowed:             report.Overflowed,
	}

	// Persist to DB if available.
	if a.pool != nil && a.contextRepo != nil {
		budget := contextlayer.Budget{
			WorkspaceID:      input.WorkspaceID,
			MaxContextTokens: 32000,
			Status:           "active",
		}
		_, _ = a.contextRepo.UpsertBudget(ctx, budget)
		_ = a.contextRepo.RecordAuditEvent(ctx, input.WorkspaceID, map[string]any{
			"ingress_turn_id":  input.IngressTurnID,
			"prompt_allocated": report.AllocatedPromptTokens,
			"rag_allocated":    report.AllocatedRAGTokens,
			"history_allocated": report.AllocatedHistoryTokens,
			"overflowed":       report.Overflowed,
		})
		result.Persisted = true
	}

	if allocErr != nil && !report.Overflowed {
		return result, allocErr
	}

	return result, nil
}

// EvaluateLatencyBudgetActivity evaluates latency budget and persists the decision.
func (a *Activities) EvaluateLatencyBudgetActivity(ctx context.Context, input EvaluateLatencyBudgetInput) (*EvaluateLatencyBudgetResult, error) {
	preemptor := executor.NewLatencyPreemptor()
	decision := preemptor.ShouldProceed(input.BudgetMs, input.ElapsedMs, input.EstimatedNextMs)

	result := &EvaluateLatencyBudgetResult{
		ShouldProceed:     decision.ShouldProceed,
		Reason:            decision.Reason,
		RemainingBudgetMs: decision.RemainingBudgetMs,
	}

	if a.pool != nil && a.latencyRepo != nil {
		logErr := a.latencyRepo.RecordDecision(ctx, executor.LatencyBudgetLogRow{
			WorkspaceID:       input.WorkspaceID,
			WorkflowRunID:     input.WorkflowRunID,
			BudgetMs:          input.BudgetMs,
			ElapsedMs:         input.ElapsedMs,
			EstimatedNextMs:   input.EstimatedNextMs,
			ShouldProceed:     decision.ShouldProceed,
			Reason:            decision.Reason,
			RemainingBudgetMs: decision.RemainingBudgetMs,
		})
		result.Persisted = logErr == nil
	}

	return result, nil
}

// WarmFastPathCacheActivity warms the fast-path cache with rate limiting.
// Seeds default routes and warms the cache with known high-volume inputs.
func (a *Activities) WarmFastPathCacheActivity(ctx context.Context, input WarmFastPathCacheInput) (*WarmFastPathCacheResult, error) {
	maxRoutes := input.MaxRoutes
	if maxRoutes <= 0 {
		maxRoutes = 100
	}

	// Rate limiting: max 100 routes per warm cycle.
	if maxRoutes > 100 {
		return &WarmFastPathCacheResult{RateLimited: true}, nil
	}

	// Seed default routes into a FastPathService instance.
	fpSvc := fastpath.NewFastPathService()
	if err := fastpath.SeedDefaultRoutes(fpSvc); err != nil {
		return &WarmFastPathCacheResult{RoutesWarmed: 0, AnswersCached: 0}, nil
	}

	// Pre-warm the cache with known high-volume inputs.
	testInputs := []string{"hi", "hello", "thank you", "what can you do", "ok"}
	cached := 0
	for _, input := range testInputs {
		if _, ok := fpSvc.Match(input); ok {
			cached++
		}
	}

	return &WarmFastPathCacheResult{
		RoutesWarmed:  len(fastpath.DefaultRoutes()),
		AnswersCached: cached,
		RateLimited:   false,
	}, nil
}
