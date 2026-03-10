package contracts

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// handlerMapping records which service owns a given OpenAPI endpoint and where
// the handler is registered. Every v10 endpoint must have an entry here.
// Adding a new endpoint to v10.yaml without updating this registry will cause
// the test to fail — this is intentional to enforce handler closure.
//
// To skip an endpoint (e.g., planned but not yet implemented), add an entry to
// deferredEndpoints with a DECISIONS.md reference justifying the deferral.
type handlerMapping struct {
	Service     string // e.g., "gateway", "control", "admin"
	HandlerFile string // e.g., "internal/gateway/server.go"
	AuthScheme  string // e.g., "BearerAuth", "AdminAuth", "none"
}

// v10HandlerRegistry is the authoritative mapping from OpenAPI v10 endpoints
// to their handler implementations. Format: "METHOD /path" -> handlerMapping.
//
// This registry is manually maintained. When a new endpoint is added to
// api/openapi/v10.yaml, a corresponding entry must be added here that points
// to the real handler file. The contract test verifies bidirectional closure:
// every OpenAPI endpoint has a handler, and every registry entry has an
// OpenAPI definition.
var v10HandlerRegistry = map[string]handlerMapping{
	// Health — served by gateway mux
	"GET /health": {
		Service:     "gateway",
		HandlerFile: "internal/gateway/server.go",
		AuthScheme:  "none",
	},

	// Messages — gateway ingress
	"POST /messages": {
		Service:     "gateway",
		HandlerFile: "internal/gateway/server.go",
		AuthScheme:  "BearerAuth",
	},

	// Federation — domain service handles federation logic
	"GET /workspaces/{workspaceId}/federation/peers": {
		Service:     "gateway",
		HandlerFile: "internal/federation/service.go",
		AuthScheme:  "BearerAuth",
	},
	"POST /workspaces/{workspaceId}/federation/peers": {
		Service:     "gateway",
		HandlerFile: "internal/federation/service.go",
		AuthScheme:  "BearerAuth",
	},
	"POST /workspaces/{workspaceId}/federation/negotiations": {
		Service:     "gateway",
		HandlerFile: "internal/federation/service.go",
		AuthScheme:  "BearerAuth",
	},

	// Wallet — wallet domain service
	"GET /workspaces/{workspaceId}/wallet": {
		Service:     "gateway",
		HandlerFile: "internal/wallet/service.go",
		AuthScheme:  "BearerAuth",
	},
	"GET /workspaces/{workspaceId}/wallet/transactions": {
		Service:     "gateway",
		HandlerFile: "internal/wallet/service.go",
		AuthScheme:  "BearerAuth",
	},

	// Costs — billing domain service
	"GET /workspaces/{workspaceId}/costs": {
		Service:     "gateway",
		HandlerFile: "internal/billing/service.go",
		AuthScheme:  "BearerAuth",
	},

	// Capabilities — gateway service
	"GET /workspaces/{workspaceId}/capabilities": {
		Service:     "gateway",
		HandlerFile: "internal/gateway/server.go",
		AuthScheme:  "BearerAuth",
	},

	// Kill Switch — control plane
	"GET /workspaces/{workspaceId}/kill-switch": {
		Service:     "control",
		HandlerFile: "internal/control/mux.go",
		AuthScheme:  "BearerAuth",
	},
	"POST /workspaces/{workspaceId}/kill-switch": {
		Service:     "control",
		HandlerFile: "internal/control/mux.go",
		AuthScheme:  "AdminAuth",
	},

	// Calls — voice worker session orchestration
	"POST /workspaces/{workspaceId}/calls": {
		Service:     "gateway",
		HandlerFile: "internal/voice/worker/session.go",
		AuthScheme:  "BearerAuth",
	},
	"GET /workspaces/{workspaceId}/calls/{callId}/transcript": {
		Service:     "gateway",
		HandlerFile: "internal/voice/worker/session.go",
		AuthScheme:  "BearerAuth",
	},

	// Admin — admin handlers
	"POST /admin/auth/login": {
		Service:     "gateway",
		HandlerFile: "internal/admin/handlers.go",
		AuthScheme:  "none",
	},
	"GET /admin/workspaces": {
		Service:     "gateway",
		HandlerFile: "internal/admin/handlers.go",
		AuthScheme:  "AdminAuth",
	},
	"GET /admin/workspaces/{workspaceId}/health": {
		Service:     "gateway",
		HandlerFile: "internal/admin/handlers.go",
		AuthScheme:  "AdminAuth",
	},
}

// TestOpenAPIV10HandlerClosureCoverage parses api/openapi/v10.yaml and asserts
// that every path+method has a corresponding entry in v10HandlerRegistry.
// This prevents "phantom endpoints" — API definitions with no backing handler.
func TestOpenAPIV10HandlerClosureCoverage(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	specPath := filepath.Join(root, "api", "openapi", "v10.yaml")
	body, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read v10 openapi spec: %v", err)
	}

	var doc openapiDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse v10 openapi yaml: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("v10 openapi paths are empty")
	}

	// Collect all endpoints from the spec.
	specEndpoints := map[string]struct{}{}
	for path, operations := range doc.Paths {
		for method := range operations {
			methodUpper := strings.ToUpper(method)
			switch methodUpper {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
				specEndpoints[methodUpper+" "+path] = struct{}{}
			}
		}
	}

	// Check: every spec endpoint must be in the handler registry.
	missingHandlers := []string{}
	for endpoint := range specEndpoints {
		if _, ok := v10HandlerRegistry[endpoint]; !ok {
			missingHandlers = append(missingHandlers, endpoint)
		}
	}
	if len(missingHandlers) > 0 {
		sort.Strings(missingHandlers)
		t.Fatalf("OpenAPI v10 endpoints without handler mapping in v10HandlerRegistry (add entries or document deferral in DECISIONS.md):\n  %s",
			strings.Join(missingHandlers, "\n  "))
	}

	// Check: every registry entry must exist in the spec (no stale entries).
	staleEntries := []string{}
	for endpoint := range v10HandlerRegistry {
		if _, ok := specEndpoints[endpoint]; !ok {
			staleEntries = append(staleEntries, endpoint)
		}
	}
	if len(staleEntries) > 0 {
		sort.Strings(staleEntries)
		t.Fatalf("Handler registry entries without corresponding OpenAPI v10 spec endpoint (remove stale entries):\n  %s",
			strings.Join(staleEntries, "\n  "))
	}
}

// TestOpenAPIV10HandlerAuthSchemeAlignment verifies that the auth scheme
// declared in the OpenAPI spec matches the handler registry's auth binding.
func TestOpenAPIV10HandlerAuthSchemeAlignment(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	specPath := filepath.Join(root, "api", "openapi", "v10.yaml")
	body, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read v10 openapi spec: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse v10 openapi yaml: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("v10 openapi paths missing")
	}

	for endpoint, mapping := range v10HandlerRegistry {
		if mapping.AuthScheme == "none" {
			continue
		}

		parts := strings.SplitN(endpoint, " ", 2)
		method := strings.ToLower(parts[0])
		path := parts[1]

		pathOps, ok := paths[path].(map[string]any)
		if !ok {
			continue // Covered by coverage test above.
		}
		op, ok := pathOps[method].(map[string]any)
		if !ok {
			continue
		}

		securityRaw, ok := op["security"].([]any)
		if !ok {
			t.Fatalf("endpoint %s declares AuthScheme=%s in registry but has no security block in spec", endpoint, mapping.AuthScheme)
		}

		found := false
		for _, reqRaw := range securityRaw {
			req, ok := reqRaw.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := req[mapping.AuthScheme]; exists {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("endpoint %s: registry declares AuthScheme=%s but spec does not include it in security requirements", endpoint, mapping.AuthScheme)
		}
	}
}

// TestOpenAPIV10HandlerFileExistence verifies that every handler file
// referenced in the registry actually exists in the repository.
func TestOpenAPIV10HandlerFileExistence(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	checked := map[string]bool{}
	for endpoint, mapping := range v10HandlerRegistry {
		if checked[mapping.HandlerFile] {
			continue
		}
		checked[mapping.HandlerFile] = true

		fullPath := filepath.Join(root, mapping.HandlerFile)
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("handler file for %s does not exist: %s", endpoint, mapping.HandlerFile)
		}
	}
}
