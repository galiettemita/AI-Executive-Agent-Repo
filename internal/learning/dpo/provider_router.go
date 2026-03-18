package dpo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// DPOProviderRouter selects the appropriate fine-tuning provider based on
// model name, workspace region, and privacy settings.
type DPOProviderRouter struct {
	openai    FineTuneClient
	anthropic FineTuneClient
	mistral   FineTuneClient
	logger    *slog.Logger
}

// NewDPOProviderRouter creates a router with optional provider clients.
// Nil clients are skipped during routing.
func NewDPOProviderRouter(openai, anthropic, mistral FineTuneClient, logger *slog.Logger) *DPOProviderRouter {
	return &DPOProviderRouter{
		openai:    openai,
		anthropic: anthropic,
		mistral:   mistral,
		logger:    logger,
	}
}

// RouteJob determines the best provider for the given request.
func (r *DPOProviderRouter) RouteJob(req FineTuneRequest, workspaceRegion string) (FineTuneClient, error) {
	model := strings.ToLower(req.BaseModel)

	// EU workspaces prefer Mistral for data residency.
	if workspaceRegion == "eu-west-1" && r.mistral != nil {
		r.logger.Info("dpo_route_eu_mistral",
			"workspace_id", req.WorkspaceID,
			"model", req.BaseModel,
			"reason", "eu_data_residency",
		)
		return r.mistral, nil
	}

	// Route by model prefix.
	switch {
	case strings.HasPrefix(model, "gpt-4") || strings.HasPrefix(model, "gpt-3.5"):
		if r.openai != nil {
			return r.openai, nil
		}
	case strings.HasPrefix(model, "claude"):
		if r.anthropic != nil {
			return r.anthropic, nil
		}
		// Fallback to OpenAI if Anthropic unavailable.
		if r.openai != nil {
			r.logger.Info("dpo_route_anthropic_fallback_openai",
				"model", req.BaseModel,
				"reason", "anthropic_client_nil",
			)
			return r.openai, nil
		}
	case strings.HasPrefix(model, "mistral") || strings.HasPrefix(model, "mixtral"):
		if r.mistral != nil {
			return r.mistral, nil
		}
	}

	// Default: OpenAI.
	if r.openai != nil {
		return r.openai, nil
	}

	return nil, ErrNoSuitableProvider
}

// SubmitWithFallback routes the job and handles provider-level fallback.
func (r *DPOProviderRouter) SubmitWithFallback(ctx context.Context, req FineTuneRequest, workspaceRegion string) (*FineTuneJob, error) {
	client, err := r.RouteJob(req, workspaceRegion)
	if err != nil {
		return nil, err
	}

	r.logger.Info("dpo_submit_job",
		"provider", client.ProviderName(),
		"model", req.BaseModel,
		"workspace_id", req.WorkspaceID,
		"pairs", len(req.PreferencePairs),
	)

	job, err := client.CreateFineTuneJob(ctx, req)
	if err != nil {
		// Fallback: if Anthropic fails, try OpenAI.
		if client.ProviderName() == "anthropic" && r.openai != nil &&
			(errors.Is(err, ErrAnthropicFineTuneUnavailable) || errors.Is(err, ErrAnthropicFineTuneDisabled)) {
			r.logger.Info("dpo_anthropic_fallback_openai",
				"original_error", err.Error(),
			)
			return r.openai.CreateFineTuneJob(ctx, req)
		}
		return nil, fmt.Errorf("create fine-tune job via %s: %w", client.ProviderName(), err)
	}

	return job, nil
}
