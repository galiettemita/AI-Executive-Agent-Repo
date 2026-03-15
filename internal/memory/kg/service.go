package kg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service is the top-level public API for the knowledge graph subsystem.
type Service struct {
	extractor *LLMExtractor
	retriever *Retriever
	repo      *Repository
	embedder  Embedder
	logger    Logger
}

func NewService(
	extractor *LLMExtractor,
	retriever *Retriever,
	repo *Repository,
	embedder Embedder,
	logger Logger,
) *Service {
	return &Service{
		extractor: extractor,
		retriever: retriever,
		repo:      repo,
		embedder:  embedder,
		logger:    logger,
	}
}

// ExtractAndStore runs triple extraction and persists results.
// MUST be called asynchronously (in a goroutine).
func (s *Service) ExtractAndStore(ctx context.Context, req ExtractionRequest) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("kg_service: panic in ExtractAndStore",
				"panic", fmt.Sprintf("%v", r))
		}
	}()

	if s.extractor == nil {
		return
	}

	triples, err := s.extractor.Extract(ctx, req)
	if err != nil || len(triples) == 0 {
		return
	}

	s.logger.Info("kg_service: extracted triples",
		"turn_id", req.TurnID, "count", len(triples))

	for _, et := range triples {
		triple := Triple{
			ID:           uuid.New().String(),
			WorkspaceID:  req.WorkspaceID,
			Subject:      strings.TrimSpace(et.Subject),
			Predicate:    et.Predicate,
			Object:       strings.TrimSpace(et.Object),
			SubjectType:  et.SubjectType,
			ObjectType:   et.ObjectType,
			Confidence:   et.Confidence,
			SourceTurnID: req.TurnID,
			CreatedAt:    time.Now().UTC(),
		}

		if s.repo != nil {
			if err := s.repo.UpsertTriple(ctx, triple); err != nil {
				s.logger.Error("kg_service: upsert failed",
					"subject", triple.Subject, "error", err)
				continue
			}
		}

		// Embed entities asynchronously
		if s.embedder != nil && s.repo != nil {
			go s.embedAndStoreEntities(
				context.Background(),
				req.WorkspaceID,
				triple.Subject,
				triple.Object,
			)
		}
	}
}

func (s *Service) embedAndStoreEntities(ctx context.Context, workspaceID, subject, object string) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("kg_service: panic in embedAndStoreEntities")
		}
	}()

	texts := []string{subject, object}
	embeddings, err := s.embedder.Embed(ctx, texts)
	if err != nil || len(embeddings) < 2 {
		s.logger.Warn("kg_service: entity embed failed",
			"subject", subject, "object", object, "error", err)
		return
	}

	if err := s.repo.UpdateSubjectEmbedding(ctx, workspaceID, subject, embeddings[0]); err != nil {
		s.logger.Warn("kg_service: update subject embedding failed", "error", err)
	}
	if err := s.repo.UpdateObjectEmbedding(ctx, workspaceID, object, embeddings[1]); err != nil {
		s.logger.Warn("kg_service: update object embedding failed", "error", err)
	}
}

// QueryForContext is the primary retrieval entry point for context assembly.
// Returns "" on any failure — designed for the hot path.
func (s *Service) QueryForContext(ctx context.Context, workspaceID, queryText string) string {
	if s.retriever == nil {
		return ""
	}
	result, err := s.retriever.Query(ctx, workspaceID, queryText, 2)
	if err != nil {
		s.logger.Warn("kg_service: query failed", "error", err)
		return ""
	}
	if result == nil {
		return ""
	}
	return result.ContextSnippet
}
