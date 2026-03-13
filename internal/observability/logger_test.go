package observability

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerOutputIsJSONWithRequiredKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Service: "test-svc",
		Env:     "staging",
		Version: "1.2.3",
		Output:  &buf,
	})

	logger.Info("request handled",
		"request_id", "req-001",
		"workflow_id", "wf-abc",
	)

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("logger output is not valid JSON: %v\nraw: %s", err, buf.String())
	}

	required := []string{"time", "level", "msg", "service", "env", "version"}
	for _, key := range required {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing required key %q in logger output: %v", key, decoded)
		}
	}

	if decoded["service"] != "test-svc" {
		t.Fatalf("unexpected service: %v", decoded["service"])
	}
	if decoded["env"] != "staging" {
		t.Fatalf("unexpected env: %v", decoded["env"])
	}
	if decoded["version"] != "1.2.3" {
		t.Fatalf("unexpected version: %v", decoded["version"])
	}
	if decoded["msg"] != "request handled" {
		t.Fatalf("unexpected msg: %v", decoded["msg"])
	}
	if decoded["request_id"] != "req-001" {
		t.Fatalf("expected request_id in output: %v", decoded)
	}
}

func TestLoggerDoesNotIncludeSecrets(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Service: "test-svc",
		Env:     "prod",
		Version: "1.0.0",
		Output:  &buf,
	})

	logger.Info("startup", "listen_addr", ":8080")

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected logger output")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("logger output is not valid JSON: %v", err)
	}

	for key := range decoded {
		if key == "password" || key == "secret" || key == "token" || key == "api_key" {
			t.Fatalf("logger output contains secret-like key: %s", key)
		}
	}
}
