package gateway

import "net/http"

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/gateway/webhook/whatsapp", service.HandleWhatsAppVerification)
	mux.HandleFunc("POST /v1/gateway/webhook/whatsapp", service.HandleInbound)
	mux.HandleFunc("POST /v1/gateway/webhook/imessage", service.HandleInbound)
	mux.HandleFunc("POST /v1/gateway/outbound/send", service.HandleOutboundSend)
	mux.HandleFunc("POST /v1/gateway/inject/tool_call", service.HandleInjectToolCall)
	mux.HandleFunc("GET /health", service.HandleHealth)
	mux.HandleFunc("GET /health/deep", service.HandleHealth)
	mux.HandleFunc("GET /healthz/ready", service.HandleHealth)
	mux.HandleFunc("GET /healthz/live", service.HandleHealth)
	return mux
}
