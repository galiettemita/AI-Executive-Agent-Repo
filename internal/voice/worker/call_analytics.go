package worker

import (
	"context"
	"time"

	"github.com/brevio/brevio/internal/gateway"
)

// CallAnalytics bundles sentiment analysis results for a completed voice session.
type CallAnalytics struct {
	SessionID       string
	WorkspaceID     string
	TranscriptText  string
	SentimentResult *gateway.CallSentimentResult
	AnalysedAt      time.Time
}

// CallAnalyticsService runs post-session analytics.
type CallAnalyticsService struct {
	analyser gateway.SentimentAnalyser
}

// NewCallAnalyticsService creates a CallAnalyticsService.
func NewCallAnalyticsService(analyser gateway.SentimentAnalyser) *CallAnalyticsService {
	return &CallAnalyticsService{analyser: analyser}
}

// Analyse runs sentiment analysis on a completed session.
// Returns a CallAnalytics struct even on error (with neutral fallback).
func (s *CallAnalyticsService) Analyse(ctx context.Context, sessionID, workspaceID, transcript string) *CallAnalytics {
	result, err := s.analyser.Analyse(ctx, transcript, nil)
	if err != nil || result == nil {
		result = &gateway.CallSentimentResult{
			Overall: gateway.SentimentScore{
				Label: gateway.SentimentNeutral, Score: 0.5, Confidence: 0,
			},
			EscalationSignal: false,
			Summary:          "analysis unavailable",
			AnalysedAt:       time.Now().UTC(),
		}
	}
	return &CallAnalytics{
		SessionID:       sessionID,
		WorkspaceID:     workspaceID,
		TranscriptText:  transcript,
		SentimentResult: result,
		AnalysedAt:      time.Now().UTC(),
	}
}
