package llm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/llm"
)

type stubGenClient struct {
	responses []string
	callCount int
}

func (s *stubGenClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if s.callCount >= len(s.responses) {
		return &llm.GenerateResponse{Content: s.responses[len(s.responses)-1]}, &llm.Usage{}, nil
	}
	resp := s.responses[s.callCount]
	s.callCount++
	return &llm.GenerateResponse{Content: resp}, &llm.Usage{}, nil
}

func (s *stubGenClient) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	close(out)
}

func TestGenerationService_PassesOnFirstAttempt(t *testing.T) {
	t.Parallel()
	client := &stubGenClient{responses: []string{`{"ok":true}`}}
	svc := llm.NewGenerationService(client)
	result, err := svc.Generate(context.Background(), llm.ConstrainedGenerateRequest{
		Prompt:      "return json",
		Constraints: []llm.GenerationConstraint{llm.JSONConstraint()},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, result)
}

func TestGenerationService_RetriesOnConstraintFailure(t *testing.T) {
	t.Parallel()
	client := &stubGenClient{responses: []string{"not json at all", `{"ok":true}`}}
	svc := llm.NewGenerationService(client)
	result, err := svc.Generate(context.Background(), llm.ConstrainedGenerateRequest{
		Prompt:      "return json",
		MaxRetries:  2,
		Constraints: []llm.GenerationConstraint{llm.JSONConstraint()},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, result)
	assert.Equal(t, 2, client.callCount)
}

func TestGenerationService_FailsAfterMaxRetries(t *testing.T) {
	t.Parallel()
	client := &stubGenClient{responses: []string{"bad", "also bad", "still bad", "still bad"}}
	svc := llm.NewGenerationService(client)
	_, err := svc.Generate(context.Background(), llm.ConstrainedGenerateRequest{
		Prompt:      "return json",
		MaxRetries:  2,
		Constraints: []llm.GenerationConstraint{llm.JSONConstraint()},
	})
	assert.Error(t, err)
}

func TestGenerationService_NilClient_ReturnsError(t *testing.T) {
	t.Parallel()
	svc := llm.NewGenerationService(nil)
	_, err := svc.Generate(context.Background(), llm.ConstrainedGenerateRequest{Prompt: "test"})
	assert.Error(t, err)
}

func TestNonEmptyConstraint(t *testing.T) {
	t.Parallel()
	c := llm.NonEmptyConstraint()
	assert.Error(t, c.Validate(""))
	assert.NoError(t, c.Validate("some content"))
}

func TestMaxLengthConstraint(t *testing.T) {
	t.Parallel()
	c := llm.MaxLengthConstraint(10)
	assert.Error(t, c.Validate("this is definitely too long"))
	assert.NoError(t, c.Validate("short"))
}
