package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/brevio/brevio/internal/llm"
)

// TranscriptEvent is emitted by the STT stage.
type TranscriptEvent struct {
	Text       string
	IsFinal    bool
	Speaker    string
	Confidence float64
}

// AudioChunkEvent is emitted by the TTS stage.
type AudioChunkEvent struct {
	Data     []byte
	Sequence int
	IsFinal  bool
}

// StreamingPipelineConfig configures the dual-streaming pipeline.
type StreamingPipelineConfig struct {
	LLMClient         llm.Client
	TTS               StreamingTTSProvider
	STT               STTProvider           // used for URL-based fallback
	SystemPrompt      string
	Voice             string
	Format            string
	MaxResponseTokens int
	SentenceMinTokens int           // tokens before first sentence flush; default 20
	InactivityTimeout time.Duration // max silence before treating as end-of-turn; default 30s
	VAD               VADProvider   // optional; wired by PROMPT 08
}

// StreamingPipeline orchestrates STT→LLM→TTS dual-streaming.
type StreamingPipeline struct {
	cfg StreamingPipelineConfig
}

// NewStreamingPipeline creates a StreamingPipeline. LLMClient and TTS are required.
func NewStreamingPipeline(cfg StreamingPipelineConfig) (*StreamingPipeline, error) {
	if cfg.LLMClient == nil {
		return nil, fmt.Errorf("streaming pipeline: LLMClient is required")
	}
	if cfg.TTS == nil {
		return nil, fmt.Errorf("streaming pipeline: TTS provider is required")
	}
	if cfg.SentenceMinTokens <= 0 {
		cfg.SentenceMinTokens = 20
	}
	if cfg.MaxResponseTokens <= 0 {
		cfg.MaxResponseTokens = 1024
	}
	if cfg.InactivityTimeout <= 0 {
		cfg.InactivityTimeout = 30 * time.Second
	}
	return &StreamingPipeline{cfg: cfg}, nil
}

// Process collects a final transcript from transcriptEvents then streams LLM→TTS,
// emitting AudioChunkEvents to audioSink. audioSink is always closed before return.
func (sp *StreamingPipeline) Process(
	ctx context.Context,
	transcriptEvents <-chan TranscriptEvent,
	audioSink chan<- AudioChunkEvent,
) error {
	defer close(audioSink)

	final, err := sp.collectFinalTranscript(ctx, transcriptEvents)
	if err != nil {
		return fmt.Errorf("streaming pipeline: collect transcript: %w", err)
	}
	if strings.TrimSpace(final) == "" {
		return nil
	}
	return sp.streamLLMToTTS(ctx, final, audioSink)
}

func (sp *StreamingPipeline) collectFinalTranscript(
	ctx context.Context,
	events <-chan TranscriptEvent,
) (string, error) {
	timer := time.NewTimer(sp.cfg.InactivityTimeout)
	defer timer.Stop()
	var latest string
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return latest, nil
		case ev, ok := <-events:
			if !ok {
				return latest, nil
			}
			timer.Reset(sp.cfg.InactivityTimeout)
			latest = ev.Text
			if ev.IsFinal {
				return latest, nil
			}
		}
	}
}

func (sp *StreamingPipeline) streamLLMToTTS(
	ctx context.Context,
	transcript string,
	audioSink chan<- AudioChunkEvent,
) error {
	systemPrompt := sp.cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful executive assistant. Be concise and direct."
	}

	req := llm.GenerateRequest{
		System:    systemPrompt,
		Messages:  []llm.ChatMsg{{Role: "user", Content: transcript}},
		MaxTokens: sp.cfg.MaxResponseTokens,
	}

	llmOut := make(chan llm.StreamChunk, 64)
	go sp.cfg.LLMClient.Stream(ctx, req, llmOut)

	sentBuf := &sentenceAccumulator{minTokens: sp.cfg.SentenceMinTokens}
	chunkSeq := 0
	var wg sync.WaitGroup
	var mu sync.Mutex
	var pipelineErr error

	for chunk := range llmOut {
		if chunk.Error != nil {
			pipelineErr = fmt.Errorf("streaming pipeline: LLM error: %w", chunk.Error)
			break
		}
		if chunk.Done {
			if rem := sentBuf.Flush(); rem != "" {
				seq := chunkSeq
				chunkSeq++
				wg.Add(1)
				go func(s string, n int) {
					defer wg.Done()
					if e := sp.synthesiseAndEmit(ctx, s, audioSink, n, true); e != nil {
						mu.Lock()
						pipelineErr = e
						mu.Unlock()
					}
				}(rem, seq)
			}
			break
		}
		sentBuf.Write(chunk.Delta)
		if sentence := sentBuf.TryFlushSentence(); sentence != "" {
			seq := chunkSeq
			chunkSeq++
			wg.Add(1)
			go func(s string, n int) {
				defer wg.Done()
				if e := sp.synthesiseAndEmit(ctx, s, audioSink, n, false); e != nil {
					mu.Lock()
					pipelineErr = e
					mu.Unlock()
				}
			}(sentence, seq)
		}
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	return pipelineErr
}

func (sp *StreamingPipeline) synthesiseAndEmit(
	ctx context.Context,
	text string,
	audioSink chan<- AudioChunkEvent,
	baseSeq int,
	isFinalSentence bool,
) error {
	sink := make(chan []byte, 32)
	opts := TTSOptions{Voice: sp.cfg.Voice, Format: sp.cfg.Format}
	errCh := make(chan error, 1)
	go func() { errCh <- sp.cfg.TTS.SynthesizeStream(ctx, text, opts, sink) }()

	idx := 0
	for audio := range sink {
		select {
		case audioSink <- AudioChunkEvent{Data: audio, Sequence: baseSeq*100 + idx}:
		case <-ctx.Done():
			return ctx.Err()
		}
		idx++
	}
	if err := <-errCh; err != nil {
		return err
	}
	if isFinalSentence && idx > 0 {
		audioSink <- AudioChunkEvent{Sequence: baseSeq*100 + idx, IsFinal: true}
	}
	return nil
}

// sentenceAccumulator buffers LLM tokens and flushes at sentence boundaries.
type sentenceAccumulator struct {
	minTokens  int
	tokenCount int
	buf        strings.Builder
}

func (s *sentenceAccumulator) Write(token string) {
	if token == "" {
		return
	}
	s.tokenCount++
	s.buf.WriteString(token)
}

func (s *sentenceAccumulator) TryFlushSentence() string {
	if s.tokenCount < s.minTokens {
		return ""
	}
	text := s.buf.String()
	idx := findSentenceBoundary(text)
	if idx < 0 {
		return ""
	}
	sentence := strings.TrimSpace(text[:idx+1])
	remainder := text[idx+1:]
	s.buf.Reset()
	s.buf.WriteString(remainder)
	s.tokenCount = 0
	return sentence
}

func (s *sentenceAccumulator) Flush() string {
	result := strings.TrimSpace(s.buf.String())
	s.buf.Reset()
	s.tokenCount = 0
	return result
}

// findSentenceBoundary returns the index of '.', '!', or '?' followed by whitespace
// or end of string. Returns -1 if none found.
func findSentenceBoundary(text string) int {
	runes := []rune(text)
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		if r == '.' || r == '!' || r == '?' {
			if i == len(runes)-1 || unicode.IsSpace(runes[i+1]) {
				return i
			}
		}
	}
	return -1
}
