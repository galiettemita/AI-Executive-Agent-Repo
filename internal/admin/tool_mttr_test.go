package admin

import (
	"testing"
	"time"
)

func TestToolMTTRRecordIncident(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	log, err := svc.RecordIncident("ws1", "stripe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.ID == "" {
		t.Fatal("expected generated ID")
	}
	if log.IncidentEnd != nil {
		t.Fatal("expected nil IncidentEnd for open incident")
	}
}

func TestToolMTTRRecordIncidentMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	_, err := svc.RecordIncident("", "stripe")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.RecordIncident("ws1", "")
	if err == nil {
		t.Fatal("expected error for missing tool_id")
	}
}

func TestToolMTTRDoubleIncident(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	_, _ = svc.RecordIncident("ws1", "stripe")
	_, err := svc.RecordIncident("ws1", "stripe")
	if err == nil {
		t.Fatal("expected error for duplicate open incident")
	}
}

func TestToolMTTRResolveIncident(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	start := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return start }

	_, _ = svc.RecordIncident("ws1", "stripe")

	end := start.Add(5 * time.Minute)
	svc.now = func() time.Time { return end }

	resolved, err := svc.ResolveIncident("ws1", "stripe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.IncidentEnd == nil {
		t.Fatal("expected non-nil IncidentEnd")
	}
	expectedMs := int64(5 * 60 * 1000)
	if resolved.ResolutionDurationMs != expectedMs {
		t.Fatalf("expected %d ms, got %d ms", expectedMs, resolved.ResolutionDurationMs)
	}
}

func TestToolMTTRResolveNotFound(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	_, err := svc.ResolveIncident("ws1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for no open incident")
	}
}

func TestToolMTTRGetMTTR(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	t0 := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)

	// Incident 1: 2 minutes
	svc.now = func() time.Time { return t0 }
	_, _ = svc.RecordIncident("ws1", "stripe")
	svc.now = func() time.Time { return t0.Add(2 * time.Minute) }
	_, _ = svc.ResolveIncident("ws1", "stripe")

	// Incident 2: 4 minutes
	svc.now = func() time.Time { return t0.Add(10 * time.Minute) }
	_, _ = svc.RecordIncident("ws1", "stripe")
	svc.now = func() time.Time { return t0.Add(14 * time.Minute) }
	_, _ = svc.ResolveIncident("ws1", "stripe")

	mttr := svc.GetMTTR("ws1", "stripe")
	// Average of 2min and 4min = 3min = 180000ms
	expectedMs := int64(180000)
	if mttr != expectedMs {
		t.Fatalf("expected MTTR %d ms, got %d ms", expectedMs, mttr)
	}
}

func TestToolMTTRGetMTTREmpty(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	mttr := svc.GetMTTR("ws1", "nonexistent")
	if mttr != 0 {
		t.Fatalf("expected 0 MTTR, got %d", mttr)
	}
}

func TestToolMTTRGetHistory(t *testing.T) {
	t.Parallel()
	svc := NewToolMTTRService()
	t0 := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)

	svc.now = func() time.Time { return t0 }
	_, _ = svc.RecordIncident("ws1", "stripe")
	svc.now = func() time.Time { return t0.Add(2 * time.Minute) }
	_, _ = svc.ResolveIncident("ws1", "stripe")

	svc.now = func() time.Time { return t0.Add(10 * time.Minute) }
	_, _ = svc.RecordIncident("ws1", "stripe")

	history := svc.GetToolMTTRHistory("ws1", "stripe")
	if len(history) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(history))
	}
	// First should be earlier
	if !history[0].IncidentStart.Before(history[1].IncidentStart) {
		t.Fatal("expected sorted by start time")
	}
}
