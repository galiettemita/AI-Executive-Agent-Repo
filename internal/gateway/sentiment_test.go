package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSentimentLLM struct {
	content string
	err     error
}

func (m *mockSentimentLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.content}, &llm.Usage{}, nil
}
func (m *mockSentimentLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
	out <- llm.StreamChunk{Delta: m.content, Done: true}
}

var goodSentimentJSON = `{
  "overall":{"label":"positive","score":0.8,"positive":0.7,"negative":0.1,"neutral":0.2,"confidence":0.9},
  "per_speaker":[{
    "speaker":"Speaker 1",
    "sentiment":{"label":"positive","score":0.8,"positive":0.7,"negative":0.1,"neutral":0.2,"confidence":0.9},
    "emotion":{"primary":"satisfaction","scores":{"joy":0.3,"satisfaction":0.6,"frustration":0.1,"anger":0.0,"anxiety":0.0,"neutral":0.0},"confidence":0.85}
  }],
  "escalation_signal":false,
  "summary":"Positive call outcome."
}`

func TestNewLLMSentimentAnalyser_NilClient(t *testing.T) {
	_, err := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{})
	require.Error(t, err)
}

func TestLLMSentimentAnalyser_Name(t *testing.T) {
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{}})
	assert.Equal(t, "llm_sentiment", a.Name())
}

func TestLLMSentimentAnalyser_EmptyTranscript(t *testing.T) {
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{}})
	r, err := a.Analyse(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Equal(t, SentimentNeutral, r.Overall.Label)
}

func TestLLMSentimentAnalyser_HappyPath(t *testing.T) {
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{content: goodSentimentJSON}})
	r, err := a.Analyse(context.Background(), "great meeting", nil)
	require.NoError(t, err)
	assert.Contains(t, []SentimentLabel{SentimentPositive, SentimentNegative, SentimentNeutral, SentimentMixed}, r.Overall.Label)
	assert.InDelta(t, 1.0, r.Overall.Positive+r.Overall.Negative+r.Overall.Neutral, 0.06)
	assert.Equal(t, "llm_sentiment", r.Provider)
}

func TestLLMSentimentAnalyser_NormalisationClampsScores(t *testing.T) {
	j := `{"overall":{"label":"positive","score":1.5,"positive":0.5,"negative":0.3,"neutral":0.2,"confidence":0.9},"per_speaker":[],"escalation_signal":false,"summary":"ok"}`
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{content: j}})
	r, err := a.Analyse(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, r.Overall.Score, 1.0)
}

func TestLLMSentimentAnalyser_EscalationFromFrustration(t *testing.T) {
	j := `{"overall":{"label":"negative","score":0.7,"positive":0.1,"negative":0.7,"neutral":0.2,"confidence":0.8},"per_speaker":[{
      "speaker":"S1",
      "sentiment":{"label":"negative","score":0.7,"positive":0.0,"negative":0.8,"neutral":0.2,"confidence":0.8},
      "emotion":{"primary":"frustration","scores":{"frustration":0.6,"anger":0.0,"joy":0.0,"satisfaction":0.0,"anxiety":0.0,"neutral":0.0},"confidence":0.8}
    }],"escalation_signal":false,"summary":"tense"}`
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{content: j}})
	r, err := a.Analyse(context.Background(), "you never listen!", nil)
	require.NoError(t, err)
	assert.True(t, r.EscalationSignal)
}

func TestLLMSentimentAnalyser_EscalationRecomputed(t *testing.T) {
	j := `{"overall":{"label":"negative","score":0.6,"positive":0.1,"negative":0.6,"neutral":0.3,"confidence":0.7},"per_speaker":[{
      "speaker":"S1",
      "sentiment":{"label":"negative","score":0.6,"positive":0.0,"negative":0.6,"neutral":0.4,"confidence":0.7},
      "emotion":{"primary":"anger","scores":{"anger":0.7,"frustration":0.0,"joy":0.0,"satisfaction":0.0,"anxiety":0.0,"neutral":0.0},"confidence":0.8}
    }],"escalation_signal":false,"summary":"angry"}`
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{content: j}})
	r, err := a.Analyse(context.Background(), "this is unacceptable!", nil)
	require.NoError(t, err)
	assert.True(t, r.EscalationSignal, "must override LLM false when anger > 0.4")
}

func TestLLMSentimentAnalyser_MalformedJSON(t *testing.T) {
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{content: "not json"}})
	_, err := a.Analyse(context.Background(), "test", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSentimentProvider)
}

func TestLLMSentimentAnalyser_LLMError(t *testing.T) {
	a, _ := NewLLMSentimentAnalyser(LLMSentimentAnalyserConfig{LLMClient: &mockSentimentLLM{err: fmt.Errorf("rate limit")}})
	_, err := a.Analyse(context.Background(), "test", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSentimentProvider)
}

func TestNormaliseSentimentScore_ComponentNormalisation(t *testing.T) {
	s := normaliseSentimentScore(SentimentScore{Positive: 0.6, Negative: 0.6, Neutral: 0.6})
	total := s.Positive + s.Negative + s.Neutral
	assert.InDelta(t, 1.0, total, 0.01)
}

func TestNormaliseConfidence_Clamping(t *testing.T) {
	assert.Equal(t, 1.0, NormaliseConfidence(1.5))
	assert.Equal(t, -1.0, NormaliseConfidence(-1))
	assert.Equal(t, 0.8, NormaliseConfidence(0.8))
}

func TestSynthesizeTranscription_UnknownConfidence(t *testing.T) {
	primary := STTResult{Text: "primary", Confidence: NormaliseConfidence(-1), Provider: "whisper"}
	fallback := STTResult{Text: "fallback", Confidence: 0.9, Provider: "deepgram"}
	result := SynthesizeTranscription(primary, fallback, 0.7)
	assert.Equal(t, "deepgram", result.Provider, "unknown confidence must prefer fallback")
}
