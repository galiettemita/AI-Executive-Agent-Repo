package compliance

import (
	"context"
	"slices"
	"testing"
	"time"
)

type fakeExecutionLogPIIScrubStore struct {
	records          []ExecutionLogRecord
	receivedBefore   time.Time
	receivedLimit    int
	scrubbedIDs      []string
	scrubbedReason   string
	scrubbedAt       time.Time
	nullifyCallCount int
}

func (f *fakeExecutionLogPIIScrubStore) ListExecutionLogsOlderThan(_ context.Context, before time.Time, limit int) ([]ExecutionLogRecord, error) {
	f.receivedBefore = before
	f.receivedLimit = limit
	return f.records, nil
}

func (f *fakeExecutionLogPIIScrubStore) NullifyExecutionLogPayloads(_ context.Context, ids []string, reason string, scrubbedAt time.Time) error {
	f.nullifyCallCount++
	f.scrubbedIDs = append([]string(nil), ids...)
	f.scrubbedReason = reason
	f.scrubbedAt = scrubbedAt
	return nil
}

func TestNextExecutionLogPIIScrubRun(t *testing.T) {
	t.Parallel()

	morning := time.Date(2026, time.March, 3, 1, 0, 0, 0, time.UTC)
	next := NextExecutionLogPIIScrubRun(morning)
	if next != time.Date(2026, time.March, 3, 3, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected next run for pre-3am: %s", next)
	}

	after := time.Date(2026, time.March, 3, 4, 30, 0, 0, time.UTC)
	next = NextExecutionLogPIIScrubRun(after)
	if next != time.Date(2026, time.March, 4, 3, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected next run for post-3am: %s", next)
	}
}

func TestShouldScrubExecutionLogPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		output string
		want   bool
	}{
		{name: "email", input: `{"email":"ceo@example.com"}`, want: true},
		{name: "phone", output: `{"phone":"+1 (415) 555-1212"}`, want: true},
		{name: "ssn", input: `{"ssn":"123-45-6789"}`, want: true},
		{name: "credit_card", output: `{"card":"4111 1111 1111 1111"}`, want: true},
		{name: "non_pii", input: `{"topic":"quarterly strategy planning"}`, output: `{"status":"ok"}`, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldScrubExecutionLogPayload(tc.input, tc.output); got != tc.want {
				t.Fatalf("unexpected scrub decision for %s: got=%v want=%v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRunExecutionLogPIIScrub(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 3, 3, 0, 0, 0, time.UTC)
	store := &fakeExecutionLogPIIScrubStore{
		records: []ExecutionLogRecord{
			{ID: "row_1", InputPayload: `{"email":"ceo@example.com"}`, OutputPayload: `{"ok":true}`},
			{ID: "row_2", InputPayload: `{"request":"summarize meeting notes"}`, OutputPayload: `{"ok":true}`},
			{ID: "row_3", InputPayload: `{"user":"x"}`, OutputPayload: `{"phone":"+14155551212"}`},
		},
	}

	result, err := RunExecutionLogPIIScrub(context.Background(), store, now, 100)
	if err != nil {
		t.Fatalf("run scrub: %v", err)
	}
	if result.Evaluated != 3 || result.Scrubbed != 2 || result.Skipped != 1 {
		t.Fatalf("unexpected scrub result: %+v", result)
	}
	if !slices.Equal(result.CandidateIDs, []string{"row_1", "row_3"}) {
		t.Fatalf("unexpected scrub candidates: %v", result.CandidateIDs)
	}
	if store.nullifyCallCount != 1 {
		t.Fatalf("expected one nullify call, got %d", store.nullifyCallCount)
	}
	if store.scrubbedReason != ExecutionLogPIIScrubReason {
		t.Fatalf("unexpected scrub reason: %s", store.scrubbedReason)
	}
	if !store.scrubbedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected scrub timestamp: %s", store.scrubbedAt)
	}
	if store.receivedLimit != 100 {
		t.Fatalf("unexpected received limit: %d", store.receivedLimit)
	}
	if !store.receivedBefore.Equal(now.Add(-30 * 24 * time.Hour)) {
		t.Fatalf("unexpected before threshold: %s", store.receivedBefore)
	}
}

func TestRunExecutionLogPIIScrubNoCandidatesSkipsNullify(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 3, 3, 0, 0, 0, time.UTC)
	store := &fakeExecutionLogPIIScrubStore{
		records: []ExecutionLogRecord{
			{ID: "row_1", InputPayload: `{"topic":"ops review"}`, OutputPayload: `{"status":"ok"}`},
		},
	}
	result, err := RunExecutionLogPIIScrub(context.Background(), store, now, 0)
	if err != nil {
		t.Fatalf("run scrub: %v", err)
	}
	if result.Scrubbed != 0 || len(result.CandidateIDs) != 0 {
		t.Fatalf("unexpected scrub output: %+v", result)
	}
	if store.nullifyCallCount != 0 {
		t.Fatalf("expected no nullify calls when no candidates, got %d", store.nullifyCallCount)
	}
	if store.receivedLimit != DefaultExecutionLogPIIScrubBatchSize {
		t.Fatalf("expected default limit %d, got %d", DefaultExecutionLogPIIScrubBatchSize, store.receivedLimit)
	}
}
