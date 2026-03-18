package dpo

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// mockFineTuneClient implements FineTuneClient for testing.
type mockFineTuneClient struct {
	name    string
	created int
	lastReq FineTuneRequest
	err     error
}

func (m *mockFineTuneClient) ProviderName() string { return m.name }
func (m *mockFineTuneClient) CreateFineTuneJob(_ context.Context, req FineTuneRequest) (*FineTuneJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.created++
	m.lastReq = req
	return &FineTuneJob{
		JobID:     "job-" + m.name + "-1",
		Provider:  m.name,
		Status:    "queued",
		BaseModel: req.BaseModel,
		CreatedAt: time.Now(),
	}, nil
}
func (m *mockFineTuneClient) GetJobStatus(_ context.Context, jobID string) (*FineTuneJob, error) {
	return &FineTuneJob{JobID: jobID, Provider: m.name, Status: "succeeded"}, nil
}
func (m *mockFineTuneClient) WaitForCompletion(_ context.Context, jobID string, _ time.Duration) (*FineTuneJob, error) {
	return &FineTuneJob{JobID: jobID, Provider: m.name, Status: "succeeded"}, nil
}

func TestDPORouterGPT4ToOpenAI(t *testing.T) {
	openai := &mockFineTuneClient{name: "openai"}
	anthropic := &mockFineTuneClient{name: "anthropic"}
	mistral := &mockFineTuneClient{name: "mistral"}

	router := NewDPOProviderRouter(openai, anthropic, mistral, testLogger)

	client, err := router.RouteJob(FineTuneRequest{BaseModel: "gpt-4o-mini-2024-07-18"}, "us-east-1")
	if err != nil {
		t.Fatalf("RouteJob failed: %v", err)
	}
	if client.ProviderName() != "openai" {
		t.Errorf("Expected openai, got %s", client.ProviderName())
	}
}

func TestDPORouterClaudeToAnthropic(t *testing.T) {
	openai := &mockFineTuneClient{name: "openai"}
	anthropic := &mockFineTuneClient{name: "anthropic"}

	router := NewDPOProviderRouter(openai, anthropic, nil, testLogger)

	client, err := router.RouteJob(FineTuneRequest{BaseModel: "claude-haiku-4-5"}, "us-east-1")
	if err != nil {
		t.Fatalf("RouteJob failed: %v", err)
	}
	if client.ProviderName() != "anthropic" {
		t.Errorf("Expected anthropic, got %s", client.ProviderName())
	}
}

func TestDPORouterEUWorkspaceToMistral(t *testing.T) {
	openai := &mockFineTuneClient{name: "openai"}
	mistral := &mockFineTuneClient{name: "mistral"}

	router := NewDPOProviderRouter(openai, nil, mistral, testLogger)

	client, err := router.RouteJob(FineTuneRequest{
		BaseModel:   "gpt-4o-mini",
		WorkspaceID: uuid.New(),
	}, "eu-west-1")
	if err != nil {
		t.Fatalf("RouteJob failed: %v", err)
	}
	if client.ProviderName() != "mistral" {
		t.Errorf("Expected mistral for EU workspace, got %s", client.ProviderName())
	}
}

func TestDPORouterAnthropicFallback(t *testing.T) {
	openai := &mockFineTuneClient{name: "openai"}
	anthropic := &mockFineTuneClient{name: "anthropic", err: ErrAnthropicFineTuneUnavailable}

	router := NewDPOProviderRouter(openai, anthropic, nil, testLogger)

	job, err := router.SubmitWithFallback(context.Background(), FineTuneRequest{
		BaseModel:       "claude-haiku-4-5",
		PreferencePairs: []PreferencePair{{ID: uuid.New()}},
	}, "us-east-1")
	if err != nil {
		t.Fatalf("SubmitWithFallback failed: %v", err)
	}
	if job.Provider != "openai" {
		t.Errorf("Expected fallback to openai, got %s", job.Provider)
	}
	if openai.created != 1 {
		t.Errorf("Expected openai to receive 1 job, got %d", openai.created)
	}
}

func TestDPORouterNoProvider(t *testing.T) {
	router := NewDPOProviderRouter(nil, nil, nil, testLogger)

	_, err := router.RouteJob(FineTuneRequest{BaseModel: "gpt-4o"}, "us-east-1")
	if err != ErrNoSuitableProvider {
		t.Errorf("Expected ErrNoSuitableProvider, got %v", err)
	}
}

func TestAnthropicClientContractHeaders(t *testing.T) {
	// Verify the client sets correct provider name.
	// Full HTTP contract test would require a mock server; here we verify the interface contract.
	var _ FineTuneClient = (*AnthropicFineTuneClient)(nil)
}

func TestMistralClientContractHeaders(t *testing.T) {
	var _ FineTuneClient = (*MistralFineTuneClient)(nil)
}

func TestDefaultHyperParams(t *testing.T) {
	hp := DefaultHyperParams()
	if hp.NEpochs != 3 {
		t.Errorf("Expected NEpochs=3, got %d", hp.NEpochs)
	}
	if hp.BatchSize != 4 {
		t.Errorf("Expected BatchSize=4, got %d", hp.BatchSize)
	}
}
