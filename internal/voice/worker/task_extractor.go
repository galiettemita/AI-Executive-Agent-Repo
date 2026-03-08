package worker

import (
	"strings"
)

// ExtractedTask represents a task extracted from a voice transcript.
type ExtractedTask struct {
	Description     string
	AssignedTo      string
	DueDate         string
	Priority        int
	SourceTurnIndex int
}

// TaskExtractor extracts action items from voice session transcripts.
type TaskExtractor struct {
	patterns []string
}

// NewTaskExtractor creates a new TaskExtractor with default action item patterns.
func NewTaskExtractor() *TaskExtractor {
	return &TaskExtractor{
		patterns: []string{
			"i need to",
			"remind me to",
			"schedule",
			"book",
			"call",
			"follow up",
			"send",
			"set up",
			"arrange",
			"make sure to",
			"don't forget to",
			"we should",
			"please",
			"action item",
			"todo",
			"to do",
		},
	}
}

// ExtractTasks scans transcript turns for action items using pattern matching.
func (te *TaskExtractor) ExtractTasks(transcript []TranscriptTurn) []ExtractedTask {
	var tasks []ExtractedTask

	for i, turn := range transcript {
		lower := strings.ToLower(turn.Text)

		for _, pattern := range te.patterns {
			if strings.Contains(lower, pattern) {
				task := ExtractedTask{
					Description:     turn.Text,
					AssignedTo:      turn.Speaker,
					Priority:        te.inferPriority(lower),
					SourceTurnIndex: i,
					DueDate:         te.extractDueDate(lower),
				}
				tasks = append(tasks, task)
				break // One task per turn.
			}
		}
	}

	return tasks
}

// inferPriority assigns a priority based on urgency keywords.
func (te *TaskExtractor) inferPriority(text string) int {
	urgentWords := []string{"urgent", "asap", "immediately", "critical", "emergency"}
	for _, w := range urgentWords {
		if strings.Contains(text, w) {
			return 1 // High priority.
		}
	}

	soonWords := []string{"today", "soon", "this week", "tomorrow"}
	for _, w := range soonWords {
		if strings.Contains(text, w) {
			return 2 // Medium priority.
		}
	}

	return 3 // Normal priority.
}

// extractDueDate attempts to extract a due date from text.
func (te *TaskExtractor) extractDueDate(text string) string {
	dateKeywords := map[string]string{
		"today":          "today",
		"tomorrow":       "tomorrow",
		"next week":      "next week",
		"this week":      "this week",
		"end of day":     "end of day",
		"end of week":    "end of week",
		"monday":         "monday",
		"tuesday":        "tuesday",
		"wednesday":      "wednesday",
		"thursday":       "thursday",
		"friday":         "friday",
	}

	for keyword, date := range dateKeywords {
		if strings.Contains(text, keyword) {
			return date
		}
	}

	return ""
}
