package compliance

import (
	"context"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultExecutionLogPIIScrubBatchSize = 500
	ExecutionLogPIIScrubReason           = "PII_SCRUB_30D_RETENTION"
)

type ExecutionLogRecord struct {
	ID            string
	CreatedAt     time.Time
	InputPayload  string
	OutputPayload string
}

type ExecutionLogPIIScrubStore interface {
	ListExecutionLogsOlderThan(ctx context.Context, before time.Time, limit int) ([]ExecutionLogRecord, error)
	NullifyExecutionLogPayloads(ctx context.Context, ids []string, reason string, scrubbedAt time.Time) error
}

type ExecutionLogPIIScrubResult struct {
	Evaluated    int
	Scrubbed     int
	Skipped      int
	CandidateIDs []string
	BeforeTime   time.Time
	NextRunAt    time.Time
}

var (
	emailPIIPattern = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	phonePIIPattern = regexp.MustCompile(`\+?[0-9][0-9\-\s\(\)]{7,}[0-9]`)
	ssnPIIPattern   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	cardPIIPattern  = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
)

// NextExecutionLogPIIScrubRun returns the next daily 03:00 UTC run time.
func NextExecutionLogPIIScrubRun(now time.Time) time.Time {
	utcNow := now.UTC()
	run := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 3, 0, 0, 0, time.UTC)
	if !utcNow.Before(run) {
		run = run.Add(24 * time.Hour)
	}
	return run
}

// ShouldScrubExecutionLogPayload applies regex-based PII detection to payload text.
// This is the deterministic first-pass classifier before optional NER-based upgrades.
func ShouldScrubExecutionLogPayload(inputPayload, outputPayload string) bool {
	combined := strings.TrimSpace(inputPayload + "\n" + outputPayload)
	if combined == "" {
		return false
	}
	return emailPIIPattern.MatchString(combined) ||
		phonePIIPattern.MatchString(combined) ||
		ssnPIIPattern.MatchString(combined) ||
		cardPIIPattern.MatchString(combined)
}

func RunExecutionLogPIIScrub(ctx context.Context, store ExecutionLogPIIScrubStore, now time.Time, limit int) (ExecutionLogPIIScrubResult, error) {
	if limit <= 0 {
		limit = DefaultExecutionLogPIIScrubBatchSize
	}
	before := now.UTC().Add(-30 * 24 * time.Hour)

	records, err := store.ListExecutionLogsOlderThan(ctx, before, limit)
	if err != nil {
		return ExecutionLogPIIScrubResult{}, err
	}

	candidateIDs := make([]string, 0, len(records))
	skipped := 0
	for _, record := range records {
		if ShouldScrubExecutionLogPayload(record.InputPayload, record.OutputPayload) {
			candidateIDs = append(candidateIDs, record.ID)
			continue
		}
		skipped++
	}

	if len(candidateIDs) > 0 {
		if err := store.NullifyExecutionLogPayloads(ctx, candidateIDs, ExecutionLogPIIScrubReason, now.UTC()); err != nil {
			return ExecutionLogPIIScrubResult{}, err
		}
	}

	return ExecutionLogPIIScrubResult{
		Evaluated:    len(records),
		Scrubbed:     len(candidateIDs),
		Skipped:      skipped,
		CandidateIDs: candidateIDs,
		BeforeTime:   before,
		NextRunAt:    NextExecutionLogPIIScrubRun(now),
	}, nil
}
