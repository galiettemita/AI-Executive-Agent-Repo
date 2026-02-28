package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type openapiDoc struct {
	Paths map[string]map[string]map[string]any `yaml:"paths"`
}

type endpoint struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
}

var methodOrder = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatalf("resolve working directory: %v", err)
	}

	specPath := filepath.Join(root, "api", "openapi", "v9.yaml")
	docBytes, err := os.ReadFile(specPath)
	if err != nil {
		fatalf("read openapi spec %s: %v", specPath, err)
	}

	var doc openapiDoc
	if err := yaml.Unmarshal(docBytes, &doc); err != nil {
		fatalf("parse openapi spec %s: %v", specPath, err)
	}

	endpoints := collectEndpoints(doc)
	if len(endpoints) == 0 {
		fatalf("no endpoints discovered in %s", specPath)
	}

	outPath := filepath.Join(root, "docs", "API_REFERENCE.md")
	specDigest := sha256.Sum256(docBytes)
	if err := os.WriteFile(outPath, []byte(renderMarkdown(endpoints, specDigest)), 0o644); err != nil {
		fatalf("write api reference %s: %v", outPath, err)
	}

	fmt.Printf("generated %s with %d endpoints\n", outPath, len(endpoints))
}

func collectEndpoints(doc openapiDoc) []endpoint {
	if len(doc.Paths) == 0 {
		return nil
	}
	pathKeys := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	methodRank := make(map[string]int, len(methodOrder))
	for idx, method := range methodOrder {
		methodRank[method] = idx
	}

	var out []endpoint
	for _, path := range pathKeys {
		ops := doc.Paths[path]
		for rawMethod, operation := range ops {
			method := strings.ToUpper(strings.TrimSpace(rawMethod))
			if _, ok := methodRank[method]; !ok {
				continue
			}
			opID := strings.TrimSpace(stringValue(operation["operationId"]))
			summary := strings.TrimSpace(stringValue(operation["summary"]))
			out = append(out, endpoint{
				Method:      method,
				Path:        path,
				OperationID: opID,
				Summary:     summary,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return methodRank[out[i].Method] < methodRank[out[j].Method]
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func renderMarkdown(endpoints []endpoint, specDigest [32]byte) string {
	var b strings.Builder

	b.WriteString("# API Reference\n\n")
	b.WriteString("Generated from `api/openapi/v9.yaml`.\n\n")
	b.WriteString(fmt.Sprintf("OpenAPI SHA-256: `%s`\n\n", hex.EncodeToString(specDigest[:])))
	b.WriteString(fmt.Sprintf("Total endpoints: `%d`\n\n", len(endpoints)))
	b.WriteString("| Method | Path | operation_id | Summary |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, ep := range endpoints {
		opID := ep.OperationID
		if opID == "" {
			opID = "-"
		}
		summary := ep.Summary
		if summary == "" {
			summary = "-"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s |\n", ep.Method, ep.Path, opID, escapeTable(summary)))
	}
	b.WriteString("\n")
	return b.String()
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
