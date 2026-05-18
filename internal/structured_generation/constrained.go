package structured_generation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Constraint defines the interface for output constraints that can validate
// a value and return an error if non-compliant.
type Constraint interface {
	Validate(value any) error
}

// ValidationError describes a single schema or constraint violation.
type ValidationError struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// JSONSchemaConstraint validates that a value conforms to a simplified JSON
// schema represented as a map. StrictMode disallows additional properties
// when AllowAdditional is false.
type JSONSchemaConstraint struct {
	Schema          map[string]any `json:"schema"`
	StrictMode      bool           `json:"strict_mode"`
	AllowAdditional bool           `json:"allow_additional"`
}

func (c *JSONSchemaConstraint) Validate(value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("expected object, got %T", value)
	}
	errs := ValidateAgainstSchema(m, c.Schema)
	if len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("schema validation failed: %s", strings.Join(msgs, "; "))
	}
	return nil
}

// EnumConstraint validates that a string value is one of the allowed values.
type EnumConstraint struct {
	AllowedValues []string `json:"allowed_values"`
}

func (c *EnumConstraint) Validate(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for enum constraint, got %T", value)
	}
	for _, allowed := range c.AllowedValues {
		if s == allowed {
			return nil
		}
	}
	return fmt.Errorf("value %q not in allowed values: %v", s, c.AllowedValues)
}

// RegexConstraint validates that a string value matches the given pattern.
type RegexConstraint struct {
	Pattern string `json:"pattern"`
}

func (c *RegexConstraint) Validate(value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for regex constraint, got %T", value)
	}
	re, err := regexp.Compile(c.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %q: %w", c.Pattern, err)
	}
	if !re.MatchString(s) {
		return fmt.Errorf("value %q does not match pattern %q", s, c.Pattern)
	}
	return nil
}

// ConstraintSet holds a collection of constraints that are applied together.
type ConstraintSet struct {
	Constraints []Constraint
}

// ValidateAll runs every constraint against the given value and collects
// all validation errors.
func (cs *ConstraintSet) ValidateAll(value any) []ValidationError {
	var errs []ValidationError
	for i, c := range cs.Constraints {
		if err := c.Validate(value); err != nil {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("constraint[%d]", i),
				Message: err.Error(),
			})
		}
	}
	return errs
}

// ConstrainedOutput is returned by GenerateWithConstraints and carries
// both the raw LLM output and its parsed, validated form.
type ConstrainedOutput struct {
	RawOutput    string            `json:"raw_output"`
	ParsedOutput map[string]any    `json:"parsed_output"`
	Errors       []ValidationError `json:"errors"`
	Compliant    bool              `json:"compliant"`
}

// ConstrainedDecoder applies constraints to raw LLM outputs.
type ConstrainedDecoder struct {
	maxRepairAttempts int
}

// NewConstrainedDecoder creates a ConstrainedDecoder with sensible defaults.
func NewConstrainedDecoder() *ConstrainedDecoder {
	return &ConstrainedDecoder{
		maxRepairAttempts: 3,
	}
}

// ValidateAgainstSchema checks a parsed JSON object against a schema map.
// The schema uses a simplified format:
//
//	{"properties": {"field": {"type": "string"}}, "required": ["field"]}
func ValidateAgainstSchema(output map[string]any, schema map[string]any) []ValidationError {
	var errs []ValidationError

	properties, _ := schema["properties"].(map[string]any)
	requiredRaw, _ := schema["required"].([]any)
	required := make(map[string]bool, len(requiredRaw))
	for _, r := range requiredRaw {
		if s, ok := r.(string); ok {
			required[s] = true
		}
	}

	// Check required fields exist.
	for field := range required {
		if _, ok := output[field]; !ok {
			errs = append(errs, ValidationError{
				Path:     field,
				Expected: "present",
				Actual:   "missing",
				Message:  fmt.Sprintf("required field %q is missing", field),
			})
		}
	}

	// Check types for declared properties.
	for field, propRaw := range properties {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		expectedType, _ := prop["type"].(string)
		if expectedType == "" {
			continue
		}
		val, exists := output[field]
		if !exists {
			continue
		}
		actualType := jsonTypeName(val)
		if actualType != expectedType {
			errs = append(errs, ValidationError{
				Path:     field,
				Expected: expectedType,
				Actual:   actualType,
				Message:  fmt.Sprintf("field %q expected type %s, got %s", field, expectedType, actualType),
			})
		}
	}

	// Check for additional properties when not allowed.
	allowAdditional := true
	if v, ok := schema["additionalProperties"]; ok {
		if b, ok := v.(bool); ok {
			allowAdditional = b
		}
	}
	if !allowAdditional && properties != nil {
		for field := range output {
			if _, declared := properties[field]; !declared {
				errs = append(errs, ValidationError{
					Path:     field,
					Expected: "not present",
					Actual:   "present",
					Message:  fmt.Sprintf("additional property %q is not allowed", field),
				})
			}
		}
	}

	return errs
}

// EnforceOutputFormat takes raw LLM output and converts it to the requested
// format: "json", "yaml", or "markdown".
func EnforceOutputFormat(rawOutput string, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return enforceJSON(rawOutput)
	case "yaml":
		return enforceYAML(rawOutput)
	case "markdown":
		return enforceMarkdown(rawOutput), nil
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: json, yaml, markdown)", format)
	}
}

func enforceJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)

	// Try to extract JSON from markdown code blocks.
	if idx := strings.Index(trimmed, "```json"); idx != -1 {
		start := idx + len("```json")
		end := strings.Index(trimmed[start:], "```")
		if end != -1 {
			trimmed = strings.TrimSpace(trimmed[start : start+end])
		}
	} else if idx := strings.Index(trimmed, "```"); idx != -1 {
		start := idx + len("```")
		end := strings.Index(trimmed[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(trimmed[start : start+end])
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				trimmed = candidate
			}
		}
	}

	// Attempt to parse as-is.
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		formatted, _ := json.MarshalIndent(parsed, "", "  ")
		return string(formatted), nil
	}

	// Attempt repair.
	repaired, err := RepairJSON(trimmed)
	if err != nil {
		return "", fmt.Errorf("cannot enforce JSON format: %w", err)
	}
	return repaired, nil
}

func enforceYAML(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)

	// Try to parse as JSON first and convert.
	var jsonParsed any
	if err := json.Unmarshal([]byte(trimmed), &jsonParsed); err == nil {
		yamlBytes, yamlErr := yaml.Marshal(jsonParsed)
		if yamlErr != nil {
			return "", fmt.Errorf("cannot convert to YAML: %w", yamlErr)
		}
		return string(yamlBytes), nil
	}

	// Already YAML? Validate by round-tripping.
	var yamlParsed any
	if err := yaml.Unmarshal([]byte(trimmed), &yamlParsed); err == nil {
		yamlBytes, yamlErr := yaml.Marshal(yamlParsed)
		if yamlErr != nil {
			return "", fmt.Errorf("cannot re-marshal YAML: %w", yamlErr)
		}
		return string(yamlBytes), nil
	}

	return "", fmt.Errorf("cannot enforce YAML format: input is neither valid JSON nor YAML")
}

func enforceMarkdown(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	// If the content doesn't start with a markdown heading, wrap it.
	if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
		return "# Output\n\n" + trimmed
	}
	return trimmed
}

// GenerateWithConstraints simulates constrained generation: it parses the
// prompt output (treated as the raw LLM response for now) and validates
// it against the provided constraint set.
func (d *ConstrainedDecoder) GenerateWithConstraints(ctx context.Context, prompt string, constraints ConstraintSet) (*ConstrainedOutput, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	rawOutput := prompt

	// Try to parse as JSON.
	var parsed map[string]any
	repaired := rawOutput
	if err := json.Unmarshal([]byte(rawOutput), &parsed); err != nil {
		fixed, repairErr := RepairJSON(rawOutput)
		if repairErr != nil {
			return &ConstrainedOutput{
				RawOutput:    rawOutput,
				ParsedOutput: nil,
				Errors: []ValidationError{{
					Path:    "$",
					Message: fmt.Sprintf("failed to parse output as JSON: %s", err.Error()),
				}},
				Compliant: false,
			}, nil
		}
		repaired = fixed
		if err := json.Unmarshal([]byte(repaired), &parsed); err != nil {
			return &ConstrainedOutput{
				RawOutput:    rawOutput,
				ParsedOutput: nil,
				Errors: []ValidationError{{
					Path:    "$",
					Message: fmt.Sprintf("repaired JSON still invalid: %s", err.Error()),
				}},
				Compliant: false,
			}, nil
		}
	}

	errs := constraints.ValidateAll(parsed)
	return &ConstrainedOutput{
		RawOutput:    repaired,
		ParsedOutput: parsed,
		Errors:       errs,
		Compliant:    len(errs) == 0,
	}, nil
}

// RepairJSON attempts to fix common JSON issues: trailing commas, single
// quotes, unquoted keys, missing closing braces/brackets, and comments.
func RepairJSON(malformed string) (string, error) {
	s := strings.TrimSpace(malformed)
	if s == "" {
		return "", fmt.Errorf("empty input")
	}

	// Strip single-line comments (// ...).
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Remove inline comments (only outside strings — simplistic approach).
		if idx := strings.Index(line, "//"); idx > 0 {
			// Only strip if not inside a string (heuristic: even number of quotes before //).
			prefix := line[:idx]
			if strings.Count(prefix, `"`)%2 == 0 {
				line = prefix
			}
		}
		cleaned = append(cleaned, line)
	}
	s = strings.Join(cleaned, "\n")

	// Replace single-quoted strings with double-quoted strings.
	s = replaceSingleQuotes(s)

	// Fix unquoted keys: word followed by colon.
	unquotedKeyPattern := regexp.MustCompile(`(?m)([{\s,])([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	s = unquotedKeyPattern.ReplaceAllString(s, `$1"$2":`)

	// Remove trailing commas before } or ].
	trailingCommaPattern := regexp.MustCompile(`,\s*([}\]])`)
	s = trailingCommaPattern.ReplaceAllString(s, `$1`)

	// Balance braces and brackets.
	s = balanceBrackets(s)

	// Validate the result.
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return "", fmt.Errorf("repair failed: %w", err)
	}

	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("repair marshal failed: %w", err)
	}
	return string(formatted), nil
}

// replaceSingleQuotes replaces single-quoted strings with double-quoted ones,
// handling simple cases. This is a best-effort heuristic.
func replaceSingleQuotes(s string) string {
	var result strings.Builder
	inDouble := false
	inSingle := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' && i+1 < len(s) {
			result.WriteByte(ch)
			i++
			result.WriteByte(s[i])
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			result.WriteByte(ch)
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			result.WriteByte('"')
			continue
		}
		result.WriteByte(ch)
	}
	return result.String()
}

// balanceBrackets appends missing closing braces/brackets.
func balanceBrackets(s string) string {
	var stack []byte
	inString := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' && inString && i+1 < len(s) {
			i++
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == ch {
				stack = stack[:len(stack)-1]
			}
		}
	}
	// Append any missing closers in reverse order.
	for i := len(stack) - 1; i >= 0; i-- {
		s += string(stack[i])
	}
	return s
}

// jsonTypeName returns the JSON type name for a Go value.
func jsonTypeName(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case bool:
		return "boolean"
	case float64, float32, int, int64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}
