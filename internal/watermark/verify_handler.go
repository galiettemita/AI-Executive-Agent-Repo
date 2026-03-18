package watermark

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// VerifyContentHandler is the HTTP handler for POST /v1/verify/content.
// It checks whether text contains a Brevio watermark and returns provenance info.
type VerifyContentHandler struct {
	c2pa            *C2PAContentWatermarker
	semantic        *SemanticWatermarker
	provenanceStore *ProvenanceStore
	logger          *slog.Logger
}

// NewVerifyContentHandler constructs the handler with all dependencies.
func NewVerifyContentHandler(
	c2pa *C2PAContentWatermarker,
	semantic *SemanticWatermarker,
	store *ProvenanceStore,
	logger *slog.Logger,
) *VerifyContentHandler {
	return &VerifyContentHandler{
		c2pa:            c2pa,
		semantic:        semantic,
		provenanceStore: store,
		logger:          logger,
	}
}

type verifyRequest struct {
	Text string `json:"text"`
}

type verifyResponse struct {
	IsBrevioGenerated bool      `json:"is_brevio_generated"`
	WorkspaceID       *string   `json:"workspace_id"`
	ModelID           *string   `json:"model_id"`
	Confidence        float64   `json:"confidence"`
	VerifiedAt        time.Time `json:"verified_at"`
}

// ServeHTTP handles POST /v1/verify/content requests.
func (h *VerifyContentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONResp(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Text == "" {
		writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	// Check C2PA watermark.
	verification, err := h.c2pa.VerifyWithProvenance(req.Text, h.provenanceStore)
	if err != nil || !verification.IsBrevioGenerated {
		writeJSONResp(w, http.StatusOK, verifyResponse{
			IsBrevioGenerated: false,
			Confidence:        0.0,
			VerifiedAt:        time.Now().UTC(),
		})
		return
	}

	resp := verifyResponse{
		IsBrevioGenerated: true,
		Confidence:        verification.Confidence,
		VerifiedAt:        time.Now().UTC(),
	}

	if verification.WorkspaceID.String() != "00000000-0000-0000-0000-000000000000" {
		wsID := verification.WorkspaceID.String()
		resp.WorkspaceID = &wsID
	}
	if verification.ModelID != "" {
		resp.ModelID = &verification.ModelID
	}

	// Boost confidence with semantic detection if provenance data is available.
	if h.semantic != nil && verification.RequestID.String() != "00000000-0000-0000-0000-000000000000" {
		detected, semConf := h.semantic.Detect(req.Text, verification.WorkspaceID, verification.RequestID)
		if detected {
			resp.Confidence = (resp.Confidence + semConf) / 2.0
		}
	}

	writeJSONResp(w, http.StatusOK, resp)
}

func writeJSONResp(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// IPRateLimiter is a simple per-IP rate limiter (100 req/min).
type IPRateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateBucket
	limit    int
	window   time.Duration
}

type rateBucket struct {
	count    int
	resetAt  time.Time
}

// NewIPRateLimiter creates a rate limiter with the given limit per window.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		counters: make(map[string]*rateBucket),
		limit:    limit,
		window:   window,
	}
}

// Middleware wraps an http.Handler with per-IP rate limiting.
func (rl *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		rl.mu.Lock()
		bucket, exists := rl.counters[ip]
		now := time.Now()
		if !exists || now.After(bucket.resetAt) {
			bucket = &rateBucket{count: 0, resetAt: now.Add(rl.window)}
			rl.counters[ip] = bucket
		}
		bucket.count++
		allowed := bucket.count <= rl.limit
		rl.mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", "60")
			writeJSONResp(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
