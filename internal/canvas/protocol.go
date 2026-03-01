package canvas

func SupportedMessageTypes() []string {
	return []string{
		"canvas.init",
		"canvas.surface.push",
		"canvas.surface.update",
		"canvas.interaction",
		"canvas.ack",
		"canvas.error",
		"canvas.ping",
		"canvas.pong",
	}
}

func SupportedSurfaceTypes() []string {
	return []string{
		"approval_card",
		"activity_feed",
		"form",
		"status_tracker",
		"trust_receipt",
		"data_table",
	}
}
