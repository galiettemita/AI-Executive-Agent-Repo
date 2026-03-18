package learning

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestAnchorProtection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewLessonAnchorManager(nil, nil, logger)

	// Without DB, ProtectAnchor allows modification (test mode).
	allowed, err := manager.ProtectAnchor(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ProtectAnchor failed: %v", err)
	}
	if !allowed {
		t.Error("Expected allowed=true in test mode (no DB)")
	}
}

func TestAnchorPromotionEligibilityNoDb(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewLessonAnchorManager(nil, nil, logger)

	eligible, err := manager.EvaluateAnchorEligibility(context.Background())
	if err != nil {
		t.Fatalf("EvaluateAnchorEligibility failed: %v", err)
	}
	if eligible != nil {
		t.Error("Expected nil eligible list without DB")
	}
}

func TestRunAnchorPromotionNoDb(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewLessonAnchorManager(nil, nil, logger)

	err := manager.RunAnchorPromotion(context.Background())
	if err != nil {
		t.Fatalf("RunAnchorPromotion failed: %v", err)
	}
}

func TestErrAnchorProtectedMessage(t *testing.T) {
	if ErrAnchorProtected.Error() != "lesson is anchored and protected from modification" {
		t.Errorf("Unexpected error message: %s", ErrAnchorProtected.Error())
	}
}

func TestReplayBufferNoDb(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	buf := NewStratifiedReplayBuffer(nil, logger)

	pairs, err := buf.SampleReplayBatch(context.Background(), uuid.New(), ReplayConfig{
		TotalBatchSize: 100,
		ReplayFraction: 0.10,
		Domains:        []string{"email", "calendar", "tasks"},
	})
	if err != nil {
		t.Fatalf("SampleReplayBatch failed: %v", err)
	}
	if pairs != nil {
		t.Error("Expected nil pairs without DB")
	}
}

func TestReplayConfigCalculation(t *testing.T) {
	config := ReplayConfig{
		TotalBatchSize: 100,
		ReplayFraction: 0.10,
		Domains:        []string{"email", "calendar", "tasks"},
	}

	replayCount := int(float64(config.TotalBatchSize) * config.ReplayFraction)
	if replayCount != 10 {
		t.Errorf("Expected replay count 10, got %d", replayCount)
	}

	samplesPerDomain := replayCount / len(config.Domains)
	if samplesPerDomain != 3 {
		t.Errorf("Expected 3 samples per domain, got %d", samplesPerDomain)
	}
}

func TestForgettingDetectorNoDb(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	detector := NewForgettingDetector(nil, logger)

	err := detector.DetectForgetting(context.Background())
	if err != nil {
		t.Fatalf("DetectForgetting failed: %v", err)
	}
}

func TestDBPreferenceTransferReaderNoDb(t *testing.T) {
	reader := NewDBPreferenceTransferReader(nil)
	count, err := reader.GetWorkspaceTransferCount(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetWorkspaceTransferCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 without DB, got %d", count)
	}
}
