package memory

import (
	"slices"
	"time"
)

type ConsolidationRecord struct {
	ID               string
	CreatedAt        time.Time
	LastAccessedAt   time.Time
	Confidence       float64
	SourceTurnIDs    []string
	Version          int
	Status           string
	SupersededReason string
}

func ShouldMergeDuplicate(similarity float64, sameWorkspace bool, sameMemoryType bool, activeItems int) bool {
	if !sameWorkspace || !sameMemoryType {
		return false
	}
	threshold := 0.92
	if activeItems > 10000 {
		threshold = 0.85
	}
	return similarity > threshold
}

func MergeDuplicateRecords(left, right ConsolidationRecord) (keep ConsolidationRecord, superseded ConsolidationRecord) {
	keep = left
	superseded = right
	if right.CreatedAt.After(left.CreatedAt) {
		keep, superseded = right, left
	}
	for _, sourceTurnID := range superseded.SourceTurnIDs {
		if !slices.Contains(keep.SourceTurnIDs, sourceTurnID) {
			keep.SourceTurnIDs = append(keep.SourceTurnIDs, sourceTurnID)
		}
	}
	if keep.Version < superseded.Version {
		keep.Version = superseded.Version
	}
	keep.Version++
	superseded.Status = StatusSuperseded
	superseded.SupersededReason = "duplicate_merge"
	return keep, superseded
}

func ShouldSupersedeByStaleness(lastAccessedAt time.Time, now time.Time) bool {
	if lastAccessedAt.IsZero() {
		return false
	}
	return now.UTC().Sub(lastAccessedAt.UTC()) >= 90*24*time.Hour
}

func ShouldSupersedeByConsolidationConfidence(confidence float64, activeItems int) bool {
	threshold := 0.3
	if activeItems > 10000 {
		threshold = 0.5
	}
	return confidence < threshold
}

func ResolveContradiction(left, right ConsolidationRecord) (keep ConsolidationRecord, superseded ConsolidationRecord) {
	keep = left
	superseded = right
	if right.CreatedAt.After(left.CreatedAt) {
		keep, superseded = right, left
	}
	superseded.Status = StatusSuperseded
	superseded.SupersededReason = "contradiction"
	return keep, superseded
}
