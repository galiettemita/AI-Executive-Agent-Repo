package memory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MemoryLink represents an associative relationship between two memory items.
type MemoryLink struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	SourceID    string    `json:"source_id" db:"source_id"`
	TargetID    string    `json:"target_id" db:"target_id"`
	LinkType    string    `json:"link_type" db:"link_type"`
	Strength    float64   `json:"strength" db:"strength"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	CreatedBy   string    `json:"created_by" db:"created_by"`
}

const (
	LinkTypeAssociative   = "associative"
	LinkTypeCausal        = "causal"
	LinkTypeContradiction = "contradiction"
	LinkTypeElaboration   = "elaboration"

	AutoLinkThreshold   = 0.85
	MaxAutoLinksPerItem  = 5
)

// LinkRepository handles DB operations for memory links.
type LinkRepository interface {
	CreateLink(ctx context.Context, link MemoryLink) error
	GetLinkedItemIDs(ctx context.Context, workspaceID, itemID string) ([]string, error)
	GetItemsByIDs(ctx context.Context, workspaceID string, ids []string) ([]Item, error)
}

// LinkEmbedder provides embedding for auto-linking.
type LinkEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// LinkSearcher provides embedding search for auto-linking.
type LinkSearcher interface {
	SearchByVector(ctx context.Context, workspaceID string, vec []float32, k int) ([]Item, error)
}

// LinkLogger is the minimal logging contract for LinkService.
type LinkLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// LinkService manages automatic link creation and graph-walk retrieval.
type LinkService struct {
	linkRepo   LinkRepository
	searchRepo LinkSearcher
	embedder   LinkEmbedder
	logger     LinkLogger
}

func NewLinkService(
	linkRepo LinkRepository,
	searchRepo LinkSearcher,
	embedder LinkEmbedder,
	logger LinkLogger,
) *LinkService {
	return &LinkService{
		linkRepo:   linkRepo,
		searchRepo: searchRepo,
		embedder:   embedder,
		logger:     logger,
	}
}

// AutoLink finds semantically similar existing memories and creates associative links.
// Called asynchronously after every Write(). Best-effort — must never panic.
func (s *LinkService) AutoLink(ctx context.Context, newItem Item) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("link.AutoLink: recovered from panic", "panic", fmt.Sprint(r))
		}
	}()

	if newItem.Body == "" {
		return
	}

	embeddings, err := s.embedder.Embed(ctx, []string{newItem.Body})
	if err != nil || len(embeddings) == 0 {
		return
	}

	similar, err := s.searchRepo.SearchByVector(ctx, newItem.WorkspaceID, embeddings[0], 10)
	if err != nil {
		return
	}

	newItemID := newItem.ID.String()
	linksCreated := 0
	for _, candidate := range similar {
		if linksCreated >= MaxAutoLinksPerItem {
			break
		}
		candidateID := candidate.ID.String()
		if candidateID == newItemID {
			continue
		}

		candidateEmbs, err := s.embedder.Embed(ctx, []string{candidate.Body})
		if err != nil || len(candidateEmbs) == 0 {
			continue
		}
		sim := cosineSim32(embeddings[0], candidateEmbs[0])
		if float64(sim) < AutoLinkThreshold {
			continue
		}

		link := MemoryLink{
			ID:          uuid.New().String(),
			WorkspaceID: newItem.WorkspaceID,
			SourceID:    newItemID,
			TargetID:    candidateID,
			LinkType:    LinkTypeAssociative,
			Strength:    float64(sim),
			CreatedAt:   time.Now().UTC(),
			CreatedBy:   "auto",
		}

		if err := s.linkRepo.CreateLink(ctx, link); err != nil {
			if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
				s.logger.Warn("link.AutoLink: create link failed",
					"source", newItemID, "target", candidateID, "error", err)
			}
			continue
		}
		linksCreated++
	}
}

// ExpandWithLinks augments retrieved items with their linked neighbors.
// Linked items are added with a 70% score penalty.
func (s *LinkService) ExpandWithLinks(ctx context.Context, workspaceID string, items []Item, maxLinked int) ([]Item, error) {
	if len(items) == 0 || maxLinked <= 0 {
		return items, nil
	}

	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[item.ID.String()] = true
	}

	expanded := make([]Item, len(items))
	copy(expanded, items)

	addedCount := 0
	for _, item := range items {
		if addedCount >= maxLinked {
			break
		}

		// Use in-memory LinkedItemIDs if already hydrated (avoids DB call).
		var linkedIDs []string
		if len(item.LinkedItemIDs) > 0 {
			linkedIDs = item.LinkedItemIDs
		} else {
			var err error
			linkedIDs, err = s.linkRepo.GetLinkedItemIDs(ctx, workspaceID, item.ID.String())
			if err != nil {
				s.logger.Warn("link.ExpandWithLinks: get linked IDs failed",
					"item_id", item.ID, "error", err)
				continue
			}
		}

		unseenIDs := make([]string, 0, len(linkedIDs))
		for _, id := range linkedIDs {
			if !seen[id] {
				unseenIDs = append(unseenIDs, id)
			}
		}
		if len(unseenIDs) == 0 {
			continue
		}

		linkedItems, err := s.linkRepo.GetItemsByIDs(ctx, workspaceID, unseenIDs)
		if err != nil {
			continue
		}

		for _, linked := range linkedItems {
			if addedCount >= maxLinked {
				break
			}
			lid := linked.ID.String()
			if seen[lid] {
				continue
			}
			seen[lid] = true
			linked.RelevanceScore *= 0.70
			expanded = append(expanded, linked)
			addedCount++
		}
	}

	return expanded, nil
}

func cosineSim32(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(magA) * math.Sqrt(magB)))
}
