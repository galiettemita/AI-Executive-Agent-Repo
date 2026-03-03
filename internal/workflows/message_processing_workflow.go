package workflows

import (
	"fmt"
	"hash/fnv"
	"strings"
)

type MessageProcessingState string

const (
	MessageStateReceived    MessageProcessingState = "RECEIVED"
	MessageStateClassifying MessageProcessingState = "CLASSIFYING"
	MessageStateDecomposing MessageProcessingState = "DECOMPOSING"
	MessageStateExecuting   MessageProcessingState = "EXECUTING"
	MessageStateAggregating MessageProcessingState = "AGGREGATING"
	MessageStateFormatting  MessageProcessingState = "FORMATTING"
	MessageStateDelivering  MessageProcessingState = "DELIVERING"
	MessageStateCompleted   MessageProcessingState = "COMPLETED"
	MessageStateFailed      MessageProcessingState = "FAILED"
	MessageStateDeadLetter  MessageProcessingState = "DEAD_LETTER"
)

type MessageProcessingInput struct {
	MessageID                        string
	WorkflowRunID                    string
	EnvelopeValid                    bool
	ClassifyError                    bool
	ClassifyConfidence               float64
	KeywordFallbackAvailable         bool
	DecomposeError                   bool
	DAGValid                         bool
	ExecuteError                     bool
	RequiresCompensation             bool
	DeliveryFailures                 int
	FormattingError                  bool
	AggregateError                   bool
	DeliveryFallbackQueueAllowed     bool
	ForceDeadLetterOnEnvelopeFailure bool
}

type MessageProcessingResult struct {
	WorkflowID         string
	States             []MessageProcessingState
	TerminalState      MessageProcessingState
	Fallbacks          []string
	CompensationNeeded bool
}

type DailyRhythmState string

const (
	DailyRhythmStateInit       DailyRhythmState = "INIT"
	DailyRhythmStateComposing  DailyRhythmState = "COMPOSING"
	DailyRhythmStateDelivering DailyRhythmState = "DELIVERING"
	DailyRhythmStateCompleted  DailyRhythmState = "COMPLETED"
	DailyRhythmStateFailed     DailyRhythmState = "FAILED"
)

type DailyRhythmInput struct {
	UserID              string
	HasProfile          bool
	HasScheduleContext  bool
	ComposeFailed       bool
	DeliveryFailures    int
	DeliveryRetryBudget int
}

type DailyRhythmResult struct {
	WorkflowID    string
	States        []DailyRhythmState
	TerminalState DailyRhythmState
}

func MessageWorkflowID(messageID string) string {
	return "msg-" + strings.TrimSpace(messageID)
}

func (s *Service) MessageProcessingWorkflowV1(input MessageProcessingInput) MessageProcessingResult {
	workflowID := MessageWorkflowID(input.MessageID)
	result := MessageProcessingResult{
		WorkflowID: workflowID,
		States:     []MessageProcessingState{MessageStateReceived},
		Fallbacks:  []string{},
	}

	if !input.EnvelopeValid {
		if input.ForceDeadLetterOnEnvelopeFailure {
			result.States = append(result.States, MessageStateDeadLetter)
			result.TerminalState = MessageStateDeadLetter
			return result
		}
		result.States = append(result.States, MessageStateFailed)
		result.TerminalState = MessageStateFailed
		return result
	}

	result.States = append(result.States, MessageStateClassifying)
	if input.ClassifyError {
		if input.KeywordFallbackAvailable {
			result.Fallbacks = append(result.Fallbacks, "keyword_classifier")
		} else {
			result.States = append(result.States, MessageStateFailed)
			result.TerminalState = MessageStateFailed
			return result
		}
	} else if input.ClassifyConfidence < 0.7 {
		if input.KeywordFallbackAvailable {
			result.Fallbacks = append(result.Fallbacks, "keyword_classifier_low_confidence")
		} else {
			result.States = append(result.States, MessageStateFailed)
			result.TerminalState = MessageStateFailed
			return result
		}
	}

	result.States = append(result.States, MessageStateDecomposing)
	if input.DecomposeError || !input.DAGValid {
		result.Fallbacks = append(result.Fallbacks, "single_skill_fallback")
	}

	result.States = append(result.States, MessageStateExecuting)
	if input.ExecuteError {
		if input.RequiresCompensation {
			result.CompensationNeeded = true
		}
		result.States = append(result.States, MessageStateFailed)
		result.TerminalState = MessageStateFailed
		return result
	}

	result.States = append(result.States, MessageStateAggregating)
	if input.AggregateError {
		result.States = append(result.States, MessageStateFailed)
		result.TerminalState = MessageStateFailed
		return result
	}

	result.States = append(result.States, MessageStateFormatting)
	if input.FormattingError {
		result.States = append(result.States, MessageStateFailed)
		result.TerminalState = MessageStateFailed
		return result
	}

	result.States = append(result.States, MessageStateDelivering)
	if input.DeliveryFailures >= 3 {
		if input.DeliveryFallbackQueueAllowed {
			result.Fallbacks = append(result.Fallbacks, "delivery_dlq")
			result.States = append(result.States, MessageStateDeadLetter)
			result.TerminalState = MessageStateDeadLetter
			return result
		}
		result.States = append(result.States, MessageStateFailed)
		result.TerminalState = MessageStateFailed
		return result
	}

	result.States = append(result.States, MessageStateCompleted)
	result.TerminalState = MessageStateCompleted
	return result
}

func (s *Service) DailyRhythmWorkflowV1(input DailyRhythmInput) DailyRhythmResult {
	workflowID := "daily-rhythm-" + strings.TrimSpace(input.UserID)
	result := DailyRhythmResult{
		WorkflowID: workflowID,
		States:     []DailyRhythmState{DailyRhythmStateInit},
	}
	if !input.HasProfile || !input.HasScheduleContext {
		result.States = append(result.States, DailyRhythmStateFailed)
		result.TerminalState = DailyRhythmStateFailed
		return result
	}

	result.States = append(result.States, DailyRhythmStateComposing)
	if input.ComposeFailed {
		result.States = append(result.States, DailyRhythmStateFailed)
		result.TerminalState = DailyRhythmStateFailed
		return result
	}

	result.States = append(result.States, DailyRhythmStateDelivering)
	retryBudget := input.DeliveryRetryBudget
	if retryBudget <= 0 {
		retryBudget = 3
	}
	if input.DeliveryFailures >= retryBudget {
		result.States = append(result.States, DailyRhythmStateFailed)
		result.TerminalState = DailyRhythmStateFailed
		return result
	}

	result.States = append(result.States, DailyRhythmStateCompleted)
	result.TerminalState = DailyRhythmStateCompleted
	return result
}

func DeterministicRetryJitterMS(workflowRunID string, attemptNumber int, maxJitterMS int) int {
	if maxJitterMS <= 0 {
		return 0
	}
	key := fmt.Sprintf("%s:%d", strings.TrimSpace(workflowRunID), attemptNumber)
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return int(hasher.Sum32() % uint32(maxJitterMS))
}
