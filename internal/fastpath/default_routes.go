package fastpath

// DefaultRoutes returns the 5 pre-seeded fast-path routes for common greetings and
// status queries. These handle the highest-volume, zero-risk intents at T0 latency.
func DefaultRoutes() []struct {
	Pattern    string
	Response   string
	Confidence float64
} {
	return []struct {
		Pattern    string
		Response   string
		Confidence float64
	}{
		{
			Pattern:    `^(hi|hello|hey|good morning|good afternoon|good evening)[!.?\s]*$`,
			Response:   "Hey! What can I help you with today?",
			Confidence: 0.99,
		},
		{
			Pattern:    `^(what can you do|what do you do|help|what are your capabilities)[?\s]*$`,
			Response:   "I can manage your calendar, emails, travel bookings, and more. What would you like me to do?",
			Confidence: 0.98,
		},
		{
			Pattern:    `^(are you there|you there|hello\?)[\s]*$`,
			Response:   "Yes, I'm here! What do you need?",
			Confidence: 0.99,
		},
		{
			Pattern:    `^(thank you|thanks|cheers|thx)[!.?\s]*$`,
			Response:   "You're welcome! Let me know if there's anything else.",
			Confidence: 0.99,
		},
		{
			Pattern:    `^(ok|okay|got it|understood|sure|sounds good)[!.?\s]*$`,
			Response:   "Got it! Anything else?",
			Confidence: 0.97,
		},
	}
}

// SeedDefaultRoutes registers the 5 default routes into the given FastPathService.
func SeedDefaultRoutes(svc *FastPathService) error {
	for _, r := range DefaultRoutes() {
		if _, err := svc.RegisterRoute(r.Pattern, r.Response, r.Confidence); err != nil {
			return err
		}
	}
	return nil
}
