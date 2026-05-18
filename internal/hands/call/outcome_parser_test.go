package call

import (
	"testing"
)

func TestParseOutcomeReservationSuccess(t *testing.T) {
	t.Parallel()

	transcript := "Hi, I'd like to make a reservation for a party of 4 on Friday. " +
		"Sure, we have availability. Your reservation is confirmed for Friday at 7pm."

	outcome := ParseOutcome(transcript, "reservation")
	if !outcome.Success {
		t.Fatal("expected successful reservation")
	}
	if outcome.Summary != "Reservation confirmed" {
		t.Fatalf("expected 'Reservation confirmed', got %s", outcome.Summary)
	}
	if outcome.Details["party_size"] != "4" {
		t.Fatalf("expected party_size 4, got %v", outcome.Details["party_size"])
	}
	if outcome.Details["day_mentioned"] != "friday" {
		t.Fatalf("expected friday, got %v", outcome.Details["day_mentioned"])
	}
}

func TestParseOutcomeReservationFailure(t *testing.T) {
	t.Parallel()

	transcript := "I'm sorry, we're fully booked for that evening. Unfortunately we cannot accommodate your party."

	outcome := ParseOutcome(transcript, "reservation")
	if outcome.Success {
		t.Fatal("expected failed reservation")
	}
	if !outcome.FollowUpRequired {
		t.Fatal("expected follow up required on failure")
	}
	if outcome.FollowUpAction == "" {
		t.Fatal("expected follow up action to be set")
	}
}

func TestParseOutcomeAppointment(t *testing.T) {
	t.Parallel()

	transcript := "Your appointment is set for Monday at 2pm. Please arrive 15 minutes early."

	outcome := ParseOutcome(transcript, "appointment")
	if !outcome.Success {
		t.Fatal("expected successful appointment")
	}
	if outcome.Summary != "Appointment scheduled" {
		t.Fatalf("expected 'Appointment scheduled', got %s", outcome.Summary)
	}
}

func TestParseOutcomeQuote(t *testing.T) {
	t.Parallel()

	transcript := "The price for the service is $500 dollars and that is all set and confirmed. The turnaround timeline is 5 business days."

	outcome := ParseOutcome(transcript, "quote")
	if outcome.Summary != "Quote received" {
		t.Fatalf("expected 'Quote received', got %s", outcome.Summary)
	}
	if _, ok := outcome.Details["price_context"]; !ok {
		t.Fatal("expected price_context in details")
	}
	if outcome.Details["has_timeline"] != true {
		t.Fatal("expected has_timeline to be true")
	}
}

func TestParseOutcomeDefault(t *testing.T) {
	t.Parallel()

	transcript := "This is a general conversation about various topics. It has some interesting content and discussion points."

	outcome := ParseOutcome(transcript, "custom")
	if outcome.Summary == "" {
		t.Fatal("expected non-empty summary for default type")
	}
}
