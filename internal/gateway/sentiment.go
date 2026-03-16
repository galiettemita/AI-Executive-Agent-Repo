package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/llm"
)

// SentimentLabel categorises overall sentiment.
type SentimentLabel string

const (
	SentimentPositive SentimentLabel = "positive"
	SentimentNeutral  SentimentLabel = "neutral"
	SentimentNegative SentimentLabel = "negative"
	SentimentMixed    SentimentLabel = "mixed"
)

// EmotionLabel categorises a detected emotion.
type EmotionLabel string

const (
	EmotionJoy          EmotionLabel = "joy"
	EmotionSatisfaction EmotionLabel = "satisfaction"
	EmotionFrustration  EmotionLabel = "frustration"
	EmotionAnger        EmotionLabel = "anger"
	EmotionAnxiety      EmotionLabel = "anxiety"
	EmotionSurprise     EmotionLabel = "surprise"
	EmotionNeutral      EmotionLabel = "neutral"
)

// SentimentScore holds per-component sentiment scores.
type SentimentScore struct {
	Label      SentimentLabel `json:"label"`
	Score      float64        `json:"score"`
	Positive   float64        `json:"positive"`
	Negative   float64        `json:"negative"`
	Neutral    float64        `json:"neutral"`
	Confidence float64        `json:"confidence"`
}

// EmotionScore holds per-emotion scores.
type EmotionScore struct {
	Primary    EmotionLabel             `json:"primary"`
	Scores     map[EmotionLabel]float64 `json:"scores"`
	Confidence float64                  `json:"confidence"`
}

// SpeakerSentiment holds sentiment and emotion for a single speaker.
type SpeakerSentiment struct {
	Speaker   string         `json:"speaker"`
	Sentiment SentimentScore `json:"sentiment"`
	Emotion   EmotionScore   `json:"emotion"`
}

// CallSentimentResult is the complete sentiment analysis output for a call.
type CallSentimentResult struct {
	Overall          SentimentScore     `json:"overall"`
	PerSpeaker       []SpeakerSentiment `json:"per_speaker"`
	EscalationSignal bool               `json:"escalation_signal"`
	Summary          string             `json:"summary"`
	AnalysedAt       time.Time          `json:"analysed_at"`
	Provider         string             `json:"provider"`
}

// ErrSentimentProvider signals an error from the sentiment analysis provider.
var ErrSentimentProvider = errors.New("sentiment: provider error")

// SentimentAnalyser computes sentiment and emotion from transcript text.
type SentimentAnalyser interface {
	Analyse(ctx context.Context, transcript string, turns []SpeakerTurn) (*CallSentimentResult, error)
	Name() string
}

const sentimentSystem = `You are a precise sentiment and emotion analysis engine for business call transcripts.

Analyse the provided transcript and return structured JSON.

Rules:
- Overall sentiment label: positive | negative | neutral | mixed
- Component scores (positive, negative, neutral) must sum to approximately 1.0
- Emotion categories: joy, satisfaction, frustration, anger, anxiety, surprise, neutral
- escalation_signal: true if any speaker has frustration or anger > 0.4
- Return ONLY valid JSON, no markdown fences

Response schema:
{
  "overall": {"label":"...","score":0.0-1.0,"positive":0.0-1.0,"negative":0.0-1.0,"neutral":0.0-1.0,"confidence":0.0-1.0},
  "per_speaker": [
    {
      "speaker": "string",
      "sentiment": {"label":"...","score":0.0-1.0,"positive":0.0-1.0,"negative":0.0-1.0,"neutral":0.0-1.0,"confidence":0.0-1.0},
      "emotion": {"primary":"string","scores":{"joy":0.0-1.0,"satisfaction":0.0-1.0,"frustration":0.0-1.0,"anger":0.0-1.0,"anxiety":0.0-1.0,"neutral":0.0-1.0},"confidence":0.0-1.0}
    }
  ],
  "escalation_signal": true|false,
  "summary": "string (1 sentence)"
}`

// LLMSentimentAnalyserConfig configures the LLM-backed sentiment analyser.
type LLMSentimentAnalyserConfig struct {
	LLMClient llm.Client
	Timeout   time.Duration
}

// LLMSentimentAnalyser analyses sentiment using an LLM.
type LLMSentimentAnalyser struct {
	cfg LLMSentimentAnalyserConfig
}

// NewLLMSentimentAnalyser creates an LLMSentimentAnalyser.
func NewLLMSentimentAnalyser(cfg LLMSentimentAnalyserConfig) (*LLMSentimentAnalyser, error) {
	if cfg.LLMClient == nil {
		return nil, fmt.Errorf("llm sentiment analyser: LLMClient is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &LLMSentimentAnalyser{cfg: cfg}, nil
}

func (a *LLMSentimentAnalyser) Name() string { return "llm_sentiment" }

func (a *LLMSentimentAnalyser) Analyse(ctx context.Context, transcript string, turns []SpeakerTurn) (*CallSentimentResult, error) {
	if strings.TrimSpace(transcript) == "" {
		return neutralSentimentResult(a.Name()), nil
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()

	inputText := transcript
	if len(turns) > 0 {
		dr := &DiarizationResult{Turns: turns}
		inputText = dr.ToTranscriptText(true)
	}

	req := llm.GenerateRequest{
		System:    sentimentSystem,
		Messages:  []llm.ChatMsg{{Role: "user", Content: inputText}},
		MaxTokens: 1024,
	}

	resp, _, err := a.cfg.LLMClient.Generate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: LLM call failed: %v", ErrSentimentProvider, err)
	}

	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result CallSentimentResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		preview := content
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("%w: unmarshal response: %v (raw: %s)", ErrSentimentProvider, err, preview)
	}

	result.Overall = normaliseSentimentScore(result.Overall)
	for i := range result.PerSpeaker {
		result.PerSpeaker[i].Sentiment = normaliseSentimentScore(result.PerSpeaker[i].Sentiment)
		result.PerSpeaker[i].Emotion = normaliseEmotionScore(result.PerSpeaker[i].Emotion)
	}
	result.AnalysedAt = time.Now().UTC()
	result.Provider = a.Name()
	// Recompute escalation from actual scores — don't rely solely on LLM boolean.
	result.EscalationSignal = result.EscalationSignal || checkEscalation(result.PerSpeaker)

	return &result, nil
}

func normaliseSentimentScore(s SentimentScore) SentimentScore {
	// clampF is defined in tts_cartesia.go (PROMPT 05) — same gateway package.
	s.Score = clampF(s.Score, 0, 1)
	s.Positive = clampF(s.Positive, 0, 1)
	s.Negative = clampF(s.Negative, 0, 1)
	s.Neutral = clampF(s.Neutral, 0, 1)
	s.Confidence = clampF(s.Confidence, 0, 1)
	total := s.Positive + s.Negative + s.Neutral
	if total > 0 && math.Abs(total-1.0) > 0.05 {
		s.Positive /= total
		s.Negative /= total
		s.Neutral /= total
	}
	return s
}

func normaliseEmotionScore(e EmotionScore) EmotionScore {
	e.Confidence = clampF(e.Confidence, 0, 1)
	if e.Scores == nil {
		e.Scores = make(map[EmotionLabel]float64)
	}
	for k, v := range e.Scores {
		e.Scores[k] = clampF(v, 0, 1)
	}
	return e
}

func checkEscalation(speakers []SpeakerSentiment) bool {
	for _, s := range speakers {
		if s.Emotion.Scores[EmotionFrustration] > 0.4 || s.Emotion.Scores[EmotionAnger] > 0.4 {
			return true
		}
	}
	return false
}

func neutralSentimentResult(provider string) *CallSentimentResult {
	return &CallSentimentResult{
		Overall: SentimentScore{
			Label: SentimentNeutral, Score: 0.5,
			Positive: 0.33, Negative: 0.0, Neutral: 0.67, Confidence: 0.5,
		},
		PerSpeaker: []SpeakerSentiment{},
		Summary:    "No content to analyse.",
		AnalysedAt: time.Now().UTC(),
		Provider:   provider,
	}
}
