package canvas

import (
	"strings"
	"time"
)

type MessageType string

const (
	MessageCanvasInit          MessageType = "canvas.init"
	MessageCanvasSurfacePush   MessageType = "canvas.surface.push"
	MessageCanvasSurfaceUpdate MessageType = "canvas.surface.update"
	MessageCanvasInteraction   MessageType = "canvas.interaction"
	MessageCanvasAck           MessageType = "canvas.ack"
	MessageCanvasError         MessageType = "canvas.error"
	MessageCanvasPing          MessageType = "canvas.ping"
	MessageCanvasPong          MessageType = "canvas.pong"
)

func AllowedMessageTypes() []MessageType {
	return []MessageType{
		MessageCanvasInit,
		MessageCanvasSurfacePush,
		MessageCanvasSurfaceUpdate,
		MessageCanvasInteraction,
		MessageCanvasAck,
		MessageCanvasError,
		MessageCanvasPing,
		MessageCanvasPong,
	}
}

func CanvasSurfaceTypes() []string {
	return []string{
		"approval_card",
		"activity_feed",
		"form",
		"status_tracker",
		"trust_receipt",
		"data_table",
	}
}

func CanvasInteractionRateLimitPerMinute() int {
	return 60
}

func CanvasKeepaliveInterval() time.Duration {
	return 30 * time.Second
}

func IsValidCanvasMessageType(messageType string) bool {
	messageType = strings.TrimSpace(messageType)
	for _, allowed := range AllowedMessageTypes() {
		if string(allowed) == messageType {
			return true
		}
	}
	return false
}
