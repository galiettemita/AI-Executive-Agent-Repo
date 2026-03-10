package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SchemaEnvelope holds a parsed JSON Schema with additionalProperties enforcement.
type SchemaEnvelope struct {
	Type                 string                    `json:"type"`
	AdditionalProperties *bool                     `json:"additionalProperties,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	Properties           map[string]SchemaProperty `json:"properties,omitempty"`
}

// SchemaProperty is a simplified property within a JSON schema.
type SchemaProperty struct {
	Type                 string                    `json:"type"`
	AdditionalProperties *bool                     `json:"additionalProperties,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	Properties           map[string]SchemaProperty `json:"properties,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Pattern              string                    `json:"pattern,omitempty"`
	MinLength            *int                      `json:"minLength,omitempty"`
	MaxLength            *int                      `json:"maxLength,omitempty"`
}

// LoadSchema loads and parses a JSON schema file from the schemas directory.
func LoadSchema(schemaFile string) (*SchemaEnvelope, error) {
	schemasDir := findSchemasDir()
	path := filepath.Join(schemasDir, schemaFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", schemaFile, err)
	}
	var schema SchemaEnvelope
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", schemaFile, err)
	}
	return &schema, nil
}

// ValidatePayloadAgainstSchema validates a JSON payload against a schema envelope.
// Checks: required fields present, no additional properties (when schema says false),
// and nested object validation.
func ValidatePayloadAgainstSchema(schema *SchemaEnvelope, payload map[string]any) []string {
	var violations []string

	// Check required fields.
	for _, req := range schema.Required {
		if _, ok := payload[req]; !ok {
			violations = append(violations, fmt.Sprintf("missing required field: %s", req))
		}
	}

	// Check additionalProperties: false enforcement.
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for key := range payload {
			if _, defined := schema.Properties[key]; !defined {
				violations = append(violations, fmt.Sprintf("additional property not allowed: %s", key))
			}
		}
	}

	// Recursive validation for nested objects.
	for key, propSchema := range schema.Properties {
		val, ok := payload[key]
		if !ok {
			continue
		}
		if propSchema.Type == "object" && propSchema.AdditionalProperties != nil && !*propSchema.AdditionalProperties {
			nested, isMap := val.(map[string]any)
			if !isMap {
				violations = append(violations, fmt.Sprintf("field %s: expected object, got %T", key, val))
				continue
			}
			for subKey := range nested {
				if _, defined := propSchema.Properties[subKey]; !defined {
					violations = append(violations, fmt.Sprintf("field %s: additional property not allowed: %s", key, subKey))
				}
			}
			for _, req := range propSchema.Required {
				if _, exists := nested[req]; !exists {
					violations = append(violations, fmt.Sprintf("field %s: missing required field: %s", key, req))
				}
			}
		}
	}

	return violations
}

// findSchemasDir locates the schemas directory relative to the project root.
func findSchemasDir() string {
	// Try relative to caller source file first (works in tests).
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		candidate := filepath.Join(dir, "..", "..", "schemas")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	// Fallback: relative to working directory.
	return "schemas"
}
