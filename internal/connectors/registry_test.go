package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// YAML Parsing & Seed Loading
// ---------------------------------------------------------------------------

func TestSeedFileYAMLParsing(t *testing.T) {
	t.Parallel()

	content := []byte(`
connectors:
  - { key: test_conn, domain: web, risk_level: LOW, data_class: public, mcp_server_url: https://mcp.example/test_conn }
tools:
  - { connector_key: test_conn, tool_key: test_conn.search, write: false, reversible: false, autonomy_floor: A0 }
  - { connector_key: test_conn, tool_key: test_conn.write, write: true, reversible: true, autonomy_floor: A2 }
`)

	seed, err := parseSeed(".yaml", content)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(seed.Connectors) != 1 {
		t.Fatalf("expected 1 connector, got %d", len(seed.Connectors))
	}
	if seed.Connectors[0].Key != "test_conn" {
		t.Errorf("expected key test_conn, got %s", seed.Connectors[0].Key)
	}
	if len(seed.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(seed.Tools))
	}
	if seed.Tools[0].ToolKey != "test_conn.search" {
		t.Errorf("expected tool_key test_conn.search, got %s", seed.Tools[0].ToolKey)
	}
	if seed.Tools[1].Write != true {
		t.Error("expected test_conn.write to be write-capable")
	}
}

func TestSeedFileYAMLParsing_InvalidYAML(t *testing.T) {
	t.Parallel()

	content := []byte(`{{{broken yaml}}}`)
	_, err := parseSeed(".yaml", content)
	if err == nil {
		t.Fatal("expected parse error for invalid YAML")
	}
}

func TestSeedLoaderValidation_InvalidToolKey(t *testing.T) {
	t.Parallel()

	kp := NewInMemoryKeyProvider("v1", make([]byte, 32))
	svc := NewService(kp)
	err := svc.LoadSeed(seedFile{
		Connectors: []Connector{{
			Key:          "test_conn",
			Domain:       "web",
			RiskLevel:    "LOW",
			DataClass:    "public",
			MCPServerURL: "https://mcp.example/test_conn",
		}},
		Tools: []ConnectorTool{{
			ConnectorKey:  "test_conn",
			ToolKey:       "INVALID-KEY",
			AutonomyFloor: "A0",
		}},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid tool_key")
	}
}

func TestSeedLoaderValidation_UnknownConnector(t *testing.T) {
	t.Parallel()

	kp := NewInMemoryKeyProvider("v1", make([]byte, 32))
	svc := NewService(kp)
	err := svc.LoadSeed(seedFile{
		Connectors: []Connector{{
			Key:          "test_conn",
			Domain:       "web",
			RiskLevel:    "LOW",
			DataClass:    "public",
			MCPServerURL: "https://mcp.example/test_conn",
		}},
		Tools: []ConnectorTool{{
			ConnectorKey:  "unknown_conn",
			ToolKey:       "unknown_conn.search",
			AutonomyFloor: "A0",
		}},
	})
	if err == nil {
		t.Fatal("expected validation error for unknown connector reference")
	}
}

// ---------------------------------------------------------------------------
// In-memory Service: ListAllTools, ToolKeys, HasTool
// ---------------------------------------------------------------------------

func TestServiceListAllTools(t *testing.T) {
	t.Parallel()
	svc := newSeededService(t)

	tools := svc.ListAllTools()
	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Verify sorted order.
	for i := 1; i < len(tools); i++ {
		if tools[i].ToolKey < tools[i-1].ToolKey {
			t.Fatalf("tools not sorted: %s < %s at index %d", tools[i].ToolKey, tools[i-1].ToolKey, i)
		}
	}
}

func TestServiceToolKeys(t *testing.T) {
	t.Parallel()
	svc := newSeededService(t)

	keys := svc.ToolKeys()
	if len(keys) == 0 {
		t.Fatal("expected at least one tool key")
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Fatalf("keys not sorted: %s < %s", keys[i], keys[i-1])
		}
	}
}

func TestServiceHasTool(t *testing.T) {
	t.Parallel()
	svc := newSeededService(t)

	keys := svc.ToolKeys()
	if len(keys) == 0 {
		t.Skip("no tools loaded")
	}
	if !svc.HasTool(keys[0]) {
		t.Fatalf("HasTool(%s) returned false", keys[0])
	}
	if svc.HasTool("nonexistent.tool") {
		t.Fatal("HasTool(nonexistent.tool) returned true")
	}
}

// ---------------------------------------------------------------------------
// GET /v1/tools Handler
// ---------------------------------------------------------------------------

func TestHandleListTools(t *testing.T) {
	t.Parallel()
	svc := newSeededService(t)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/tools", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var resp ToolRegistryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.ConnectorCount < 40 {
		t.Errorf("expected at least 40 connectors, got %d", resp.ConnectorCount)
	}
	if resp.ToolCount == 0 {
		t.Error("expected at least one tool")
	}
	if len(resp.Connectors) != resp.ConnectorCount {
		t.Errorf("connector count mismatch: header=%d body=%d", resp.ConnectorCount, len(resp.Connectors))
	}
	if len(resp.Tools) != resp.ToolCount {
		t.Errorf("tool count mismatch: header=%d body=%d", resp.ToolCount, len(resp.Tools))
	}

	// Verify tools have valid structure.
	for _, tool := range resp.Tools {
		if tool.ToolKey == "" {
			t.Error("tool has empty tool_key")
		}
		if tool.ConnectorKey == "" {
			t.Error("tool has empty connector_key")
		}
	}

	t.Logf("GET /v1/tools returned %d connectors, %d tools", resp.ConnectorCount, resp.ToolCount)
}

func TestHandleListTools_EmptyRegistry(t *testing.T) {
	t.Parallel()

	kp := NewInMemoryKeyProvider("v1", make([]byte, 32))
	svc := NewService(kp)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/tools", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var resp ToolRegistryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.ConnectorCount != 0 {
		t.Errorf("expected 0 connectors, got %d", resp.ConnectorCount)
	}
	if resp.ToolCount != 0 {
		t.Errorf("expected 0 tools, got %d", resp.ToolCount)
	}
}

// ---------------------------------------------------------------------------
// SeedToRepository (in-memory mock)
// ---------------------------------------------------------------------------

type inMemoryRegistryRepo struct {
	connectors map[string]Connector
	tools      map[string]ConnectorTool
}

func newInMemoryRegistryRepo() *inMemoryRegistryRepo {
	return &inMemoryRegistryRepo{
		connectors: map[string]Connector{},
		tools:      map[string]ConnectorTool{},
	}
}

func (r *inMemoryRegistryRepo) UpsertConnector(_ context.Context, c Connector) error {
	r.connectors[c.Key] = c
	return nil
}

func (r *inMemoryRegistryRepo) UpsertTool(_ context.Context, t ConnectorTool) error {
	r.tools[t.ToolKey] = t
	return nil
}

func (r *inMemoryRegistryRepo) ListAllTools(_ context.Context) ([]ConnectorTool, error) {
	out := make([]ConnectorTool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out, nil
}

func (r *inMemoryRegistryRepo) ListAllConnectors(_ context.Context) ([]Connector, error) {
	out := make([]Connector, 0, len(r.connectors))
	for _, c := range r.connectors {
		out = append(out, c)
	}
	return out, nil
}

func TestSeedToRepository(t *testing.T) {
	t.Parallel()

	svc := newSeededService(t)
	repo := newInMemoryRegistryRepo()

	nConn, nTools, err := svc.SeedToRepository(context.Background(), repo)
	if err != nil {
		t.Fatalf("SeedToRepository failed: %v", err)
	}
	if nConn < 40 {
		t.Errorf("expected at least 40 connectors seeded, got %d", nConn)
	}
	if nTools == 0 {
		t.Error("expected at least one tool seeded")
	}
	if len(repo.connectors) != nConn {
		t.Errorf("repo connector count mismatch: %d vs %d", len(repo.connectors), nConn)
	}
	if len(repo.tools) != nTools {
		t.Errorf("repo tool count mismatch: %d vs %d", len(repo.tools), nTools)
	}

	t.Logf("SeedToRepository: %d connectors, %d tools", nConn, nTools)
}
