package mcp

import (
	"testing"
	"time"
)

func TestRegistrySupportsNativeAndMCPSingleSource(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if err := svc.RegisterTool(ToolSpec{
		ToolKey:   "calendar.create_event",
		Source:    ToolSourceNative,
		RiskLevel: "MEDIUM",
	}); err != nil {
		t.Fatalf("register native tool: %v", err)
	}
	if err := svc.RegisterTool(ToolSpec{
		ToolKey:   "stripe.create_payment",
		Source:    ToolSourceMCP,
		ServerID:  "stripe_mcp",
		AuthType:  AuthOAuth2,
		RiskLevel: "CRITICAL",
	}); err != nil {
		t.Fatalf("register mcp tool: %v", err)
	}

	tools := svc.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tool specs, got %d", len(tools))
	}
	spec, ok := svc.ResolveTool("stripe.create_payment")
	if !ok {
		t.Fatal("expected mcp tool to resolve")
	}
	if spec.Source != ToolSourceMCP || spec.ServerID != "stripe_mcp" {
		t.Fatalf("unexpected mcp tool spec: %+v", spec)
	}
}

func TestAuthMatrixCoverageForMCPServers(t *testing.T) {
	t.Parallel()

	svc := NewService()
	for _, spec := range []ToolSpec{
		{ToolKey: "slack.post_message", Source: ToolSourceMCP, ServerID: "slack_mcp", AuthType: AuthIntegrationToken, RiskLevel: "LOW"},
		{ToolKey: "plaid.fetch_transactions", Source: ToolSourceMCP, ServerID: "plaid_mcp", AuthType: AuthAPIKey, RiskLevel: "CRITICAL"},
		{ToolKey: "zoom.fetch_transcript", Source: ToolSourceMCP, ServerID: "zoom_mcp", AuthType: AuthPAT, RiskLevel: "MEDIUM"},
		{ToolKey: "google_calendar.list_events", Source: ToolSourceMCP, ServerID: "google_calendar_mcp", AuthType: AuthOAuth2, RiskLevel: "MEDIUM"},
	} {
		if err := svc.RegisterTool(spec); err != nil {
			t.Fatalf("register mcp tool %s: %v", spec.ToolKey, err)
		}
	}

	coverage := svc.AuthMatrixCoverage()
	for _, authType := range []AuthType{AuthOAuth2, AuthAPIKey, AuthPAT, AuthIntegrationToken} {
		if !coverage[authType] {
			t.Fatalf("expected auth matrix coverage for %s", authType)
		}
	}
}

func TestPerServerBudgetAndRateLimitEnforcement(t *testing.T) {
	t.Parallel()

	svc := NewService()
	now := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	if err := svc.ConfigureServerPolicy(ServerPolicy{
		ServerID:           "stripe_mcp",
		MonthlyCallCap:     2,
		MonthlyCostCapUSD:  0.03,
		RateLimitPerMinute: 1,
	}); err != nil {
		t.Fatalf("configure policy: %v", err)
	}

	if err := svc.EnforceServerPolicy("stripe_mcp", 0.01, now); err != nil {
		t.Fatalf("first enforce should pass: %v", err)
	}
	if err := svc.RecordInvocation(Invocation{
		WorkspaceID: "ws_1",
		ToolKey:     "stripe.create_payment",
		ServerID:    "stripe_mcp",
		IsMCP:       true,
		CostUSD:     0.01,
		CalledAt:    now,
	}); err != nil {
		t.Fatalf("first record invocation: %v", err)
	}

	if err := svc.EnforceServerPolicy("stripe_mcp", 0.01, now.Add(30*time.Second)); err == nil {
		t.Fatal("expected minute rate limit breach")
	}

	if err := svc.EnforceServerPolicy("stripe_mcp", 0.03, now.Add(time.Minute)); err == nil {
		t.Fatal("expected monthly budget breach")
	}
}

func TestMCPInvocationProvenanceDefaultsToMCPResult(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if err := svc.RecordInvocation(Invocation{
		WorkspaceID: "ws_1",
		ToolKey:     "stripe.create_payment",
		ServerID:    "stripe_mcp",
		IsMCP:       true,
		CostUSD:     0.01,
	}); err != nil {
		t.Fatalf("record invocation: %v", err)
	}

	invocations := svc.Invocations()
	if len(invocations) != 1 {
		t.Fatalf("expected one invocation, got %d", len(invocations))
	}
	if invocations[0].ContentProvenance != "mcp_result" {
		t.Fatalf("expected mcp_result provenance, got %s", invocations[0].ContentProvenance)
	}
}
