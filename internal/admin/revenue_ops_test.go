package admin

import (
	"testing"
	"time"
)

func TestRecordSubscriptionEventHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	evt, err := svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID:   "ws1",
		UserID:        "u1",
		EventType:     "new",
		Amount:        99.0,
		StripeEventID: "evt_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID == "" {
		t.Fatal("expected generated ID")
	}
	if evt.Currency != "USD" {
		t.Fatalf("expected USD currency default, got %s", evt.Currency)
	}
}

func TestRecordSubscriptionEventMissingWorkspace(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, err := svc.RecordSubscriptionEvent(SubscriptionEvent{EventType: "new"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestRecordSubscriptionEventMissingEventType(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, err := svc.RecordSubscriptionEvent(SubscriptionEvent{WorkspaceID: "ws1"})
	if err == nil {
		t.Fatal("expected error for missing event_type")
	}
}

func TestComputeMRRSnapshot(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, _ = svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1",
		EventType:   "new",
		Amount:      100.0,
	})

	snap, err := svc.ComputeMRRSnapshot("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.MRR != 100.0 {
		t.Fatalf("expected MRR 100.0, got %f", snap.MRR)
	}
	if snap.ARR != 1200.0 {
		t.Fatalf("expected ARR 1200.0, got %f", snap.ARR)
	}
}

func TestComputeMRRSnapshotMissingWorkspace(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, err := svc.ComputeMRRSnapshot("")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestGetMRRAfterChurn(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, _ = svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1",
		EventType:   "new",
		Amount:      100.0,
	})
	_, _ = svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1",
		EventType:   "churn",
		Amount:      0,
	})

	mrr := svc.GetMRR("ws1")
	if mrr != 0 {
		t.Fatalf("expected MRR 0 after churn, got %f", mrr)
	}
}

func TestCohortRetentionTracking(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	week := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

	svc.ComputeCohortRetention("ws1", "u1", week, 4, true)
	svc.ComputeCohortRetention("ws1", "u2", week, 4, false)

	cohorts := svc.GetCohorts("ws1")
	if len(cohorts) != 2 {
		t.Fatalf("expected 2 cohort entries, got %d", len(cohorts))
	}

	active := 0
	for _, c := range cohorts {
		if c.IsActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("expected 1 active, got %d", active)
	}
}

func TestComputeLTV(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	week := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

	_, _ = svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1",
		EventType:   "new",
		Amount:      100.0,
	})
	svc.ComputeCohortRetention("ws1", "u1", week, 10, true)

	ltv := svc.ComputeLTV("ws1")
	// avgRevenue=100, avgRetention=10 => LTV=1000
	if ltv != 1000.0 {
		t.Fatalf("expected LTV 1000.0, got %f", ltv)
	}
}

func TestComputeLTVNoEvents(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	ltv := svc.ComputeLTV("ws1")
	if ltv != 0 {
		t.Fatalf("expected LTV 0 with no events, got %f", ltv)
	}
}

func TestGetMarginReport(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	_, _ = svc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1",
		EventType:   "new",
		Amount:      1000.0,
	})
	svc.SetCOGS("ws1", 300.0)

	report := svc.GetMarginReport("ws1")
	if report.Revenue != 1000.0 {
		t.Fatalf("expected revenue 1000.0, got %f", report.Revenue)
	}
	if report.COGS != 300.0 {
		t.Fatalf("expected COGS 300.0, got %f", report.COGS)
	}
	if report.Margin != 700.0 {
		t.Fatalf("expected margin 700.0, got %f", report.Margin)
	}
	if report.MarginPercent != 70.0 {
		t.Fatalf("expected margin%% 70.0, got %f", report.MarginPercent)
	}
}

func TestGetMarginReportZeroRevenue(t *testing.T) {
	t.Parallel()
	svc := NewRevenueOpsService()
	report := svc.GetMarginReport("ws1")
	if report.MarginPercent != 0 {
		t.Fatalf("expected 0 margin percent, got %f", report.MarginPercent)
	}
}
