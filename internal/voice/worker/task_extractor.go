package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

// ── Legacy types (backward compat) ─────────────────────────────────────────

// ExtractedTask is the legacy output type (used by KeywordTaskExtractor).
type ExtractedTask struct {
	Description     string
	AssignedTo      string
	DueDate         string
	Priority        int // 1=high, 2=medium, 3=low
	SourceTurnIndex int
}

// ToStructuredTask converts a legacy task to the rich type.
func (e ExtractedTask) ToStructuredTask() StructuredTask {
	pm := map[int]string{1: "high", 2: "medium", 3: "low"}
	p, ok := pm[e.Priority]
	if !ok {
		p = "low"
	}
	return StructuredTask{
		Description: e.Description,
		Assignee:    e.AssignedTo,
		DueDate:     e.DueDate,
		Priority:    p,
		Speaker:     e.AssignedTo,
		Confidence:  0.5,
	}
}

// ── Rich types ───────────────────────────────────────────────────────────────

// StructuredTask is the output of LLM-based extraction.
type StructuredTask struct {
	Description string  `json:"description"`
	Assignee    string  `json:"assignee"`
	DueDate     string  `json:"due_date"`   // ISO-8601 YYYY-MM-DD or ""
	Priority    string  `json:"priority"`   // "high" | "medium" | "low"
	Speaker     string  `json:"speaker"`
	Confidence  float64 `json:"confidence"` // 0.0–1.0
}

// TaskExtractionResult is the top-level LLM JSON response.
type TaskExtractionResult struct {
	Tasks   []StructuredTask `json:"tasks"`
	Summary string           `json:"summary"`
}

// ── KeywordTaskExtractor ─────────────────────────────────────────────────────

// KeywordTaskExtractor extracts tasks via string pattern matching (legacy/fallback).
type KeywordTaskExtractor struct {
	patterns []string
}

// TaskExtractor is a backward-compatible alias for KeywordTaskExtractor.
type TaskExtractor = KeywordTaskExtractor

// NewKeywordTaskExtractor creates a KeywordTaskExtractor.
func NewKeywordTaskExtractor() *KeywordTaskExtractor {
	return &KeywordTaskExtractor{
		patterns: []string{
			"i need to", "remind me to", "schedule", "book", "call",
			"follow up", "send", "set up", "arrange", "make sure to",
			"don't forget to", "we should", "please", "action item",
			"todo", "to do",
		},
	}
}

// NewTaskExtractor is a backward-compatible alias for NewKeywordTaskExtractor.
func NewTaskExtractor() *KeywordTaskExtractor {
	return NewKeywordTaskExtractor()
}

// ExtractTasks returns a legacy ExtractedTask slice.
func (k *KeywordTaskExtractor) ExtractTasks(transcript []TranscriptTurn) []ExtractedTask {
	var tasks []ExtractedTask
	for i, turn := range transcript {
		lower := strings.ToLower(turn.Text)
		for _, p := range k.patterns {
			if strings.Contains(lower, p) {
				tasks = append(tasks, ExtractedTask{
					Description:     turn.Text,
					AssignedTo:      turn.Speaker,
					Priority:        k.inferPriority(lower),
					SourceTurnIndex: i,
					DueDate:         k.extractDueDate(lower),
				})
				break
			}
		}
	}
	return tasks
}

func (k *KeywordTaskExtractor) inferPriority(text string) int {
	for _, w := range []string{"urgent", "asap", "immediately", "critical", "emergency"} {
		if strings.Contains(text, w) {
			return 1
		}
	}
	for _, w := range []string{"today", "soon", "this week", "tomorrow"} {
		if strings.Contains(text, w) {
			return 2
		}
	}
	return 3
}

func (k *KeywordTaskExtractor) extractDueDate(text string) string {
	dates := map[string]string{
		"today": "today", "tomorrow": "tomorrow", "next week": "next week",
		"this week": "this week", "end of day": "end of day",
		"monday": "monday", "tuesday": "tuesday", "wednesday": "wednesday",
		"thursday": "thursday", "friday": "friday",
	}
	for keyword, date := range dates {
		if strings.Contains(text, keyword) {
			return date
		}
	}
	return ""
}

// ── LLMTaskExtractor ─────────────────────────────────────────────────────────

const llmTaskExtractionSystem = `You are a precise action-item extractor for an AI executive assistant.

Given a meeting or voice call transcript, extract all genuine action items.

RULES:
- Only GENUINE action items (commitments, tasks, follow-ups) — not filler like "please hold"
- Normalise due dates to ISO-8601 (YYYY-MM-DD); use "" if no date mentioned
- Priority: "high" = urgent/ASAP, "medium" = this week/soon, "low" = no urgency
- Assignee: person responsible, or "" if unclear
- Confidence: 0.0–1.0 — how certain this is a real action item
- Return ONLY valid JSON, no markdown fences

Response schema:
{
  "tasks": [
    {
      "description": "string (concise imperative)",
      "assignee": "string or ''",
      "due_date": "YYYY-MM-DD or ''",
      "priority": "high|medium|low",
      "speaker": "string",
      "confidence": 0.0-1.0
    }
  ],
  "summary": "string (1-sentence meeting summary)"
}`

// LLMTaskExtractorConfig configures the LLM-backed extractor.
type LLMTaskExtractorConfig struct {
	LLMClient      llm.Client
	TodayDate      string        // YYYY-MM-DD; defaults to today UTC
	MaxTasks       int           // default 20
	Timeout        time.Duration // default 30s
	FallbackOnFail bool          // fall back to keyword extractor on LLM error
	MinConfidence  float64       // filter below this score; default 0.6
}

// LLMTaskExtractor extracts structured tasks from transcripts via LLM.
type LLMTaskExtractor struct {
	cfg      LLMTaskExtractorConfig
	fallback *KeywordTaskExtractor
}

// NewLLMTaskExtractor creates an LLMTaskExtractor.
func NewLLMTaskExtractor(cfg LLMTaskExtractorConfig) (*LLMTaskExtractor, error) {
	if cfg.LLMClient == nil {
		return nil, fmt.Errorf("llm task extractor: LLMClient is required")
	}
	if cfg.MaxTasks <= 0 {
		cfg.MaxTasks = 20
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.6
	}
	if cfg.TodayDate == "" {
		cfg.TodayDate = time.Now().UTC().Format("2006-01-02")
	}
	return &LLMTaskExtractor{cfg: cfg, fallback: NewKeywordTaskExtractor()}, nil
}

// Extract extracts structured tasks from a flat transcript string.
func (l *LLMTaskExtractor) Extract(ctx context.Context, transcript string) (*TaskExtractionResult, error) {
	if strings.TrimSpace(transcript) == "" {
		return &TaskExtractionResult{Tasks: []StructuredTask{}}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, l.cfg.Timeout)
	defer cancel()

	userMsg := fmt.Sprintf("Today's date: %s\n\nTranscript:\n%s", l.cfg.TodayDate, transcript)

	req := llm.GenerateRequest{
		System:    llmTaskExtractionSystem,
		Messages:  []llm.ChatMsg{{Role: "user", Content: userMsg}},
		MaxTokens: 2048,
	}

	resp, _, err := l.cfg.LLMClient.Generate(ctx, req)
	if err != nil {
		if l.cfg.FallbackOnFail {
			return l.doFallback(transcript), nil
		}
		return nil, fmt.Errorf("llm task extractor: LLM call failed: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result TaskExtractionResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		if l.cfg.FallbackOnFail {
			return l.doFallback(transcript), nil
		}
		preview := content
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf("llm task extractor: unmarshal response: %w (raw: %s)", err, preview)
	}

	filtered := make([]StructuredTask, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		if t.Confidence >= l.cfg.MinConfidence {
			filtered = append(filtered, sanitiseTask(t))
		}
	}
	if len(filtered) > l.cfg.MaxTasks {
		filtered = filtered[:l.cfg.MaxTasks]
	}
	result.Tasks = filtered
	return &result, nil
}

// ExtractFromTurns converts TranscriptTurns to a labelled string and calls Extract.
// Returns legacy ExtractedTask slice for backward compat.
func (l *LLMTaskExtractor) ExtractFromTurns(ctx context.Context, turns []TranscriptTurn) ([]ExtractedTask, error) {
	result, err := l.Extract(ctx, turnsToTranscript(turns))
	if err != nil {
		return nil, err
	}
	legacy := make([]ExtractedTask, 0, len(result.Tasks))
	for i, t := range result.Tasks {
		legacy = append(legacy, ExtractedTask{
			Description:     t.Description,
			AssignedTo:      t.Assignee,
			DueDate:         t.DueDate,
			Priority:        priorityStringToInt(t.Priority),
			SourceTurnIndex: i,
		})
	}
	return legacy, nil
}

func (l *LLMTaskExtractor) doFallback(transcript string) *TaskExtractionResult {
	// Split into sentences for per-sentence matching (same as activity layer).
	sentences := strings.Split(transcript, ".")
	turns := make([]TranscriptTurn, 0, len(sentences))
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			turns = append(turns, TranscriptTurn{Speaker: "user", Text: s})
		}
	}
	extracted := l.fallback.ExtractTasks(turns)
	tasks := make([]StructuredTask, 0, len(extracted))
	for _, t := range extracted {
		tasks = append(tasks, t.ToStructuredTask())
	}
	return &TaskExtractionResult{Tasks: tasks}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func sanitiseTask(t StructuredTask) StructuredTask {
	t.Description = strings.TrimSpace(t.Description)
	t.Assignee = strings.TrimSpace(t.Assignee)
	t.DueDate = strings.TrimSpace(t.DueDate)
	if t.DueDate != "" {
		if _, err := time.Parse("2006-01-02", t.DueDate); err != nil {
			t.DueDate = ""
		}
	}
	switch strings.ToLower(t.Priority) {
	case "high":
		t.Priority = "high"
	case "medium":
		t.Priority = "medium"
	default:
		t.Priority = "low"
	}
	if t.Confidence < 0 {
		t.Confidence = 0
	}
	if t.Confidence > 1 {
		t.Confidence = 1
	}
	return t
}

func turnsToTranscript(turns []TranscriptTurn) string {
	var sb strings.Builder
	for _, t := range turns {
		if t.Speaker != "" {
			sb.WriteString(t.Speaker)
			sb.WriteString(": ")
		}
		sb.WriteString(t.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}

func priorityStringToInt(p string) int {
	switch p {
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}
