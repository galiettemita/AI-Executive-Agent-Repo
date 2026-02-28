package capture

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type DailyCapture struct {
	WorkspaceID string   `json:"workspace_id"`
	CaptureDate string   `json:"capture_date"`
	Summary     string   `json:"summary"`
	Wins        []string `json:"wins"`
	Blockers    []string `json:"blockers"`
	NextActions []string `json:"next_actions"`
	Status      string   `json:"status"`
}

type MorningBriefing struct {
	WorkspaceID  string   `json:"workspace_id"`
	BriefingDate string   `json:"briefing_date"`
	Headline     string   `json:"headline"`
	Priorities   []string `json:"priorities"`
	Risks        []string `json:"risks"`
	Agenda       []string `json:"agenda"`
}

type Service struct {
	mu       sync.RWMutex
	captures map[string]map[string]DailyCapture
	logs     map[string]map[string][]string
}

func NewService() *Service {
	return &Service{
		captures: map[string]map[string]DailyCapture{},
		logs:     map[string]map[string][]string{},
	}
}

func (s *Service) Add(capture DailyCapture) DailyCapture {
	s.mu.Lock()
	defer s.mu.Unlock()

	capture.WorkspaceID = normalizeWorkspaceID(capture.WorkspaceID)
	capture.CaptureDate = normalizeCaptureDate(capture.CaptureDate)
	if capture.Status == "" {
		capture.Status = "completed"
	}

	capture.Wins = normalizeStringList(capture.Wins)
	capture.Blockers = normalizeStringList(capture.Blockers)
	capture.NextActions = normalizeStringList(capture.NextActions)
	if strings.TrimSpace(capture.Summary) == "" {
		capture.Summary = defaultSummaryForDate(capture.CaptureDate)
	}

	s.ensureWorkspaceLocked(capture.WorkspaceID)
	s.captures[capture.WorkspaceID][capture.CaptureDate] = capture
	return capture
}

func (s *Service) List(workspaceID string) []DailyCapture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)

	out := make([]DailyCapture, 0, len(s.captures[workspaceID]))
	for _, capture := range s.captures[workspaceID] {
		out = append(out, capture)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CaptureDate < out[j].CaptureDate
	})
	return out
}

func (s *Service) Get(workspaceID, date string) (DailyCapture, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	date = normalizeCaptureDate(date)
	capture, ok := s.captures[workspaceID][date]
	return capture, ok
}

func (s *Service) RecordDailyLog(workspaceID, date, entry string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	date = normalizeCaptureDate(date)
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return len(s.logs[workspaceID][date])
	}

	s.ensureWorkspaceLocked(workspaceID)
	s.logs[workspaceID][date] = append(s.logs[workspaceID][date], entry)
	return len(s.logs[workspaceID][date])
}

// CompleteDailyCapture materializes one capture per workspace/date and is idempotent.
func (s *Service) CompleteDailyCapture(workspaceID, date string) DailyCapture {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	date = normalizeCaptureDate(date)
	s.ensureWorkspaceLocked(workspaceID)

	if capture, ok := s.captures[workspaceID][date]; ok {
		return capture
	}

	logs := append([]string(nil), s.logs[workspaceID][date]...)
	summary, wins, blockers, nextActions := summarizeDailyLogs(logs, date)
	capture := DailyCapture{
		WorkspaceID: workspaceID,
		CaptureDate: date,
		Summary:     summary,
		Wins:        wins,
		Blockers:    blockers,
		NextActions: nextActions,
		Status:      "completed",
	}
	s.captures[workspaceID][date] = capture
	return capture
}

func (s *Service) GenerateMorningBriefing(workspaceID, date string) MorningBriefing {
	capture := s.CompleteDailyCapture(workspaceID, date)
	priorities := append([]string(nil), capture.NextActions...)
	if len(priorities) == 0 {
		priorities = []string{"Review daily priorities"}
	}
	risks := append([]string(nil), capture.Blockers...)
	agenda := append([]string(nil), capture.Wins...)
	if len(agenda) == 0 {
		agenda = []string{"Proceed with planned execution"}
	}
	return MorningBriefing{
		WorkspaceID:  capture.WorkspaceID,
		BriefingDate: capture.CaptureDate,
		Headline:     fmt.Sprintf("Morning briefing for %s", capture.CaptureDate),
		Priorities:   priorities,
		Risks:        risks,
		Agenda:       agenda,
	}
}

func (s *Service) ensureWorkspaceLocked(workspaceID string) {
	if _, ok := s.captures[workspaceID]; !ok {
		s.captures[workspaceID] = map[string]DailyCapture{}
	}
	if _, ok := s.logs[workspaceID]; !ok {
		s.logs[workspaceID] = map[string][]string{}
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeCaptureDate(date string) string {
	if strings.TrimSpace(date) == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	return date
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func defaultSummaryForDate(date string) string {
	return fmt.Sprintf("Daily introspection completed for %s", date)
}

func summarizeDailyLogs(logs []string, date string) (string, []string, []string, []string) {
	wins := []string{}
	blockers := []string{}
	nextActions := []string{}
	notes := []string{}
	for _, raw := range logs {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "win:"):
			wins = append(wins, strings.TrimSpace(line[len("win:"):]))
		case strings.HasPrefix(lower, "blocker:"):
			blockers = append(blockers, strings.TrimSpace(line[len("blocker:"):]))
		case strings.HasPrefix(lower, "next:"):
			nextActions = append(nextActions, strings.TrimSpace(line[len("next:"):]))
		default:
			notes = append(notes, line)
		}
	}

	wins = normalizeStringList(wins)
	blockers = normalizeStringList(blockers)
	nextActions = normalizeStringList(nextActions)
	if len(notes) == 0 && len(wins) == 0 && len(blockers) == 0 && len(nextActions) == 0 {
		return defaultSummaryForDate(date), wins, blockers, nextActions
	}

	parts := []string{}
	if len(notes) > 0 {
		if len(notes) > 2 {
			notes = notes[:2]
		}
		parts = append(parts, strings.Join(notes, "; "))
	}
	if len(wins) > 0 {
		parts = append(parts, fmt.Sprintf("%d wins", len(wins)))
	}
	if len(blockers) > 0 {
		parts = append(parts, fmt.Sprintf("%d blockers", len(blockers)))
	}
	if len(nextActions) > 0 {
		parts = append(parts, fmt.Sprintf("%d next actions", len(nextActions)))
	}
	if len(parts) == 0 {
		return defaultSummaryForDate(date), wins, blockers, nextActions
	}
	return strings.Join(parts, " | "), wins, blockers, nextActions
}
