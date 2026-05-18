package knowledge

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DriftReport records the staleness analysis of a knowledge file.
type DriftReport struct {
	ID                    string   `json:"id"`
	WorkspaceID           string   `json:"workspace_id"`
	FileType              string   `json:"file_type"` // "persona", "playbook", "faq", "policy"
	StalenessScore        float64  `json:"staleness_score"` // 0.0 (fresh) to 1.0 (very stale)
	LastUpdated           time.Time `json:"last_updated"`
	SuggestedRefreshTopics []string `json:"suggested_refresh_topics"`
	CheckedAt             time.Time `json:"checked_at"`
	Refreshed             bool      `json:"refreshed"`
}

// KnowledgeDriftService detects staleness in knowledge files.
type KnowledgeDriftService struct {
	mu      sync.Mutex
	reports map[string]*DriftReport // key: workspaceID::fileType
	now     func() time.Time
}

// NewKnowledgeDriftService creates a new drift detection service.
func NewKnowledgeDriftService() *KnowledgeDriftService {
	return &KnowledgeDriftService{
		reports: map[string]*DriftReport{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func knowledgeDriftKey(workspaceID, fileType string) string {
	return workspaceID + "::" + fileType
}

// CheckDrift analyses a knowledge file for staleness against recent interactions.
// It returns a DriftReport with a staleness score and suggested refresh topics.
func (s *KnowledgeDriftService) CheckDrift(workspaceID, fileType, content string, recentInteractions []string) (*DriftReport, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if fileType == "" {
		return nil, fmt.Errorf("file_type is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()

	// Check existing report to compute time-based staleness.
	key := knowledgeDriftKey(workspaceID, fileType)
	existing := s.reports[key]

	var lastUpdated time.Time
	if existing != nil && !existing.Refreshed {
		lastUpdated = existing.LastUpdated
	}
	if lastUpdated.IsZero() {
		lastUpdated = now
	}

	// Time-based staleness: increases by 0.01 per day since last update.
	daysSinceUpdate := now.Sub(lastUpdated).Hours() / 24.0
	timeStaleness := daysSinceUpdate * 0.01
	if timeStaleness > 0.5 {
		timeStaleness = 0.5
	}

	// Content-interaction gap: find topics in interactions not covered by content.
	contentLower := strings.ToLower(content)
	var missingTopics []string
	for _, interaction := range recentInteractions {
		words := significantWords(interaction)
		for _, w := range words {
			if !strings.Contains(contentLower, w) {
				missingTopics = append(missingTopics, w)
			}
		}
	}
	missingTopics = dedup(missingTopics)

	gapStaleness := float64(len(missingTopics)) * 0.1
	if gapStaleness > 0.5 {
		gapStaleness = 0.5
	}

	stalenessScore := timeStaleness + gapStaleness
	if stalenessScore > 1.0 {
		stalenessScore = 1.0
	}

	report := &DriftReport{
		ID:                    uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:           workspaceID,
		FileType:              fileType,
		StalenessScore:        stalenessScore,
		LastUpdated:           lastUpdated,
		SuggestedRefreshTopics: missingTopics,
		CheckedAt:             now,
		Refreshed:             false,
	}
	s.reports[key] = report
	return report, nil
}

// significantWords extracts words longer than 4 characters, excluding stop words.
func significantWords(text string) []string {
	stopWords := map[string]bool{
		"about": true, "these": true, "their": true, "there": true,
		"which": true, "would": true, "could": true, "should": true,
		"where": true, "after": true, "before": true,
	}
	words := strings.Fields(strings.ToLower(text))
	var out []string
	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:'\"")
		if len(cleaned) > 4 && !stopWords[cleaned] {
			out = append(out, cleaned)
		}
	}
	return out
}

// dedup removes duplicate strings preserving order.
func dedup(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// GetDriftReports returns all drift reports for a workspace.
func (s *KnowledgeDriftService) GetDriftReports(workspaceID string) []DriftReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []DriftReport
	for _, r := range s.reports {
		if r.WorkspaceID == workspaceID {
			out = append(out, *r)
		}
	}
	return out
}

// MarkRefreshed marks a knowledge file as refreshed, resetting its staleness.
func (s *KnowledgeDriftService) MarkRefreshed(workspaceID, fileType string) error {
	if workspaceID == "" || fileType == "" {
		return fmt.Errorf("workspace_id and file_type are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := knowledgeDriftKey(workspaceID, fileType)
	report, ok := s.reports[key]
	if !ok {
		return fmt.Errorf("no drift report found for %s/%s", workspaceID, fileType)
	}

	report.Refreshed = true
	report.StalenessScore = 0
	report.LastUpdated = s.now()
	report.SuggestedRefreshTopics = nil
	return nil
}
