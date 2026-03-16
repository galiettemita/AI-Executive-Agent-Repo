package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func newWSTestServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func wsResultMsg(transcript string, isFinal bool) []byte {
	msg := deepgramWSResult{}
	msg.Type = "Results"
	msg.IsFinal = isFinal
	msg.Channel.Alternatives = []struct {
		Transcript string  `json:"transcript"`
		Confidence float64 `json:"confidence"`
		Words      []struct {
			Word    string  `json:"word"`
			Start   float64 `json:"start"`
			End     float64 `json:"end"`
			Speaker *int    `json:"speaker,omitempty"`
		} `json:"words"`
	}{{Transcript: transcript, Confidence: 0.98}}
	b, _ := json.Marshal(msg)
	return b
}

func TestNewDeepgramWSClient_EmptyAPIKey(t *testing.T) {
	_, err := NewDeepgramWSClient(DeepgramWSConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APIKey is required")
}

func TestDeepgramWSClient_SendAudioFrame_NotConnected(t *testing.T) {
	c, _ := NewDeepgramWSClient(DeepgramWSConfig{APIKey: "key"})
	err := c.SendAudioFrame([]byte{0x00, 0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestDeepgramWSClient_CloseStream_NotConnected(t *testing.T) {
	c, _ := NewDeepgramWSClient(DeepgramWSConfig{APIKey: "key"})
	assert.NoError(t, c.CloseStream()) // must not panic
}

func TestDeepgramWSClient_Connect_InvalidURL(t *testing.T) {
	c, _ := NewDeepgramWSClient(DeepgramWSConfig{
		APIKey:          "key",
		BaseURLOverride: "http://localhost:1", // nothing listening
		ConnectTimeout:  100 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := c.Connect(ctx)
	require.Error(t, err)
}

func TestDeepgramWSClient_StreamTranscripts_FinalTranscript(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.TextMessage, wsResultMsg("Hello world", true))
		conn.ReadMessage() // wait for close
	})

	c, _ := NewDeepgramWSClient(DeepgramWSConfig{
		APIKey: "key", BaseURLOverride: srv.URL,
	})
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer c.Close()

	events := make(chan TranscriptEvent, 4)
	go func() { c.StreamTranscripts(ctx, events) }()

	select {
	case e := <-events:
		assert.Equal(t, "Hello world", e.Text)
		assert.True(t, e.IsFinal)
		assert.InDelta(t, 0.98, e.Confidence, 0.01)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transcript")
	}
}

func TestDeepgramWSClient_StreamTranscripts_InterimAndFinal(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.TextMessage, wsResultMsg("hel", false))
		conn.WriteMessage(websocket.TextMessage, wsResultMsg("hello", true))
	})

	c, _ := NewDeepgramWSClient(DeepgramWSConfig{
		APIKey: "key", BaseURLOverride: srv.URL, InterimResults: true,
	})
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	defer c.Close()

	events := make(chan TranscriptEvent, 8)
	go func() { c.StreamTranscripts(ctx, events) }()

	var received []TranscriptEvent
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case e := <-events:
			received = append(received, e)
			if e.IsFinal {
				break loop
			}
		case <-deadline:
			break loop
		}
	}
	assert.NotEmpty(t, received)
	last := received[len(received)-1]
	assert.True(t, last.IsFinal)
	assert.Equal(t, "hello", last.Text)
}

func TestDeepgramWSClient_StreamTranscripts_ContextCancel(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		time.Sleep(5 * time.Second)
	})

	c, _ := NewDeepgramWSClient(DeepgramWSConfig{
		APIKey: "key", BaseURLOverride: srv.URL,
	})
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, c.Connect(ctx))

	events := make(chan TranscriptEvent, 4)
	errCh := make(chan error, 1)
	go func() { errCh <- c.StreamTranscripts(ctx, events) }()

	cancel()
	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cancellation")
	}
}

func TestDeepgramWSClient_BuildURL_Production(t *testing.T) {
	c, _ := NewDeepgramWSClient(DeepgramWSConfig{APIKey: "key"})
	u := c.buildURL()
	assert.Contains(t, u, "wss://api.deepgram.com")
	assert.Contains(t, u, "nova-3")
}

func TestDeepgramWSClient_BuildURL_Override(t *testing.T) {
	c, _ := NewDeepgramWSClient(DeepgramWSConfig{
		APIKey: "key", BaseURLOverride: "http://localhost:9999",
	})
	u := c.buildURL()
	assert.Contains(t, u, "ws://localhost:9999")
	assert.NotContains(t, u, "deepgram.com")
}
