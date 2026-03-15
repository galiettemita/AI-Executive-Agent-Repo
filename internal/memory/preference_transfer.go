package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TransferredPreference is a preference borrowed from another workspace.
type TransferredPreference struct {
	Summary             string
	Category            string
	EffectiveConfidence float64
	IsUniversal         bool
	SourceWorkspaceID   string // internal routing only — never in LLM context
}

// PreferenceTransferIndexEntry is a row in preference_transfer_index.
type PreferenceTransferIndexEntry struct {
	ID                 string
	UserID             string
	SourceWorkspaceID  string
	SourceItemID       string
	PreferenceCategory string
	PreferenceSummary  string
	Embedding          []float32
	Confidence         float64
	IsUniversal        bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// TransferSettings holds workspace transfer opt-in config.
type TransferSettings struct {
	Enabled bool
	Scope   string // "none" | "universal" | "all"
}

// PreferenceTransferRepo is the DB interface for the transfer index.
type PreferenceTransferRepo interface {
	Upsert(ctx context.Context, entry PreferenceTransferIndexEntry) error
	FindForUser(ctx context.Context, userID string, queryVec []float32, limit int) ([]PreferenceTransferIndexEntry, error)
	GetLocalObservationCount(ctx context.Context, targetWorkspaceID, category string) (int, error)
	LogTransfer(ctx context.Context, targetWorkspaceID, sourceWorkspaceID, transferIndexID string, confidence float64) error
}

// WorkspaceTransferSettingsProvider checks transfer eligibility.
type WorkspaceTransferSettingsProvider interface {
	GetTransferSettings(ctx context.Context, workspaceID string) (TransferSettings, error)
	GetOwnerID(ctx context.Context, workspaceID string) (string, error)
}

// PreferenceTransferEmbedder provides embeddings for the transfer index.
type PreferenceTransferEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// PreferenceTransferLogger is the logging interface.
type PreferenceTransferLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// PreferenceTransferService orchestrates cross-workspace preference sharing.
type PreferenceTransferService struct {
	repo     PreferenceTransferRepo
	wsRepo   WorkspaceTransferSettingsProvider
	embedder PreferenceTransferEmbedder
	logger   PreferenceTransferLogger
}

func NewPreferenceTransferService(
	repo PreferenceTransferRepo,
	wsRepo WorkspaceTransferSettingsProvider,
	embedder PreferenceTransferEmbedder,
	logger PreferenceTransferLogger,
) *PreferenceTransferService {
	return &PreferenceTransferService{
		repo:     repo,
		wsRepo:   wsRepo,
		embedder: embedder,
		logger:   logger,
	}
}

// QueryForContext retrieves transferred preferences for the current turn.
// Returns formatted context text or "" if nothing applicable.
func (s *PreferenceTransferService) QueryForContext(
	ctx context.Context,
	targetWorkspaceID string,
	currentTurnText string,
) string {
	prefs, err := s.QueryTransferredPreferences(ctx, targetWorkspaceID, currentTurnText, 5)
	if err != nil {
		s.logger.Warn("preference_transfer: query failed",
			"workspace_id", targetWorkspaceID, "error", err)
		return ""
	}
	return FormatTransferredPreferencesForContext(prefs)
}

// QueryTransferredPreferences retrieves applicable preferences from other workspaces.
func (s *PreferenceTransferService) QueryTransferredPreferences(
	ctx context.Context,
	targetWorkspaceID string,
	queryText string,
	limit int,
) ([]TransferredPreference, error) {
	if limit <= 0 {
		limit = 5
	}

	userID, err := s.wsRepo.GetOwnerID(ctx, targetWorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("preference_transfer: get owner: %w", err)
	}

	settings, err := s.wsRepo.GetTransferSettings(ctx, targetWorkspaceID)
	if err != nil || !settings.Enabled || settings.Scope == "none" {
		return nil, nil
	}

	embeddings, err := s.embedder.Embed(ctx, []string{queryText})
	if err != nil || len(embeddings) == 0 {
		return nil, fmt.Errorf("preference_transfer: embed: %w", err)
	}

	candidates, err := s.repo.FindForUser(ctx, userID, embeddings[0], limit*3)
	if err != nil {
		return nil, fmt.Errorf("preference_transfer: query: %w", err)
	}

	var results []TransferredPreference
	for _, c := range candidates {
		if c.SourceWorkspaceID == targetWorkspaceID {
			continue
		}

		localObs, _ := s.repo.GetLocalObservationCount(ctx, targetWorkspaceID, c.PreferenceCategory)

		initial := InitialTransferConfidence(c.Confidence)
		effective := DecayedTransferConfidence(initial, localObs)

		if effective <= 0.30 {
			continue
		}

		go func(entry PreferenceTransferIndexEntry, eff float64) {
			logCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.repo.LogTransfer(logCtx, targetWorkspaceID, entry.SourceWorkspaceID, entry.ID, eff)
		}(c, effective)

		results = append(results, TransferredPreference{
			Summary:             c.PreferenceSummary,
			Category:            c.PreferenceCategory,
			EffectiveConfidence: effective,
			IsUniversal:         c.IsUniversal,
			SourceWorkspaceID:   c.SourceWorkspaceID,
		})

		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// FormatTransferredPreferencesForContext formats transferred preferences for context injection.
// SourceWorkspaceID is NEVER included in the output.
func FormatTransferredPreferencesForContext(prefs []TransferredPreference) string {
	if len(prefs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Cross-context preference signals]\n")
	for _, p := range prefs {
		confidence := "typically"
		if p.EffectiveConfidence > 0.60 {
			confidence = "consistently"
		}
		b.WriteString(fmt.Sprintf("• User %s: %s\n", confidence, p.Summary))
	}
	return b.String()
}
