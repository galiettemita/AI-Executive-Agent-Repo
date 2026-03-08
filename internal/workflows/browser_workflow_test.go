package workflows

import (
	"slices"
	"testing"
)

func TestBrowserWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:    "session-001",
		UserID:       "user-001",
		SkillID:      "browser.stealth_session",
		SessionType:  "stealth",
		UseProxy:     true,
		ProxyHealthy: true,
		TaskCount:    3,
	})

	if result.WorkflowID != "browser-session-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != BrowserStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	wantStates := []BrowserSessionState{
		BrowserStateInit, BrowserStateProvisioning, BrowserStateConfiguring,
		BrowserStateExecuting, BrowserStateCapturing, BrowserStateCleanup, BrowserStateCompleted,
	}
	if !slices.Equal(result.States, wantStates) {
		t.Fatalf("unexpected states: %v", result.States)
	}
	if len(result.Fallbacks) != 0 {
		t.Fatalf("unexpected fallbacks: %v", result.Fallbacks)
	}
}

func TestBrowserWorkflowProxyFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:    "session-002",
		UserID:       "user-001",
		SkillID:      "browser.web_scraper",
		SessionType:  "playwright",
		UseProxy:     true,
		ProxyHealthy: false,
	})

	if result.TerminalState != BrowserStateCompleted {
		t.Fatalf("expected COMPLETED with fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "direct_connection") {
		t.Fatalf("missing direct_connection fallback: %v", result.Fallbacks)
	}
}

func TestBrowserWorkflowTimeout(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:       "session-003",
		TimeoutExceeded: true,
	})

	if result.TerminalState != BrowserStateTimedOut {
		t.Fatalf("expected TIMED_OUT, got %s", result.TerminalState)
	}
}

func TestBrowserWorkflowCaptchaFailure(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:       "session-004",
		CaptchaRequired: true,
		CaptchaSolved:   false,
	})

	if result.TerminalState != BrowserStateFailed {
		t.Fatalf("expected FAILED, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "captcha_retry") {
		t.Fatalf("missing captcha_retry fallback: %v", result.Fallbacks)
	}
}

func TestBrowserWorkflowProvisionError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:      "session-005",
		ProvisionError: true,
	})

	if result.TerminalState != BrowserStateFailed {
		t.Fatalf("expected FAILED on provision error, got %s", result.TerminalState)
	}
	if !slices.Contains(result.States, BrowserStateProvisioning) {
		t.Fatalf("expected PROVISIONING state: %v", result.States)
	}
}

func TestBrowserWorkflowExecuteError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:    "session-006",
		ExecuteError: true,
	})

	if result.TerminalState != BrowserStateFailed {
		t.Fatalf("expected FAILED on execute error, got %s", result.TerminalState)
	}
}

func TestBrowserWorkflowConfigErrorWithFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:   "session-007",
		ConfigError: true,
	})

	if result.TerminalState != BrowserStateCompleted {
		t.Fatalf("expected COMPLETED with config fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "default_config") {
		t.Fatalf("expected default_config fallback: %v", result.Fallbacks)
	}
}

func TestBrowserWorkflowCaptchaSolvedSuccess(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID:       "session-008",
		CaptchaRequired: true,
		CaptchaSolved:   true,
	})

	if result.TerminalState != BrowserStateCompleted {
		t.Fatalf("expected COMPLETED when captcha solved, got %s", result.TerminalState)
	}
}

func TestBrowserWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	id := BrowserWorkflowID("  session-padded  ")
	if id != "browser-session-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}

func TestBrowserWorkflowEmptyFallbacks(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.BrowserSessionWorkflowV1(BrowserWorkflowInput{
		SessionID: "session-009",
	})

	if result.Fallbacks == nil {
		t.Fatal("Fallbacks should be initialized, not nil")
	}
}
