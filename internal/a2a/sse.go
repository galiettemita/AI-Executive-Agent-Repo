package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// StreamTaskStatus streams A2A task status updates via SSE.
func StreamTaskStatus(ctx context.Context, w http.ResponseWriter, taskID string, store *TaskStore, maxWait time.Duration) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			task, err := store.Get(ctx, taskID)
			if err != nil {
				writeSSEEvent(w, "error", map[string]string{"error": err.Error()})
				flusher.Flush()
				return
			}

			eventName := "status_update"
			if task.Status == TaskStatusCompleted {
				eventName = "completed"
			} else if task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
				eventName = "failed"
			}

			writeSSEEvent(w, eventName, task)
			flusher.Flush()

			if isTerminalStatus(task.Status) {
				return
			}
			if time.Now().After(deadline) {
				writeSSEEvent(w, "timeout", map[string]string{"message": "max wait exceeded"})
				flusher.Flush()
				return
			}
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event string, data any) {
	encoded, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, encoded)
}

func isTerminalStatus(s TaskStatus) bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed || s == TaskStatusCancelled
}
