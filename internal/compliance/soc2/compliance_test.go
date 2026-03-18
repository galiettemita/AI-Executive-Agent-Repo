package soc2

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestCC61CollectorPass(t *testing.T) {
	collector := NewComplianceEvidenceCollector(nil, testLogger)
	ev, err := collector.CollectCC61(context.Background())
	if err != nil {
		t.Fatalf("CollectCC61 failed: %v", err)
	}
	if ev == nil {
		t.Fatal("Expected non-nil evidence")
	}
	if ev.ControlID != ControlCC61 {
		t.Errorf("Expected control_id=%s, got %s", ControlCC61, ev.ControlID)
	}
	if ev.Framework != FrameworkSOC2 {
		t.Errorf("Expected framework=%s, got %s", FrameworkSOC2, ev.Framework)
	}
	t.Logf("CC6.1 pass=%v details=%v", ev.Pass, ev.Details)
}

func TestCC72AnomalyDetection(t *testing.T) {
	collector := NewComplianceEvidenceCollector(nil, testLogger)
	ev, err := collector.CollectCC72(context.Background())
	if err != nil {
		t.Fatalf("CollectCC72 failed: %v", err)
	}
	if ev.ControlID != ControlCC72 {
		t.Errorf("Expected control_id=%s, got %s", ControlCC72, ev.ControlID)
	}
	if !ev.Pass {
		t.Errorf("Expected CC7.2 to pass without DB (no anomalies), got pass=%v", ev.Pass)
	}
}

func TestPI14ProcessingIntegrity(t *testing.T) {
	collector := NewComplianceEvidenceCollector(nil, testLogger)
	ev, err := collector.CollectPI14(context.Background())
	if err != nil {
		t.Fatalf("CollectPI14 failed: %v", err)
	}
	if ev.ControlID != ControlPI14 {
		t.Errorf("Expected control_id=%s, got %s", ControlPI14, ev.ControlID)
	}
	t.Logf("PI1.4 pass=%v details=%v", ev.Pass, ev.Details)
}

func TestSOC2PDFGeneration(t *testing.T) {
	gen := NewSOC2ReportGenerator(nil, testLogger)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	pdf, err := gen.GenerateReport(context.Background(), start, end)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("Expected non-empty PDF bytes")
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Errorf("Expected PDF to start with %%PDF header, got first 20 bytes: %q", string(pdf[:min(20, len(pdf))]))
	}
	t.Logf("PDF generated: %d bytes", len(pdf))
}

func TestTruncateFunction(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != short {
		t.Errorf("Expected no truncation for short string")
	}

	long := strings.Repeat("x", 100)
	result := truncate(long, 50)
	if !strings.HasSuffix(result, "...[truncated]") {
		t.Errorf("Expected truncated suffix, got %q", result)
	}
}
