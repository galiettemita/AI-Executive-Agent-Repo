package workflows

import "strings"

type BrowserSessionState string

const (
	BrowserStateInit        BrowserSessionState = "INIT"
	BrowserStateProvisioning BrowserSessionState = "PROVISIONING"
	BrowserStateConfiguring BrowserSessionState = "CONFIGURING"
	BrowserStateExecuting   BrowserSessionState = "EXECUTING"
	BrowserStateCapturing   BrowserSessionState = "CAPTURING"
	BrowserStateCleanup     BrowserSessionState = "CLEANUP"
	BrowserStateCompleted   BrowserSessionState = "COMPLETED"
	BrowserStateFailed      BrowserSessionState = "FAILED"
	BrowserStateTimedOut    BrowserSessionState = "TIMED_OUT"
)

type BrowserWorkflowInput struct {
	SessionID        string
	UserID           string
	SkillID          string
	SessionType      string
	UseProxy         bool
	UseFingerprint   bool
	ProxyHealthy     bool
	ProvisionError   bool
	ConfigError      bool
	ExecuteError     bool
	CaptchaRequired  bool
	CaptchaSolved    bool
	TimeoutExceeded  bool
	TaskCount        int
}

type BrowserWorkflowResult struct {
	WorkflowID    string
	States        []BrowserSessionState
	TerminalState BrowserSessionState
	Fallbacks     []string
}

func BrowserWorkflowID(sessionID string) string {
	return "browser-" + strings.TrimSpace(sessionID)
}

func (s *Service) BrowserSessionWorkflowV1(input BrowserWorkflowInput) BrowserWorkflowResult {
	workflowID := BrowserWorkflowID(input.SessionID)
	result := BrowserWorkflowResult{
		WorkflowID: workflowID,
		States:     []BrowserSessionState{BrowserStateInit},
		Fallbacks:  []string{},
	}

	if input.TimeoutExceeded {
		result.States = append(result.States, BrowserStateTimedOut)
		result.TerminalState = BrowserStateTimedOut
		return result
	}

	result.States = append(result.States, BrowserStateProvisioning)
	if input.ProvisionError {
		result.States = append(result.States, BrowserStateFailed)
		result.TerminalState = BrowserStateFailed
		return result
	}

	result.States = append(result.States, BrowserStateConfiguring)
	if input.UseProxy && !input.ProxyHealthy {
		result.Fallbacks = append(result.Fallbacks, "direct_connection")
	}
	if input.ConfigError {
		result.Fallbacks = append(result.Fallbacks, "default_config")
	}

	result.States = append(result.States, BrowserStateExecuting)
	if input.CaptchaRequired && !input.CaptchaSolved {
		result.Fallbacks = append(result.Fallbacks, "captcha_retry")
		result.States = append(result.States, BrowserStateFailed)
		result.TerminalState = BrowserStateFailed
		return result
	}
	if input.ExecuteError {
		result.States = append(result.States, BrowserStateFailed)
		result.TerminalState = BrowserStateFailed
		return result
	}

	result.States = append(result.States, BrowserStateCapturing)
	result.States = append(result.States, BrowserStateCleanup)
	result.States = append(result.States, BrowserStateCompleted)
	result.TerminalState = BrowserStateCompleted
	return result
}
