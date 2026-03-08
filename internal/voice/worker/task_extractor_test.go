package worker

import (
	"testing"
	"time"
)

func TestExtractTasksBasicPatterns(t *testing.T) {
	t.Parallel()

	te := NewTaskExtractor()
	transcript := []TranscriptTurn{
		{Speaker: "user", Text: "I need to schedule a dentist appointment", Timestamp: time.Now()},
		{Speaker: "agent", Text: "Sure, I can help with that", Timestamp: time.Now()},
		{Speaker: "user", Text: "Also remind me to call the plumber tomorrow", Timestamp: time.Now()},
	}

	tasks := te.ExtractTasks(transcript)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].SourceTurnIndex != 0 {
		t.Fatalf("expected source turn 0, got %d", tasks[0].SourceTurnIndex)
	}
	if tasks[0].AssignedTo != "user" {
		t.Fatalf("expected assigned to user, got %s", tasks[0].AssignedTo)
	}
}

func TestExtractTasksPriority(t *testing.T) {
	t.Parallel()

	te := NewTaskExtractor()
	transcript := []TranscriptTurn{
		{Speaker: "user", Text: "I need to fix this urgent issue immediately"},
		{Speaker: "user", Text: "Remind me to book a flight today"},
		{Speaker: "user", Text: "I need to review the document next month"},
	}

	tasks := te.ExtractTasks(transcript)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].Priority != 1 {
		t.Fatalf("expected priority 1 for urgent task, got %d", tasks[0].Priority)
	}
	if tasks[1].Priority != 2 {
		t.Fatalf("expected priority 2 for today task, got %d", tasks[1].Priority)
	}
	if tasks[2].Priority != 3 {
		t.Fatalf("expected priority 3 for normal task, got %d", tasks[2].Priority)
	}
}

func TestExtractTasksDueDate(t *testing.T) {
	t.Parallel()

	te := NewTaskExtractor()
	transcript := []TranscriptTurn{
		{Speaker: "user", Text: "I need to finish the report by tomorrow"},
		{Speaker: "user", Text: "Schedule a meeting for friday"},
	}

	tasks := te.ExtractTasks(transcript)
	if tasks[0].DueDate != "tomorrow" {
		t.Fatalf("expected due date 'tomorrow', got %s", tasks[0].DueDate)
	}
	if tasks[1].DueDate != "friday" {
		t.Fatalf("expected due date 'friday', got %s", tasks[1].DueDate)
	}
}

func TestExtractTasksNoTasks(t *testing.T) {
	t.Parallel()

	te := NewTaskExtractor()
	transcript := []TranscriptTurn{
		{Speaker: "user", Text: "Hello, how are you?"},
		{Speaker: "agent", Text: "I'm doing well, thanks!"},
	}

	tasks := te.ExtractTasks(transcript)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestExtractTasksEmptyTranscript(t *testing.T) {
	t.Parallel()

	te := NewTaskExtractor()
	tasks := te.ExtractTasks(nil)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}
