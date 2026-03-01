package memory

import (
	"testing"
	"time"
)

func TestShouldMergeDuplicate(t *testing.T) {
	t.Parallel()

	if !ShouldMergeDuplicate(0.93, true, true, 500) {
		t.Fatal("expected duplicate merge at normal threshold")
	}
	if ShouldMergeDuplicate(0.90, true, true, 500) {
		t.Fatal("did not expect merge below normal threshold")
	}
	if !ShouldMergeDuplicate(0.86, true, true, 10001) {
		t.Fatal("expected aggressive merge threshold above 10k items")
	}
}

func TestMergeDuplicateRecordsAndContradictions(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	older := ConsolidationRecord{
		ID:            "old",
		CreatedAt:     now.Add(-1 * time.Hour),
		SourceTurnIDs: []string{"t1"},
		Version:       2,
	}
	newer := ConsolidationRecord{
		ID:            "new",
		CreatedAt:     now,
		SourceTurnIDs: []string{"t2"},
		Version:       3,
	}
	keep, superseded := MergeDuplicateRecords(older, newer)
	if keep.ID != "new" || keep.Version != 4 {
		t.Fatalf("unexpected merge winner/version: %+v", keep)
	}
	if superseded.Status != StatusSuperseded || superseded.SupersededReason != "duplicate_merge" {
		t.Fatalf("unexpected superseded state: %+v", superseded)
	}

	keep, superseded = ResolveContradiction(older, newer)
	if keep.ID != "new" || superseded.SupersededReason != "contradiction" {
		t.Fatalf("unexpected contradiction resolution: keep=%+v superseded=%+v", keep, superseded)
	}
}

func TestSupersedeByStalenessAndConfidence(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	if !ShouldSupersedeByStaleness(now.Add(-91*24*time.Hour), now) {
		t.Fatal("expected stale supersede")
	}
	if ShouldSupersedeByStaleness(now.Add(-10*24*time.Hour), now) {
		t.Fatal("did not expect fresh item supersede")
	}
	if !ShouldSupersedeByConsolidationConfidence(0.29, 1) {
		t.Fatal("expected low-confidence supersede")
	}
	if !ShouldSupersedeByConsolidationConfidence(0.49, 10001) {
		t.Fatal("expected aggressive confidence supersede for large workspaces")
	}
}
