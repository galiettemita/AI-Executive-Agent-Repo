package learning

import (
	"fmt"
	"strings"
	"sync"
)

// LessonConflict describes a detected conflict between two lessons.
type LessonConflict struct {
	ID           string  `json:"id"`
	LessonA      string  `json:"lesson_a"`
	LessonB      string  `json:"lesson_b"`
	ConflictType string  `json:"conflict_type"` // contradictory, redundant, superseded
	Confidence   float64 `json:"confidence"`
	Description  string  `json:"description"`
	Resolved     bool    `json:"resolved"`
	Resolution   string  `json:"resolution"` // keep_a, keep_b, merge, retire_both
}

// ConflictDetector detects and resolves conflicts between lessons.
type ConflictDetector struct {
	mu        sync.RWMutex
	nextID    int
	conflicts map[string]*LessonConflict
}

// NewConflictDetector creates a new conflict detector.
func NewConflictDetector() *ConflictDetector {
	return &ConflictDetector{
		nextID:    1,
		conflicts: map[string]*LessonConflict{},
	}
}

// DetectConflicts analyses a set of lessons for conflicts.
// It checks for redundancy (similar titles) and supersession (same workspace,
// differing statuses).
func (d *ConflictDetector) DetectConflicts(lessons []Lesson) []LessonConflict {
	d.mu.Lock()
	defer d.mu.Unlock()

	var detected []LessonConflict

	for i := 0; i < len(lessons); i++ {
		for j := i + 1; j < len(lessons); j++ {
			a := lessons[i]
			b := lessons[j]

			// Redundancy: very similar titles
			if similarTitles(a.Title, b.Title) {
				conflict := LessonConflict{
					ID:           fmt.Sprintf("conflict_%06d", d.nextID),
					LessonA:      a.ID,
					LessonB:      b.ID,
					ConflictType: "redundant",
					Confidence:   computeTitleSimilarity(a.Title, b.Title),
					Description:  fmt.Sprintf("lessons %q and %q have similar titles", a.ID, b.ID),
				}
				d.nextID++
				d.conflicts[conflict.ID] = &conflict
				detected = append(detected, conflict)
				continue
			}

			// Supersession: same workspace, one confirmed and one proposed
			if a.WorkspaceID == b.WorkspaceID {
				if (a.Status == "confirmed" && b.Status == "proposed") ||
					(a.Status == "proposed" && b.Status == "confirmed") {
					if relatedContent(a.Title, b.Title) {
						conflictType := "superseded"
						conflict := LessonConflict{
							ID:           fmt.Sprintf("conflict_%06d", d.nextID),
							LessonA:      a.ID,
							LessonB:      b.ID,
							ConflictType: conflictType,
							Confidence:   0.7,
							Description:  fmt.Sprintf("lesson %q may supersede %q in workspace %q", a.ID, b.ID, a.WorkspaceID),
						}
						d.nextID++
						d.conflicts[conflict.ID] = &conflict
						detected = append(detected, conflict)
						continue
					}
				}

				// Contradictory: same workspace, both confirmed, overlapping words
				if a.Status == "confirmed" && b.Status == "confirmed" && relatedContent(a.Title, b.Title) {
					conflict := LessonConflict{
						ID:           fmt.Sprintf("conflict_%06d", d.nextID),
						LessonA:      a.ID,
						LessonB:      b.ID,
						ConflictType: "contradictory",
						Confidence:   0.6,
						Description:  fmt.Sprintf("confirmed lessons %q and %q may contradict each other", a.ID, b.ID),
					}
					d.nextID++
					d.conflicts[conflict.ID] = &conflict
					detected = append(detected, conflict)
				}
			}
		}
	}

	return detected
}

// ResolveConflict applies a resolution to a detected conflict.
func (d *ConflictDetector) ResolveConflict(conflictID string, resolution string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	conflict, ok := d.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("conflict %q not found", conflictID)
	}

	validResolutions := map[string]bool{
		"keep_a":      true,
		"keep_b":      true,
		"merge":       true,
		"retire_both": true,
	}
	if !validResolutions[resolution] {
		return fmt.Errorf("invalid resolution %q; must be one of: keep_a, keep_b, merge, retire_both", resolution)
	}

	conflict.Resolved = true
	conflict.Resolution = resolution
	return nil
}

// GetConflict retrieves a specific conflict by ID.
func (d *ConflictDetector) GetConflict(conflictID string) (*LessonConflict, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.conflicts[conflictID]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

// similarTitles returns true if two titles share more than 60% of their words.
func similarTitles(a, b string) bool {
	return computeTitleSimilarity(a, b) > 0.6
}

// computeTitleSimilarity computes Jaccard similarity of words in two titles.
func computeTitleSimilarity(a, b string) float64 {
	wordsA := wordSet(a)
	wordsB := wordSet(b)

	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0
	}

	intersection := 0
	for w := range wordsA {
		if _, ok := wordsB[w]; ok {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if _, ok := wordsA[w]; !ok {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// relatedContent checks if two titles share at least one meaningful word.
func relatedContent(a, b string) bool {
	wordsA := wordSet(a)
	wordsB := wordSet(b)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "in": true,
		"to": true, "of": true, "and": true, "for": true, "from": true,
		"lesson:": true, "lesson": true,
	}

	for w := range wordsA {
		if stopWords[w] {
			continue
		}
		if _, ok := wordsB[w]; ok {
			return true
		}
	}
	return false
}

func wordSet(s string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(s))
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}
