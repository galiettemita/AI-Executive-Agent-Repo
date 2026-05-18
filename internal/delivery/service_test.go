package delivery

import (
	"testing"
)

func TestSendDelivery(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()
	dr, err := svc.Send("ws1", "ch1", "recipient-1", "Hello World")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dr.ID == "" {
		t.Fatal("expected non-empty delivery ID")
	}
	if dr.Status != "pending" {
		t.Fatalf("expected pending status, got %s", dr.Status)
	}
	if dr.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", dr.Attempts)
	}
}

func TestSendDeliveryValidation(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()

	_, err := svc.Send("ws1", "ch1", "r1", "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}

	_, err = svc.Send("ws1", "ch1", "", "content")
	if err == nil {
		t.Fatal("expected error for empty recipientID")
	}
}

func TestMarkDelivered(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()
	dr, _ := svc.Send("ws1", "ch1", "r1", "msg")

	err := svc.MarkDelivered(dr.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, _ := svc.GetStatus(dr.ID)
	if got.Status != "delivered" {
		t.Fatalf("expected delivered status, got %s", got.Status)
	}
	if got.DeliveredAt == nil {
		t.Fatal("expected deliveredAt to be set")
	}

	err = svc.MarkDelivered("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent delivery")
	}
}

func TestRetryDelivery(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()
	dr, _ := svc.Send("ws1", "ch1", "r1", "msg")

	err := svc.Retry(dr.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, _ := svc.GetStatus(dr.ID)
	if got.Status != "retrying" {
		t.Fatalf("expected retrying status, got %s", got.Status)
	}
	if got.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", got.Attempts)
	}

	// Exhaust retries.
	svc.Retry(dr.ID) // 3rd attempt
	err = svc.Retry(dr.ID)
	if err == nil {
		t.Fatal("expected error when exceeding max retries")
	}

	// Cannot retry delivered.
	dr2, _ := svc.Send("ws1", "ch1", "r1", "msg2")
	svc.MarkDelivered(dr2.ID)
	err = svc.Retry(dr2.ID)
	if err == nil {
		t.Fatal("expected error when retrying delivered message")
	}
}

func TestMarkFailed(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()
	dr, _ := svc.Send("ws1", "ch1", "r1", "msg")

	err := svc.MarkFailed(dr.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, _ := svc.GetStatus(dr.ID)
	if got.Status != "failed" {
		t.Fatalf("expected failed status, got %s", got.Status)
	}
}

func TestListPending(t *testing.T) {
	t.Parallel()

	svc := NewDeliveryService()
	svc.Send("ws1", "ch1", "r1", "msg1")
	dr2, _ := svc.Send("ws1", "ch1", "r2", "msg2")
	svc.Send("ws2", "ch1", "r3", "msg3")

	svc.MarkDelivered(dr2.ID)

	pending := svc.ListPending("ws1")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending delivery for ws1, got %d", len(pending))
	}
}
