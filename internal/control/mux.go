package control

import (
	"encoding/json"
	"net/http"
)

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.NotFound(w, r)
			return
		}

		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusAccepted
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "accepted",
			"service": "control",
			"path":    r.URL.Path,
			"method":  r.Method,
		})
	})

	_ = service
	return mux
}
