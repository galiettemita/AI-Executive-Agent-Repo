package proactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SignalType identifies a proactive signal category.
type SignalType string

const (
	SignalCalendarConflict    SignalType = "calendar_conflict"
	SignalDeadlineApproaching SignalType = "deadline_approaching"
	SignalEmailUrgency        SignalType = "email_urgency"
	SignalTravelConflict      SignalType = "travel_conflict"
)

// Signal is a detected proactive opportunity.
type Signal struct {
	Type        SignalType      `json:"type"`
	WorkspaceID string         `json:"workspace_id"`
	Data        map[string]any `json:"data"`
	Urgency     float64        `json:"urgency"`
	DetectedAt  time.Time      `json:"detected_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// CalendarEvent is a minimal calendar event for conflict detection.
type CalendarEvent struct {
	ID        string
	Title     string
	StartTime time.Time
	EndTime   time.Time
	Location  string
}

// EmailSignal is a minimal email for urgency detection.
type EmailSignal struct {
	Subject    string
	Sender     string
	ReceivedAt time.Time
	Snippet    string
}

// CalendarFetcher retrieves upcoming calendar events for a workspace.
type CalendarFetcher interface {
	FetchUpcoming(ctx context.Context, workspaceID string, horizon time.Duration) ([]CalendarEvent, error)
}

// EmailFetcher retrieves recent unread emails for a workspace.
type EmailFetcher interface {
	FetchRecent(ctx context.Context, workspaceID string, since time.Duration) ([]EmailSignal, error)
}

// ProactiveMonitor checks for proactive opportunities.
type ProactiveMonitor struct {
	pool            *pgxpool.Pool
	snoozeStore     *SnoozeStore
	calendarFetcher CalendarFetcher
	emailFetcher    EmailFetcher
}

// NewProactiveMonitor creates a ProactiveMonitor.
func NewProactiveMonitor(pool *pgxpool.Pool, snoozeStore *SnoozeStore, calendarFetcher CalendarFetcher, emailFetcher EmailFetcher) *ProactiveMonitor {
	return &ProactiveMonitor{pool: pool, snoozeStore: snoozeStore, calendarFetcher: calendarFetcher, emailFetcher: emailFetcher}
}

// DetectSignals runs all signal checkers for a workspace and returns detected signals.
func (m *ProactiveMonitor) DetectSignals(ctx context.Context, workspaceID string) ([]Signal, error) {
	if m.snoozeStore != nil {
		if snoozed, _ := m.snoozeStore.IsSnoozed(ctx, workspaceID, ""); snoozed {
			return nil, nil
		}
	}

	var signals []Signal

	calSignals, err := m.detectCalendarConflicts(ctx, workspaceID)
	if err == nil {
		signals = append(signals, calSignals...)
	}

	emailSignals, err := m.detectEmailUrgency(ctx, workspaceID)
	if err == nil {
		signals = append(signals, emailSignals...)
	}

	var filtered []Signal
	for _, s := range signals {
		if s.Urgency < 0.5 {
			continue
		}
		if m.snoozeStore != nil {
			if typeSnoozed, _ := m.snoozeStore.IsSnoozed(ctx, workspaceID, string(s.Type)); typeSnoozed {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	return filtered, nil
}

func (m *ProactiveMonitor) detectCalendarConflicts(ctx context.Context, workspaceID string) ([]Signal, error) {
	if m.calendarFetcher == nil {
		return nil, nil
	}
	events, err := m.calendarFetcher.FetchUpcoming(ctx, workspaceID, 4*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("detect_calendar_conflicts: %w", err)
	}

	var signals []Signal
	for i := 0; i < len(events)-1; i++ {
		a, b := events[i], events[i+1]
		if a.EndTime.After(b.StartTime) {
			overlapMinutes := a.EndTime.Sub(b.StartTime).Minutes()
			urgency := 0.6
			if overlapMinutes > 15 {
				urgency = 0.9
			}
			signals = append(signals, Signal{
				Type:        SignalCalendarConflict,
				WorkspaceID: workspaceID,
				Data: map[string]any{
					"event_a_title":   a.Title,
					"event_b_title":   b.Title,
					"overlap_minutes": overlapMinutes,
				},
				Urgency:    urgency,
				DetectedAt: time.Now().UTC(),
				ExpiresAt:  b.StartTime,
			})
		}
	}
	return signals, nil
}

func (m *ProactiveMonitor) detectEmailUrgency(ctx context.Context, workspaceID string) ([]Signal, error) {
	if m.emailFetcher == nil {
		return nil, nil
	}
	emails, err := m.emailFetcher.FetchRecent(ctx, workspaceID, 30*time.Minute)
	if err != nil {
		return nil, err
	}

	urgentKeywords := []string{
		"urgent", "asap", "immediately", "action required", "deadline today",
		"time sensitive", "by eod", "critical", "board", "investor", "due today",
	}

	var signals []Signal
	for _, email := range emails {
		combined := strings.ToLower(email.Subject + " " + email.Snippet)
		for _, kw := range urgentKeywords {
			if strings.Contains(combined, kw) {
				snippet := email.Snippet
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				signals = append(signals, Signal{
					Type:        SignalEmailUrgency,
					WorkspaceID: workspaceID,
					Data: map[string]any{
						"sender":  email.Sender,
						"subject": email.Subject,
						"snippet": snippet,
						"keyword": kw,
					},
					Urgency:    0.75,
					DetectedAt: time.Now().UTC(),
					ExpiresAt:  time.Now().UTC().Add(2 * time.Hour),
				})
				break
			}
		}
	}
	return signals, nil
}

// PersistSignal saves a signal record to the DB and returns its ID.
func (m *ProactiveMonitor) PersistSignal(ctx context.Context, s Signal, offerText string) (string, error) {
	if m.pool == nil {
		return "", nil
	}
	dataBytes, _ := json.Marshal(s.Data)
	var id string
	err := m.pool.QueryRow(ctx, `
		INSERT INTO proactive_signals
			(workspace_id, signal_type, signal_data, offer_text, status, expires_at)
		VALUES ($1, $2, $3::jsonb, $4, 'pending', $5)
		RETURNING id
	`, s.WorkspaceID, string(s.Type), string(dataBytes), offerText, s.ExpiresAt).Scan(&id)
	return id, err
}
