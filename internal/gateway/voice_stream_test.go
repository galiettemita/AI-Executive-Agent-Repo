package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock types ──────────────────────────────────────────────────────────────

type mockStreamingTTS struct {
	chunks [][]byte
	err    error
}

func (m *mockStreamingTTS) Name() string { return "mock-tts" }
func (m *mockStreamingTTS) Synthesize(_ context.Context, _ string, _ TTSOptions) (*AudioResult, error) {
	return &AudioResult{AudioURL: "https://mock.example.com/audio.mp3"}, nil
}
func (m *mockStreamingTTS) SynthesizeStream(_ context.Context, _ string, _ TTSOptions, sink chan<- []byte) error {
	defer close(sink)
	for _, c := range m.chunks {
		sink <- c
	}
	return m.err
}

// mockLLMClient implements llm.Client.
type mockLLMClient struct {
	tokens []string
	err    error
}

func (m *mockLLMClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	return &llm.GenerateResponse{Content: strings.Join(m.tokens, "")}, &llm.Usage{}, nil
}

func (m *mockLLMClient) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
	for _, t := range m.tokens {
		out <- llm.StreamChunk{Delta: t}
	}
	if m.err != nil {
		out <- llm.StreamChunk{Error: m.err}
	} else {
		out <- llm.StreamChunk{Done: true}
	}
}

// ── StreamingPipeline tests ─────────────────────────────────────────────────

func TestNewStreamingPipeline_NilLLMClient(t *testing.T) {
	_, err := NewStreamingPipeline(StreamingPipelineConfig{TTS: &mockStreamingTTS{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLMClient is required")
}

func TestNewStreamingPipeline_NilTTS(t *testing.T) {
	_, err := NewStreamingPipeline(StreamingPipelineConfig{LLMClient: &mockLLMClient{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TTS provider is required")
}

func TestNewStreamingPipeline_DefaultSentenceMinTokens(t *testing.T) {
	sp, err := NewStreamingPipeline(StreamingPipelineConfig{
		LLMClient: &mockLLMClient{},
		TTS:       &mockStreamingTTS{},
	})
	require.NoError(t, err)
	assert.Equal(t, 20, sp.cfg.SentenceMinTokens)
}

func TestSentenceAccumulator_TryFlush_NotEnoughTokens(t *testing.T) {
	s := &sentenceAccumulator{minTokens: 20}
	for i := 0; i < 5; i++ {
		s.Write("word ")
	}
	assert.Equal(t, "", s.TryFlushSentence())
}

func TestSentenceAccumulator_TryFlush_HasBoundary(t *testing.T) {
	s := &sentenceAccumulator{minTokens: 3}
	for _, w := range []string{"Hello", " ", "world", ".", " ", "More"} {
		s.Write(w)
	}
	result := s.TryFlushSentence()
	assert.Contains(t, result, ".")
	assert.NotEmpty(t, result)
}

func TestSentenceAccumulator_Flush_ReturnsAll(t *testing.T) {
	s := &sentenceAccumulator{minTokens: 100}
	for i := 0; i < 10; i++ {
		s.Write("word ")
	}
	assert.Contains(t, s.Flush(), "word")
}

func TestFindSentenceBoundary_Period(t *testing.T) {
	idx := findSentenceBoundary("Hello world. More text")
	assert.GreaterOrEqual(t, idx, 0)
}

func TestFindSentenceBoundary_NoBoundary(t *testing.T) {
	assert.Equal(t, -1, findSentenceBoundary("Hello world"))
}

func TestFindSentenceBoundary_Exclamation(t *testing.T) {
	idx := findSentenceBoundary("Stop! Now")
	assert.GreaterOrEqual(t, idx, 0)
}

func TestStreamingPipeline_Process_EmptyTranscript(t *testing.T) {
	sp, _ := NewStreamingPipeline(StreamingPipelineConfig{
		LLMClient: &mockLLMClient{},
		TTS:       &mockStreamingTTS{},
	})
	events := make(chan TranscriptEvent)
	audioSink := make(chan AudioChunkEvent, 10)
	close(events)
	err := sp.Process(context.Background(), events, audioSink)
	assert.NoError(t, err)
	_, ok := <-audioSink
	assert.False(t, ok, "audioSink must be closed")
}

func TestStreamingPipeline_Process_HappyPath(t *testing.T) {
	tokens := make([]string, 0, 30)
	for i := 0; i < 5; i++ {
		tokens = append(tokens, "word ")
	}
	tokens = append(tokens, "done. ", "extra ")

	sp, _ := NewStreamingPipeline(StreamingPipelineConfig{
		LLMClient:         &mockLLMClient{tokens: tokens},
		TTS:               &mockStreamingTTS{chunks: [][]byte{{1, 2, 3}}},
		SentenceMinTokens: 3,
	})
	events := make(chan TranscriptEvent, 1)
	events <- TranscriptEvent{Text: "test input", IsFinal: true}
	audioSink := make(chan AudioChunkEvent, 100)

	err := sp.Process(context.Background(), events, audioSink)
	assert.NoError(t, err)
	var received []AudioChunkEvent
	for e := range audioSink {
		received = append(received, e)
	}
	assert.NotEmpty(t, received)
}

func TestStreamingPipeline_Process_TTSError(t *testing.T) {
	tokens := make([]string, 0, 10)
	for i := 0; i < 5; i++ {
		tokens = append(tokens, "word ")
	}
	tokens = append(tokens, "end. ")

	sp, _ := NewStreamingPipeline(StreamingPipelineConfig{
		LLMClient:         &mockLLMClient{tokens: tokens},
		TTS:               &mockStreamingTTS{err: fmt.Errorf("quota exceeded")},
		SentenceMinTokens: 3,
	})
	events := make(chan TranscriptEvent, 1)
	events <- TranscriptEvent{Text: "test", IsFinal: true}
	audioSink := make(chan AudioChunkEvent, 100)
	err := sp.Process(context.Background(), events, audioSink)
	assert.Error(t, err)
}

// ── Cartesia SSE streaming tests ─────────────────────────────────────────────

func TestCartesiaStream_SynthesizeStream_HappyPath(t *testing.T) {
	sseBody := "data: {\"type\":\"chunk\",\"data\":\"AQID\"}\n\n" +
		"data: {\"type\":\"chunk\",\"data\":\"BAUG\"}\n\n" +
		"data: {\"type\":\"done\"}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	up := &mockUploader{returnURL: "https://s3.example.com/test.mp3"}
	p, err := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{
		APIKey: "test-key", BaseURL: srv.URL,
	}, up)
	require.NoError(t, err)

	sink := make(chan []byte, 10)
	err = p.SynthesizeStream(context.Background(), "hello world", TTSOptions{}, sink)
	assert.NoError(t, err)
	var chunks [][]byte
	for c := range sink {
		chunks = append(chunks, c)
	}
	assert.Len(t, chunks, 2)
}

func TestCartesiaStream_SynthesizeStream_ErrorEvent(t *testing.T) {
	sseBody := "data: {\"type\":\"error\",\"message\":\"quota exceeded\"}\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	up := &mockUploader{returnURL: "https://s3.example.com/test.mp3"}
	p, _ := NewCartesiaTTSProvider(CartesiaTTSProviderConfig{
		APIKey: "test-key", BaseURL: srv.URL,
	}, up)

	sink := make(chan []byte, 10)
	err := p.SynthesizeStream(context.Background(), "hello", TTSOptions{}, sink)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

func TestStreamingPipeline_CollectFinalTranscript_Timeout(t *testing.T) {
	sp, _ := NewStreamingPipeline(StreamingPipelineConfig{
		LLMClient:         &mockLLMClient{},
		TTS:               &mockStreamingTTS{},
		InactivityTimeout: 50 * time.Millisecond,
	})
	events := make(chan TranscriptEvent)
	result, err := sp.collectFinalTranscript(context.Background(), events)
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}
