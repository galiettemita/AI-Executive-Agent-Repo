package gateway

import "net/http"

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/gateway/webhook/whatsapp", service.HandleWhatsAppVerification)
	mux.HandleFunc("POST /v1/gateway/webhook/whatsapp", service.HandleInbound)
	mux.HandleFunc("POST /v1/gateway/webhook/imessage", service.HandleInbound)
	mux.HandleFunc("POST /v1/gateway/outbound/send", service.HandleOutboundSend)
	return mux
}
