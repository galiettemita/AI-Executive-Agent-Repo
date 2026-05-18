package knowledge

import (
	"testing"
	"time"
)

func TestCheckDrift_Fresh(t *testing.T) {
	svc := NewKnowledgeDriftService()

	report, err := svc.CheckDrift("ws1", "persona",
		"We handle billing and invoices for clients",
		[]string{"billing question from client"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.StalenessScore > 0.5 {
		t.Fatalf("expected low staleness for fresh content, got %f", report.StalenessScore)
	}
	if report.Refreshed {
		t.Fatal("expected refreshed=false for new report")
	}
}

func TestCheckDrift_MissingTopics(t *testing.T) {
	svc := NewKnowledgeDriftService()

	report, err := svc.CheckDrift("ws1", "faq",
		"We sell software products",
		[]string{
			"customer asking about refund policy",
			"question about shipping costs",
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.SuggestedRefreshTopics) == 0 {
		t.Fatal("expected suggested refresh topics for missing content")
	}
	if report.StalenessScore == 0 {
		t.Fatal("expected non-zero staleness for content gap")
	}
}

func TestCheckDrift_TimeBased(t *testing.T) {
	svc := NewKnowledgeDriftService()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return base }

	// First check sets lastUpdated to now.
	_, _ = svc.CheckDrift("ws1", "policy", "our policy covers everything", nil)

	// Advance 30 days.
	svc.now = func() time.Time { return base.Add(30 * 24 * time.Hour) }
	report, _ := svc.CheckDrift("ws1", "policy", "our policy covers everything", nil)

	if report.StalenessScore < 0.2 {
		t.Fatalf("expected time-based staleness > 0.2, got %f", report.StalenessScore)
	}
}

func TestGetDriftReports(t *testing.T) {
	svc := NewKnowledgeDriftService()

	_, _ = svc.CheckDrift("ws1", "persona", "content", nil)
	_, _ = svc.CheckDrift("ws1", "faq", "content", nil)
	_, _ = svc.CheckDrift("ws2", "policy", "content", nil)

	reports := svc.GetDriftReports("ws1")
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports for ws1, got %d", len(reports))
	}
}

func TestMarkRefreshed(t *testing.T) {
	svc := NewKnowledgeDriftService()

	_, _ = svc.CheckDrift("ws1", "persona", "content", []string{"missing topic info"})

	err := svc.MarkRefreshed("ws1", "persona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reports := svc.GetDriftReports("ws1")
	if len(reports) != 1 {
		t.Fatal("expected 1 report")
	}
	if reports[0].StalenessScore != 0 {
		t.Fatalf("expected staleness 0 after refresh, got %f", reports[0].StalenessScore)
	}
	if !reports[0].Refreshed {
		t.Fatal("expected refreshed=true")
	}

	// Mark nonexistent.
	err = svc.MarkRefreshed("ws1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent file type")
	}
}
