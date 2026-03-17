package preference

import (
	"context"

	"github.com/brevio/brevio/internal/memory"
)

// Retriever fetches preference facts from long-term memory for a workspace+user.
type Retriever struct {
	memorySvc *memory.Service
}

// NewRetriever creates a Retriever backed by the given memory service.
func NewRetriever(memorySvc *memory.Service) *Retriever {
	return &Retriever{memorySvc: memorySvc}
}

// FetchTopK retrieves the top K most relevant preference facts.
func (r *Retriever) FetchTopK(_ context.Context, workspaceID, userID, intent string, topK int) ([]PreferenceFact, error) {
	if topK <= 0 {
		topK = 5
	}
	if r.memorySvc == nil {
		return nil, nil
	}

	items, err := r.memorySvc.SearchByType(workspaceID, userID, "preference", topK)
	if err != nil {
		return nil, err
	}

	facts := make([]PreferenceFact, 0, len(items))
	for _, item := range items {
		facts = append(facts, PreferenceFact{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Category:    "general_preference",
			Preference:  item.Body,
			Confidence:  item.Confidence,
			EvidenceID:  item.ID.String(),
		})
	}
	return facts, nil
}
