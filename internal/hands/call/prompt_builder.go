package call

import (
	"fmt"
	"strings"
)

// PromptBuilder constructs system prompts for different call types.
type PromptBuilder struct{}

// NewPromptBuilder returns a new PromptBuilder.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildPrompt generates a system prompt based on call type and context.
func (pb *PromptBuilder) BuildPrompt(callType string, context map[string]any) string {
	switch callType {
	case "reservation":
		return pb.buildReservationPrompt(context)
	case "appointment":
		return pb.buildAppointmentPrompt(context)
	case "quote":
		return pb.buildQuotePrompt(context)
	case "custom":
		return pb.buildCustomPrompt(context)
	default:
		return pb.buildDefaultPrompt(context)
	}
}

func (pb *PromptBuilder) buildReservationPrompt(ctx map[string]any) string {
	var b strings.Builder
	b.WriteString("You are a polite and professional assistant making a restaurant reservation on behalf of a user. ")
	b.WriteString("Be courteous, confirm details, and thank the staff.\n\n")

	if name, ok := ctx["restaurant_name"].(string); ok {
		fmt.Fprintf(&b, "Restaurant: %s\n", name)
	}
	if size, ok := ctx["party_size"]; ok {
		fmt.Fprintf(&b, "Party size: %v\n", size)
	}
	if date, ok := ctx["date"].(string); ok {
		fmt.Fprintf(&b, "Preferred date: %s\n", date)
	}
	if t, ok := ctx["time"].(string); ok {
		fmt.Fprintf(&b, "Preferred time: %s\n", t)
	}
	if req, ok := ctx["special_requests"].(string); ok && req != "" {
		fmt.Fprintf(&b, "Special requests: %s\n", req)
	}

	b.WriteString("\nGoal: Secure a reservation with these details. If the exact time is unavailable, ask for the closest alternative. ")
	b.WriteString("Confirm the final reservation details before ending the call.")
	return b.String()
}

func (pb *PromptBuilder) buildAppointmentPrompt(ctx map[string]any) string {
	var b strings.Builder
	b.WriteString("You are a professional assistant scheduling an appointment on behalf of a user. ")
	b.WriteString("Be clear about requirements and confirm all details.\n\n")

	if provider, ok := ctx["provider"].(string); ok {
		fmt.Fprintf(&b, "Provider/Office: %s\n", provider)
	}
	if service, ok := ctx["service"].(string); ok {
		fmt.Fprintf(&b, "Service needed: %s\n", service)
	}
	if dates, ok := ctx["preferred_dates"].(string); ok {
		fmt.Fprintf(&b, "Preferred dates: %s\n", dates)
	}

	b.WriteString("\nGoal: Schedule the appointment for the preferred dates. If unavailable, find the nearest alternative. ")
	b.WriteString("Confirm date, time, and any preparation instructions before ending.")
	return b.String()
}

func (pb *PromptBuilder) buildQuotePrompt(ctx map[string]any) string {
	var b strings.Builder
	b.WriteString("You are a professional assistant requesting a price quote on behalf of a user. ")
	b.WriteString("Be specific about requirements and ask for timeline and pricing details.\n\n")

	if desc, ok := ctx["service_description"].(string); ok {
		fmt.Fprintf(&b, "Service description: %s\n", desc)
	}
	if reqs, ok := ctx["requirements"].(string); ok {
		fmt.Fprintf(&b, "Requirements: %s\n", reqs)
	}

	b.WriteString("\nGoal: Obtain a clear price quote including timeline and any conditions. ")
	b.WriteString("Ask about discounts or package options if available. Confirm all details.")
	return b.String()
}

func (pb *PromptBuilder) buildCustomPrompt(ctx map[string]any) string {
	if prompt, ok := ctx["prompt"].(string); ok {
		return prompt
	}
	return "You are a helpful phone assistant. Follow the user's instructions and be professional."
}

func (pb *PromptBuilder) buildDefaultPrompt(_ map[string]any) string {
	return "You are a helpful and professional phone assistant. Be courteous, concise, and confirm important details."
}
