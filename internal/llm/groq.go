package llm

import (
	"context"
	"fmt"
	"os"
)

const (
	groqDefaultModel = "llama-3.1-70b-versatile"
	groqBaseURL      = "https://api.groq.com/openai/v1"
)

// GroqClient uses Groq's OpenAI-compatible API via the existing OpenAI transport.
type GroqClient struct {
	inner *OpenAIClient
}

// NewGroqClient creates a Groq client using GROQ_API_KEY from the environment.
func NewGroqClient() (*GroqClient, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not set")
	}
	inner, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  key,
		BaseURL: groqBaseURL,
	})
	if err != nil {
		return nil, err
	}
	return &GroqClient{inner: inner}, nil
}

// ProviderName returns "groq".
func (g *GroqClient) ProviderName() string { return "groq" }

// Generate implements Client.Generate via the OpenAI-compatible Groq API.
func (g *GroqClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	if req.Model == "" {
		req.Model = groqDefaultModel
	}
	resp, usage, err := g.inner.Generate(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	resp.ProviderID = "groq"
	return resp, usage, nil
}

// Stream implements Client.Stream via the OpenAI-compatible Groq API.
func (g *GroqClient) Stream(ctx context.Context, req GenerateRequest, out chan<- StreamChunk) {
	if req.Model == "" {
		req.Model = groqDefaultModel
	}
	g.inner.Stream(ctx, req, out)
}

// Compile-time interface check.
var _ Client = (*GroqClient)(nil)
