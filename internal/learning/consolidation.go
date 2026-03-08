package learning

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// CorrectionCluster groups related corrections for potential rule consolidation.
type CorrectionCluster struct {
	ID           string   `json:"id"`
	Topic        string   `json:"topic"`
	Corrections  []string `json:"corrections"`
	Count        int      `json:"count"`
	ProposedRule string   `json:"proposed_rule"`
}

// RuleProposal represents a proposed rule derived from correction clustering.
type RuleProposal struct {
	ClusterID string `json:"cluster_id"`
	Rule      string `json:"rule"`
	Status    string `json:"status"` // "pending", "confirmed", "rejected"
}

// LessonConsolidationService clusters corrections and proposes rules.
type LessonConsolidationService struct {
	mu        sync.Mutex
	clusters  map[string]*CorrectionCluster
	proposals map[string]*RuleProposal
	// corrections maps workspaceID -> list of correction strings
	corrections map[string][]string
}

// NewLessonConsolidationService creates a new consolidation service.
func NewLessonConsolidationService() *LessonConsolidationService {
	return &LessonConsolidationService{
		clusters:    map[string]*CorrectionCluster{},
		proposals:   map[string]*RuleProposal{},
		corrections: map[string][]string{},
	}
}

// AddCorrection adds a correction for a workspace.
func (s *LessonConsolidationService) AddCorrection(workspaceID, correction string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(correction) == "" {
		return fmt.Errorf("correction is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.corrections[workspaceID] = append(s.corrections[workspaceID], correction)
	return nil
}

// ClusterCorrections groups corrections for a workspace by shared keywords.
// Returns the list of generated clusters.
func (s *LessonConsolidationService) ClusterCorrections(workspaceID string) ([]CorrectionCluster, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	corrs := s.corrections[workspaceID]
	if len(corrs) == 0 {
		return nil, nil
	}

	// Simple keyword-based clustering.
	topicMap := map[string][]string{}
	for _, c := range corrs {
		topic := extractTopic(c)
		topicMap[topic] = append(topicMap[topic], c)
	}

	var clusters []CorrectionCluster
	for topic, items := range topicMap {
		if len(items) < 2 {
			continue // only cluster if there are at least 2 corrections
		}
		cluster := CorrectionCluster{
			ID:          uuid.Must(uuid.NewV7()).String(),
			Topic:       topic,
			Corrections: items,
			Count:       len(items),
		}
		s.clusters[cluster.ID] = &cluster
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

// extractTopic derives a simple topic from a correction string by taking the
// first significant word (>3 chars).
func extractTopic(correction string) string {
	words := strings.Fields(strings.ToLower(correction))
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "in": true,
		"to": true, "of": true, "and": true, "for": true, "from": true,
		"should": true, "not": true, "don't": true, "always": true,
		"never": true, "use": true, "when": true, "with": true,
	}
	for _, w := range words {
		if len(w) > 3 && !stopWords[w] {
			return w
		}
	}
	if len(words) > 0 {
		return words[0]
	}
	return "general"
}

// ProposeRules generates a rule proposal for a cluster.
func (s *LessonConsolidationService) ProposeRules(cluster CorrectionCluster) (*RuleProposal, error) {
	if cluster.ID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	if cluster.Count < 2 {
		return nil, fmt.Errorf("cluster must have at least 2 corrections")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a proposed rule from the cluster topic and count.
	rule := fmt.Sprintf("When handling %q-related tasks, apply: %s (based on %d corrections)",
		cluster.Topic, summarizeCorrections(cluster.Corrections), cluster.Count)

	cluster.ProposedRule = rule
	s.clusters[cluster.ID] = &cluster

	proposal := &RuleProposal{
		ClusterID: cluster.ID,
		Rule:      rule,
		Status:    "pending",
	}
	s.proposals[cluster.ID] = proposal
	return proposal, nil
}

// summarizeCorrections creates a brief summary of the corrections.
func summarizeCorrections(corrections []string) string {
	if len(corrections) == 0 {
		return ""
	}
	if len(corrections) == 1 {
		return corrections[0]
	}
	summary := corrections[0]
	if len(summary) > 60 {
		summary = summary[:60] + "..."
	}
	return fmt.Sprintf("%s (and %d more)", summary, len(corrections)-1)
}

// GetPendingProposals returns all proposals with status "pending".
func (s *LessonConsolidationService) GetPendingProposals() []RuleProposal {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []RuleProposal
	for _, p := range s.proposals {
		if p.Status == "pending" {
			out = append(out, *p)
		}
	}
	return out
}

// ConfirmProposal marks a proposal as confirmed.
func (s *LessonConsolidationService) ConfirmProposal(clusterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.proposals[clusterID]
	if !ok {
		return fmt.Errorf("proposal not found for cluster %q", clusterID)
	}
	p.Status = "confirmed"
	return nil
}

// RejectProposal marks a proposal as rejected.
func (s *LessonConsolidationService) RejectProposal(clusterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.proposals[clusterID]
	if !ok {
		return fmt.Errorf("proposal not found for cluster %q", clusterID)
	}
	p.Status = "rejected"
	return nil
}
