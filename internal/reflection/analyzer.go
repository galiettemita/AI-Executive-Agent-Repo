package reflection

import (
	"fmt"
	"sort"
	"strings"
)

var themeKeywords = map[string][]string{
	"email_management":    {"email", "mail", "inbox", "send", "reply", "forward"},
	"calendar_scheduling": {"schedule", "meeting", "calendar", "appointment", "reschedule", "block"},
	"task_management":     {"task", "todo", "remind", "deadline", "priority", "complete"},
	"document_work":       {"document", "draft", "write", "summarize", "report", "create"},
	"communication":       {"slack", "message", "notify", "team", "channel"},
	"research":            {"search", "find", "lookup", "research", "who", "what", "when"},
	"travel_booking":      {"flight", "hotel", "travel", "book", "trip", "conference"},
	"financial":           {"pay", "invoice", "expense", "budget", "transfer", "cost"},
}

// ClusterIntents groups a list of intent events into themes.
func ClusterIntents(events []IntentEvent) []IntentCluster {
	counts := make(map[string][]IntentEvent)
	for _, ev := range events {
		theme := detectTheme(ev.Intent)
		counts[theme] = append(counts[theme], ev)
	}

	clusters := make([]IntentCluster, 0, len(counts))
	for theme, evs := range counts {
		successCount := 0
		intents := make([]string, 0, len(evs))
		for _, ev := range evs {
			intents = append(intents, ev.Intent)
			if ev.Outcome == "success" {
				successCount++
			}
		}
		rate := 0.0
		if len(evs) > 0 {
			rate = float64(successCount) / float64(len(evs))
		}
		clusters = append(clusters, IntentCluster{
			Theme:       theme,
			Count:       len(evs),
			Intents:     intents,
			SuccessRate: rate,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})
	return clusters
}

func detectTheme(intent string) string {
	lower := strings.ToLower(intent)
	bestTheme := "general"
	bestScore := 0
	for theme, keywords := range themeKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestTheme = theme
		}
	}
	return bestTheme
}

// IdentifyFailurePatterns finds tools with fail rate > 20% and at least 2 failures.
func IdentifyFailurePatterns(events []ToolEvent) []ToolFailurePattern {
	type stats struct {
		fails  int
		total  int
		errors map[string]int
	}
	toolStats := make(map[string]*stats)

	for _, ev := range events {
		if toolStats[ev.ToolKey] == nil {
			toolStats[ev.ToolKey] = &stats{errors: make(map[string]int)}
		}
		toolStats[ev.ToolKey].total++
		if !ev.Success {
			toolStats[ev.ToolKey].fails++
			if ev.ErrorCode != "" {
				toolStats[ev.ToolKey].errors[ev.ErrorCode]++
			}
		}
	}

	patterns := []ToolFailurePattern{}
	for toolKey, s := range toolStats {
		if s.total == 0 {
			continue
		}
		failRate := float64(s.fails) / float64(s.total)
		if failRate > 0.2 && s.fails >= 2 {
			errCodes := make([]string, 0, len(s.errors))
			for code := range s.errors {
				errCodes = append(errCodes, code)
			}
			patterns = append(patterns, ToolFailurePattern{
				ToolKey:    toolKey,
				FailCount:  s.fails,
				TotalCount: s.total,
				FailRate:   failRate,
				ErrorCodes: errCodes,
				RootCause:  inferRootCause(toolKey, errCodes),
			})
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].FailRate > patterns[j].FailRate
	})
	return patterns
}

func inferRootCause(toolKey string, errorCodes []string) string {
	for _, code := range errorCodes {
		lower := strings.ToLower(code)
		switch {
		case strings.Contains(lower, "auth") || strings.Contains(lower, "unauthorized"):
			return fmt.Sprintf("OAuth token for %s may be expired or revoked", toolKey)
		case strings.Contains(lower, "rate") || strings.Contains(lower, "limit"):
			return fmt.Sprintf("Rate limit exceeded for %s — consider request batching", toolKey)
		case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
			return fmt.Sprintf("Network latency for %s is high — check connector health", toolKey)
		case strings.Contains(lower, "not_found") || strings.Contains(lower, "404"):
			return fmt.Sprintf("Resource not found via %s — user data may have changed", toolKey)
		}
	}
	return fmt.Sprintf("Repeated failures in %s — check connector status and credentials", toolKey)
}

// GenerateInsights converts clusters and failure patterns into DailyInsight records.
func GenerateInsights(workspaceID, date string, clusters []IntentCluster, failures []ToolFailurePattern, maxInsights int) []DailyInsight {
	if maxInsights <= 0 {
		maxInsights = 10
	}
	insights := []DailyInsight{}

	// Insight: Top intent theme
	if len(clusters) > 0 && clusters[0].Count >= 3 {
		top := clusters[0]
		insights = append(insights, DailyInsight{
			WorkspaceID: workspaceID,
			Date:        date,
			InsightType: "intent_pattern",
			Body: fmt.Sprintf("On %s, the most frequent task theme was '%s' with %d requests (%.0f%% success rate). "+
				"Consider optimizing this workflow path.",
				date, top.Theme, top.Count, top.SuccessRate*100),
			Strength: 0.80,
			Metadata: map[string]any{"theme": top.Theme, "count": top.Count},
		})
	}

	// Insight: Low success themes
	for _, cluster := range clusters {
		if cluster.Count >= 2 && cluster.SuccessRate < 0.5 {
			insights = append(insights, DailyInsight{
				WorkspaceID: workspaceID,
				Date:        date,
				InsightType: "intent_pattern",
				Body: fmt.Sprintf("Low success rate (%.0f%%) for '%s' tasks on %s. "+
					"%d requests in this category failed or required clarification.",
					cluster.SuccessRate*100, cluster.Theme, date, cluster.Count),
				Strength: 0.85,
				Metadata: map[string]any{"theme": cluster.Theme, "success_rate": cluster.SuccessRate},
			})
		}
	}

	// Insight: Tool failure patterns
	for _, fp := range failures {
		insights = append(insights, DailyInsight{
			WorkspaceID: workspaceID,
			Date:        date,
			InsightType: "tool_failure",
			Body: fmt.Sprintf("Tool '%s' failed %d/%d times on %s (%.0f%% failure rate). Root cause hypothesis: %s",
				fp.ToolKey, fp.FailCount, fp.TotalCount, date, fp.FailRate*100, fp.RootCause),
			Strength: 0.90,
			Metadata: map[string]any{
				"tool_key":    fp.ToolKey,
				"fail_rate":   fp.FailRate,
				"error_codes": fp.ErrorCodes,
			},
		})
	}

	if len(insights) > maxInsights {
		insights = insights[:maxInsights]
	}
	return insights
}
