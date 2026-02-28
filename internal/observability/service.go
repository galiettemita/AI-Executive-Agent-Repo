package observability

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type LogEntry struct {
	TS            time.Time      `json:"ts"`
	Service       string         `json:"service"`
	Env           string         `json:"env"`
	WorkspaceID   string         `json:"workspace_id"`
	UserID        string         `json:"user_id"`
	IngressTurnID string         `json:"ingress_turn_id"`
	TraceID       string         `json:"trace_id"`
	SpanID        string         `json:"span_id"`
	Event         string         `json:"event"`
	Severity      string         `json:"severity"`
	Attrs         map[string]any `json:"attrs"`
}

func NewLogEntry(service, env, workspaceID, userID, ingressTurnID, traceID, spanID, event, severity string, attrs map[string]any) LogEntry {
	if attrs == nil {
		attrs = map[string]any{}
	}
	return LogEntry{
		TS:            time.Now().UTC(),
		Service:       service,
		Env:           env,
		WorkspaceID:   workspaceID,
		UserID:        userID,
		IngressTurnID: ingressTurnID,
		TraceID:       traceID,
		SpanID:        spanID,
		Event:         event,
		Severity:      severity,
		Attrs:         attrs,
	}
}

func (entry LogEntry) JSON() ([]byte, error) {
	if entry.Attrs == nil {
		entry.Attrs = map[string]any{}
	}
	return json.Marshal(entry)
}

func (entry LogEntry) Validate() error {
	required := map[string]string{
		"service":  entry.Service,
		"env":      entry.Env,
		"trace_id": entry.TraceID,
		"span_id":  entry.SpanID,
		"event":    entry.Event,
		"severity": entry.Severity,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing required log field: %s", field)
		}
	}
	if entry.TS.IsZero() {
		return fmt.Errorf("missing required log field: ts")
	}
	return nil
}

type MetricRegistry struct {
	mu      sync.Mutex
	allowed map[string]struct{}
	values  map[string]float64
}

func NewMetricRegistry(metricNames []string) *MetricRegistry {
	allowed := map[string]struct{}{}
	for _, name := range metricNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	return &MetricRegistry{allowed: allowed, values: map[string]float64{}}
}

func (registry *MetricRegistry) Record(name string, value float64) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, ok := registry.allowed[name]; !ok {
		return fmt.Errorf("metric not registered: %s", name)
	}
	registry.values[name] = value
	return nil
}

func (registry *MetricRegistry) Value(name string) (float64, bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	value, ok := registry.values[name]
	return value, ok
}

func LoadCanonicalMetricNames(paths ...string) ([]string, error) {
	set := map[string]struct{}{}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			set[line] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		_ = file.Close()
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
