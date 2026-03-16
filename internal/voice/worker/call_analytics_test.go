package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/brevio/brevio/internal/gateway"
	"github.com/stretchr/testify/assert"
)

type mockSentimentAnalyser struct {
	result *gateway.CallSentimentResult
	err    error
}

func (m *mockSentimentAnalyser) Name() string { return "mock" }
func (m *mockSentimentAnalyser) Analyse(_ context.Context, _ string, _ []gateway.SpeakerTurn) (*gateway.CallSentimentResult, error) {
	return m.result, m.err
}

func TestCallAnalyticsService_HappyPath(t *testing.T) {
	r := &gateway.CallSentimentResult{
		Overall: gateway.SentimentScore{Label: gateway.SentimentPositive, Score: 0.8},
	}
	svc := NewCallAnalyticsService(&mockSentimentAnalyser{result: r})
	out := svc.Analyse(context.Background(), "s1", "ws1", "great meeting")
	assert.Equal(t, gateway.SentimentPositive, out.SentimentResult.Overall.Label)
	assert.False(t, out.AnalysedAt.IsZero())
}

func TestCallAnalyticsService_AnalyserError(t *testing.T) {
	svc := NewCallAnalyticsService(&mockSentimentAnalyser{err: errors.New("API down")})
	out := svc.Analyse(context.Background(), "s2", "ws2", "some transcript")
	assert.Equal(t, gateway.SentimentNeutral, out.SentimentResult.Overall.Label)
}

func TestCallAnalyticsService_EscalationPropagated(t *testing.T) {
	r := &gateway.CallSentimentResult{EscalationSignal: true, Summary: "angry caller"}
	svc := NewCallAnalyticsService(&mockSentimentAnalyser{result: r})
	out := svc.Analyse(context.Background(), "s3", "ws3", "this is unacceptable")
	assert.True(t, out.SentimentResult.EscalationSignal)
}

func TestCallAnalyticsService_EmptyTranscript(t *testing.T) {
	svc := NewCallAnalyticsService(&mockSentimentAnalyser{result: &gateway.CallSentimentResult{Summary: "empty"}})
	out := svc.Analyse(context.Background(), "s4", "ws4", "")
	assert.NotNil(t, out)
}
