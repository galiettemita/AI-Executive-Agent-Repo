package canvas

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketConnectionAndToolInjection(t *testing.T) {
	t.Parallel()

	injector := &InMemoryInjector{}
	svc := NewService(injector)
	server := httptest.NewServer(NewMux(svc))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	message := `{"type":"click","payload":"open task panel"}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("write message: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	if injector.Count() != 1 {
		t.Fatalf("expected 1 injected tool call, got %d", injector.Count())
	}
}
