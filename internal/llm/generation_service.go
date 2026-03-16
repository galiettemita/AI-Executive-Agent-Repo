package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// GenerationConstraint defines a validation rule for LLM output.
type GenerationConstraint struct {
	Name     string
	Validate func(output string) error
}

// ConstrainedGenerateRequest is the input to GenerationService.Generate.
type ConstrainedGenerateRequest struct {
	Prompt      string
	MaxRetries  int // 0 = use default (3)
	Constraints []GenerationConstraint
}

// GenerationService wraps an LLM client with constraint validation and retry logic.
// If the LLM output fails any constraint, it retries up to MaxRetries times,
// injecting the constraint violation as feedback into the next attempt.
type GenerationService struct {
	client Client
}

// NewGenerationService creates a GenerationService wrapping the given client.
func NewGenerationService(client Client) *GenerationService {
	return &GenerationService{client: client}
}

// Generate calls the LLM and validates output against all constraints.
// On constraint failure, retries with the violation injected as corrective feedback.
func (g *GenerationService) Generate(ctx context.Context, req ConstrainedGenerateRequest) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("generation_service: no LLM client configured")
	}
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	prompt := req.Prompt
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, _, err := g.client.Generate(ctx, GenerateRequest{
			Messages: []ChatMsg{{Role: "user", Content: prompt}},
		})
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: LLM error: %w", attempt, err)
			continue
		}

		output := resp.Content

		// Validate all constraints.
		var violations []string
		for _, c := range req.Constraints {
			if verr := c.Validate(output); verr != nil {
				violations = append(violations, fmt.Sprintf("%s: %v", c.Name, verr))
			}
		}

		if len(violations) == 0 {
			return output, nil
		}

		// Constraint failure — inject violation as corrective feedback.
		if attempt < maxRetries {
			feedback := "\n\nYour previous response had these issues:\n"
			for _, v := range violations {
				feedback += fmt.Sprintf("- %s\n", v)
			}
			feedback += "Please correct these issues and try again."
			prompt = req.Prompt + feedback
			lastErr = fmt.Errorf("constraint violations: %v", violations)
		}
	}

	return "", fmt.Errorf("generation_service: max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// JSONConstraint returns a GenerationConstraint that validates the output is valid JSON.
func JSONConstraint() GenerationConstraint {
	return GenerationConstraint{
		Name: "valid_json",
		Validate: func(output string) error {
			var v any
			return json.Unmarshal([]byte(output), &v)
		},
	}
}

// NonEmptyConstraint returns a GenerationConstraint that validates the output is non-empty.
func NonEmptyConstraint() GenerationConstraint {
	return GenerationConstraint{
		Name: "non_empty",
		Validate: func(output string) error {
			if len(output) == 0 {
				return fmt.Errorf("output is empty")
			}
			return nil
		},
	}
}

// MaxLengthConstraint returns a GenerationConstraint that validates output length.
func MaxLengthConstraint(max int) GenerationConstraint {
	return GenerationConstraint{
		Name: fmt.Sprintf("max_length_%d", max),
		Validate: func(output string) error {
			if len(output) > max {
				return fmt.Errorf("output length %d exceeds max %d", len(output), max)
			}
			return nil
		},
	}
}
