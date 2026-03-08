package billing

import "testing"

func TestDefaultPlans(t *testing.T) {
	t.Parallel()

	plans := DefaultPlans()
	if len(plans) != 4 {
		t.Fatalf("expected 4 plans, got %d", len(plans))
	}
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	sub, err := s.Subscribe("ws1", "plan_pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Status != "active" {
		t.Fatalf("expected active status, got %s", sub.Status)
	}
	if sub.PlanID != "plan_pro" {
		t.Fatalf("expected plan_pro, got %s", sub.PlanID)
	}
}

func TestSubscribeInvalidPlan(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, err := s.Subscribe("ws1", "plan_nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid plan")
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	_, err := s.Subscribe("ws1", "plan_business")
	if err == nil {
		t.Fatal("expected error for duplicate subscription")
	}
}

func TestChangePlan(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_free")
	sub, err := s.ChangePlan("ws1", "plan_pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.PlanID != "plan_pro" {
		t.Fatalf("expected plan_pro, got %s", sub.PlanID)
	}
}

func TestChangePlanNoSubscription(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, err := s.ChangePlan("ws_none", "plan_pro")
	if err == nil {
		t.Fatal("expected error for missing subscription")
	}
}

func TestCancelSubscription(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	if err := s.CancelSubscription("ws1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := s.GetSubscription("ws1")
	if sub.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", sub.Status)
	}
}

func TestCancelAlreadyCancelled(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	_ = s.CancelSubscription("ws1")
	err := s.CancelSubscription("ws1")
	if err == nil {
		t.Fatal("expected error cancelling already cancelled subscription")
	}
}

func TestHandleStripeWebhookInvoicePaid(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	err := s.HandleStripeWebhook("invoice.paid", map[string]any{"workspace_id": "ws1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := s.GetSubscription("ws1")
	if sub.Status != "active" {
		t.Fatalf("expected active status after payment, got %s", sub.Status)
	}
}

func TestHandleStripeWebhookPaymentFailed(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	err := s.HandleStripeWebhook("invoice.payment_failed", map[string]any{"workspace_id": "ws1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := s.GetSubscription("ws1")
	if sub.Status != "past_due" {
		t.Fatalf("expected past_due status, got %s", sub.Status)
	}
}

func TestHandleStripeWebhookSubscriptionDeleted(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	_, _ = s.Subscribe("ws1", "plan_pro")
	err := s.HandleStripeWebhook("customer.subscription.deleted", map[string]any{"workspace_id": "ws1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := s.GetSubscription("ws1")
	if sub.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", sub.Status)
	}
}

func TestHandleStripeWebhookUnknownEvent(t *testing.T) {
	t.Parallel()
	s := NewBillingService()

	err := s.HandleStripeWebhook("unknown.event", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
}
