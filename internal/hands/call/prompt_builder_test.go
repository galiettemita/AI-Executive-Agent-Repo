package call

import (
	"strings"
	"testing"
)

func TestBuildReservationPrompt(t *testing.T) {
	t.Parallel()

	pb := NewPromptBuilder()
	ctx := map[string]any{
		"restaurant_name": "Chez Michel",
		"party_size":      4,
		"date":            "Friday",
		"time":            "7pm",
	}

	prompt := pb.BuildPrompt("reservation", ctx)
	if !strings.Contains(prompt, "restaurant reservation") {
		t.Fatal("expected reservation context in prompt")
	}
	if !strings.Contains(prompt, "Chez Michel") {
		t.Fatal("expected restaurant name in prompt")
	}
	if !strings.Contains(prompt, "Friday") {
		t.Fatal("expected date in prompt")
	}
}

func TestBuildAppointmentPrompt(t *testing.T) {
	t.Parallel()

	pb := NewPromptBuilder()
	ctx := map[string]any{
		"provider": "Dr. Smith",
		"service":  "dental checkup",
	}

	prompt := pb.BuildPrompt("appointment", ctx)
	if !strings.Contains(prompt, "scheduling an appointment") {
		t.Fatal("expected appointment context in prompt")
	}
	if !strings.Contains(prompt, "Dr. Smith") {
		t.Fatal("expected provider name in prompt")
	}
}

func TestBuildQuotePrompt(t *testing.T) {
	t.Parallel()

	pb := NewPromptBuilder()
	ctx := map[string]any{
		"service_description": "roof repair",
		"requirements":        "emergency repair needed",
	}

	prompt := pb.BuildPrompt("quote", ctx)
	if !strings.Contains(prompt, "price quote") {
		t.Fatal("expected quote context in prompt")
	}
	if !strings.Contains(prompt, "roof repair") {
		t.Fatal("expected service description in prompt")
	}
}

func TestBuildCustomPrompt(t *testing.T) {
	t.Parallel()

	pb := NewPromptBuilder()
	ctx := map[string]any{
		"prompt": "You are a custom assistant doing specific things.",
	}

	prompt := pb.BuildPrompt("custom", ctx)
	if prompt != "You are a custom assistant doing specific things." {
		t.Fatalf("expected custom prompt to be used directly, got %s", prompt)
	}

	// Custom without prompt key falls back to default.
	prompt = pb.BuildPrompt("custom", nil)
	if !strings.Contains(prompt, "helpful phone assistant") {
		t.Fatal("expected fallback custom prompt")
	}
}

func TestBuildDefaultPrompt(t *testing.T) {
	t.Parallel()

	pb := NewPromptBuilder()
	prompt := pb.BuildPrompt("unknown_type", nil)
	if !strings.Contains(prompt, "professional phone assistant") {
		t.Fatal("expected default prompt for unknown type")
	}
}
