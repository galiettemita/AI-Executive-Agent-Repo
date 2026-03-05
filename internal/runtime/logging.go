package runtime

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JSONLogger struct {
	mu          sync.Mutex
	serviceName string
	environment string
	output      io.Writer
	now         func() time.Time
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func NewJSONLogger(serviceName, environment string) *JSONLogger {
	return &JSONLogger{
		serviceName: strings.TrimSpace(serviceName),
		environment: normalizeEnv(environment),
		output:      io.Discard,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (l *JSONLogger) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if output == nil {
		l.output = io.Discard
		return
	}
	l.output = output
}

func (l *JSONLogger) SetNowForTest(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now == nil {
		l.now = func() time.Time { return time.Now().UTC() }
		return
	}
	l.now = now
}

func (l *JSONLogger) Info(event string, attrs map[string]any) {
	l.log(event, "info", "", "", "", attrs)
}

func (l *JSONLogger) Middleware(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := l.nowUTC()
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = nextRequestID()
		}
		w.Header().Set("X-Request-Id", requestID)

		traceID, spanID := parseTraceparent(r.Header.Get("traceparent"))
		userID := strings.TrimSpace(r.Header.Get("X-User-Id"))

		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		next.ServeHTTP(recorder, r)

		l.log("http_request", "info", traceID, spanID, userID, map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": recorder.status,
			"duration_ms": time.Since(start).Milliseconds(),
			"request_id":  requestID,
		})
	})
}

func (l *JSONLogger) log(event, severity, traceID, spanID, userID string, attrs map[string]any) {
	payload := map[string]any{
		"ts":       l.nowUTC().Format(time.RFC3339Nano),
		"service":  l.serviceName,
		"env":      l.environment,
		"trace_id": strings.TrimSpace(traceID),
		"span_id":  strings.TrimSpace(spanID),
		"user_id":  strings.TrimSpace(userID),
		"event":    strings.TrimSpace(event),
		"severity": strings.TrimSpace(severity),
		"attrs":    attrsOrEmpty(attrs),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.output == nil {
		return
	}
	_, _ = l.output.Write(append(encoded, '\n'))
}

func (l *JSONLogger) nowUTC() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.now().UTC()
}

func attrsOrEmpty(attrs map[string]any) map[string]any {
	if attrs == nil {
		return map[string]any{}
	}
	return attrs
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func parseTraceparent(raw string) (string, string) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 4 {
		return "", ""
	}
	traceID := strings.TrimSpace(parts[1])
	spanID := strings.TrimSpace(parts[2])
	if len(traceID) != 32 || len(spanID) != 16 {
		return "", ""
	}
	return traceID, spanID
}

func nextRequestID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return "req-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return id.String()
}
