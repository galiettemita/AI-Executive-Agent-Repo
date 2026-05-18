package worker

import (
	"sync"
	"testing"
	"time"
)

func TestDispatchBasic(t *testing.T) {
	t.Parallel()

	wd := NewWorkerDispatch(4)

	err := wd.Dispatch(VoiceSession{ID: "session-1", WorkspaceID: "ws1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Give goroutine time to start and finish.
	time.Sleep(50 * time.Millisecond)
}

func TestDispatchEmptySessionID(t *testing.T) {
	t.Parallel()

	wd := NewWorkerDispatch(4)

	err := wd.Dispatch(VoiceSession{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestDispatchMaxWorkers(t *testing.T) {
	t.Parallel()

	wd := NewWorkerDispatch(2)

	if wd.MaxWorkers() != 2 {
		t.Fatalf("expected max workers 2, got %d", wd.MaxWorkers())
	}
}

func TestDispatchDefaultMaxWorkers(t *testing.T) {
	t.Parallel()

	wd := NewWorkerDispatch(0)
	if wd.MaxWorkers() != 4 {
		t.Fatalf("expected default max workers 4, got %d", wd.MaxWorkers())
	}
}

func TestDispatchConcurrency(t *testing.T) {
	t.Parallel()

	wd := NewWorkerDispatch(3)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			wd.Dispatch(VoiceSession{
				ID:          "session-" + string(rune('0'+idx)),
				WorkspaceID: "ws1",
			})
		}(i)
	}

	wg.Wait()
	// Allow goroutines to complete.
	time.Sleep(100 * time.Millisecond)

	if wd.ActiveWorkers() < 0 {
		t.Fatalf("active workers should not be negative, got %d", wd.ActiveWorkers())
	}
}
