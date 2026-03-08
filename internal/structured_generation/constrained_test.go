package structured_generation

import (
	"context"
	"testing"
)

func TestEnumConstraintValid(t *testing.T) {
	t.Parallel()
	c := &EnumConstraint{AllowedValues: []string{"low", "medium", "high"}}
	if err := c.Validate("medium"); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestEnumConstraintInvalid(t *testing.T) {
	t.Parallel()
	c := &EnumConstraint{AllowedValues: []string{"low", "medium", "high"}}
	if err := c.Validate("critical"); err == nil {
		t.Fatal("expected error for invalid enum value")
	}
}

func TestEnumConstraintWrongType(t *testing.T) {
	t.Parallel()
	c := &EnumConstraint{AllowedValues: []string{"a"}}
	if err := c.Validate(42); err == nil {
		t.Fatal("expected error for non-string value")
	}
}

func TestRegexConstraintValid(t *testing.T) {
	t.Parallel()
	c := &RegexConstraint{Pattern: `^[A-Z]{2,3}-\d+$`}
	if err := c.Validate("AB-123"); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestRegexConstraintInvalid(t *testing.T) {
	t.Parallel()
	c := &RegexConstraint{Pattern: `^[A-Z]{2,3}-\d+$`}
	if err := c.Validate("abc"); err == nil {
		t.Fatal("expected error for non-matching value")
	}
}

func TestRegexConstraintBadPattern(t *testing.T) {
	t.Parallel()
	c := &RegexConstraint{Pattern: `[invalid`}
	if err := c.Validate("abc"); err == nil {
		t.Fatal("expected error for bad pattern")
	}
}

func TestJSONSchemaConstraintRequired(t *testing.T) {
	t.Parallel()
	c := &JSONSchemaConstraint{
		Schema: map[string]any{
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}
	if err := c.Validate(map[string]any{"age": float64(30)}); err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestJSONSchemaConstraintTypeMismatch(t *testing.T) {
	t.Parallel()
	c := &JSONSchemaConstraint{
		Schema: map[string]any{
			"properties": map[string]any{
				"count": map[string]any{"type": "number"},
			},
		},
	}
	if err := c.Validate(map[string]any{"count": "not_a_number"}); err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestJSONSchemaConstraintValid(t *testing.T) {
	t.Parallel()
	c := &JSONSchemaConstraint{
		Schema: map[string]any{
			"properties": map[string]any{
				"name":  map[string]any{"type": "string"},
				"count": map[string]any{"type": "number"},
			},
			"required": []any{"name"},
		},
	}
	if err := c.Validate(map[string]any{"name": "test", "count": float64(5)}); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestJSONSchemaConstraintNonObject(t *testing.T) {
	t.Parallel()
	c := &JSONSchemaConstraint{Schema: map[string]any{}}
	if err := c.Validate("string_value"); err == nil {
		t.Fatal("expected error for non-object value")
	}
}

func TestValidateAgainstSchemaAdditionalProperties(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
	output := map[string]any{"name": "ok", "extra": "bad"}
	errs := ValidateAgainstSchema(output, schema)
	if len(errs) == 0 {
		t.Fatal("expected validation error for additional property")
	}
	found := false
	for _, e := range errs {
		if e.Path == "extra" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for 'extra' field")
	}
}

func TestConstraintSetValidateAll(t *testing.T) {
	t.Parallel()
	cs := ConstraintSet{
		Constraints: []Constraint{
			&EnumConstraint{AllowedValues: []string{"a", "b"}},
			&RegexConstraint{Pattern: `^[a-z]$`},
		},
	}
	errs := cs.ValidateAll("a")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}

	errs = cs.ValidateAll("X")
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got: %d", len(errs))
	}
}

func TestEnforceOutputFormatJSON(t *testing.T) {
	t.Parallel()
	out, err := EnforceOutputFormat(`{"key":"value"}`, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestEnforceOutputFormatJSONFromCodeBlock(t *testing.T) {
	t.Parallel()
	raw := "```json\n{\"a\": 1}\n```"
	out, err := EnforceOutputFormat(raw, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestEnforceOutputFormatYAML(t *testing.T) {
	t.Parallel()
	out, err := EnforceOutputFormat(`{"key":"value"}`, "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestEnforceOutputFormatMarkdown(t *testing.T) {
	t.Parallel()
	out, err := EnforceOutputFormat("Some plain text", "markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if out != "# Output\n\nSome plain text" {
		t.Fatalf("unexpected markdown output: %s", out)
	}
}

func TestEnforceOutputFormatMarkdownAlreadyFormatted(t *testing.T) {
	t.Parallel()
	out, err := EnforceOutputFormat("# Title\n\nBody", "markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "# Title\n\nBody" {
		t.Fatalf("should not re-wrap already formatted markdown: %s", out)
	}
}

func TestEnforceOutputFormatUnsupported(t *testing.T) {
	t.Parallel()
	_, err := EnforceOutputFormat("data", "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestRepairJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `{"a": 1, "b": 2,}`
	out, err := RepairJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRepairJSONSingleQuotes(t *testing.T) {
	t.Parallel()
	input := `{'key': 'value'}`
	out, err := RepairJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRepairJSONUnquotedKeys(t *testing.T) {
	t.Parallel()
	input := `{name: "test", count: 5}`
	out, err := RepairJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRepairJSONComments(t *testing.T) {
	t.Parallel()
	input := "{\n// this is a comment\n\"key\": \"value\"\n}"
	out, err := RepairJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRepairJSONMissingClosingBrace(t *testing.T) {
	t.Parallel()
	input := `{"key": "value"`
	out, err := RepairJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRepairJSONEmpty(t *testing.T) {
	t.Parallel()
	_, err := RepairJSON("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestGenerateWithConstraintsCompliant(t *testing.T) {
	t.Parallel()
	decoder := NewConstrainedDecoder()
	cs := ConstraintSet{
		Constraints: []Constraint{
			&JSONSchemaConstraint{
				Schema: map[string]any{
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
					},
					"required": []any{"status"},
				},
			},
		},
	}

	result, err := decoder.GenerateWithConstraints(context.Background(), `{"status": "ok"}`, cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Compliant {
		t.Fatalf("expected compliant output, errors: %v", result.Errors)
	}
	if result.ParsedOutput["status"] != "ok" {
		t.Fatalf("unexpected parsed output: %v", result.ParsedOutput)
	}
}

func TestGenerateWithConstraintsNonCompliant(t *testing.T) {
	t.Parallel()
	decoder := NewConstrainedDecoder()
	cs := ConstraintSet{
		Constraints: []Constraint{
			&JSONSchemaConstraint{
				Schema: map[string]any{
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
					},
					"required": []any{"status"},
				},
			},
		},
	}

	result, err := decoder.GenerateWithConstraints(context.Background(), `{"other": 42}`, cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Compliant {
		t.Fatal("expected non-compliant output")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestGenerateWithConstraintsInvalidJSON(t *testing.T) {
	t.Parallel()
	decoder := NewConstrainedDecoder()
	cs := ConstraintSet{}

	result, err := decoder.GenerateWithConstraints(context.Background(), "not json at all", cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Compliant {
		t.Fatal("expected non-compliant for unparseable input")
	}
}

func TestGenerateWithConstraintsCancelledContext(t *testing.T) {
	t.Parallel()
	decoder := NewConstrainedDecoder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := decoder.GenerateWithConstraints(ctx, `{}`, ConstraintSet{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGenerateWithConstraintsRepairableJSON(t *testing.T) {
	t.Parallel()
	decoder := NewConstrainedDecoder()
	cs := ConstraintSet{
		Constraints: []Constraint{
			&JSONSchemaConstraint{
				Schema: map[string]any{
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
					"required": []any{"name"},
				},
			},
		},
	}

	result, err := decoder.GenerateWithConstraints(context.Background(), `{"name": "test",}`, cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Compliant {
		t.Fatalf("expected compliant after repair, errors: %v", result.Errors)
	}
}
