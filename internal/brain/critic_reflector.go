package brain

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CriticOutput is the structured output from the CRITIC stage.
type CriticOutput struct {
	ID                   string             `json:"id"`
	OverallScore         float64            `json:"overall_score"`
	DimensionScores      map[string]float64 `json:"dimension_scores"`
	Passed               bool               `json:"passed"`
	FailureModes         []string           `json:"failure_modes"`
	ImprovementDirective string             `json:"improvement_directive"`
	CreatedAt            time.Time          `json:"created_at"`
}

// LessonCandidate represents a potential lesson extracted by the reflector.
type LessonCandidate struct {
	Topic      string `json:"topic"`
	Lesson     string `json:"lesson"`
	Confidence float64 `json:"confidence"`
}

// ReflectorOutput is the structured output from the REFLECTOR stage.
type ReflectorOutput struct {
	LessonCandidates   []LessonCandidate `json:"lesson_candidates"`
	PatternDetected    bool              `json:"pattern_detected"`
	EscalateToFeedback bool              `json:"escalate_to_feedback"`
}

// ExecutionTrace captures the data needed for critic/reflector analysis.
type ExecutionTrace struct {
	WorkspaceID string            `json:"workspace_id"`
	Intent      string            `json:"intent"`
	PlanSteps   int               `json:"plan_steps"`
	Succeeded   int               `json:"succeeded"`
	Failed      int               `json:"failed"`
	ToolsUsed   []string          `json:"tools_used"`
	Duration    time.Duration     `json:"duration"`
	Metadata    map[string]string `json:"metadata"`
}

// CriticReflectorService implements the CRITIC and REFLECTOR intelligence modules.
type CriticReflectorService struct {
	mu           sync.Mutex
	history      []CriticOutput
	passThreshold float64
	now          func() time.Time
}

// NewCriticReflectorService creates a new critic/reflector service.
func NewCriticReflectorService() *CriticReflectorService {
	return &CriticReflectorService{
		history:       []CriticOutput{},
		passThreshold: 0.7,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// Critique evaluates an execution trace and produces a CriticOutput.
func (s *CriticReflectorService) Critique(trace ExecutionTrace) (*CriticOutput, error) {
	if trace.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	dimensions := map[string]float64{}

	// Completeness: ratio of succeeded to total steps.
	if trace.PlanSteps > 0 {
		dimensions["completeness"] = float64(trace.Succeeded) / float64(trace.PlanSteps)
	} else {
		dimensions["completeness"] = 0
	}

	// Efficiency: penalize if too many tools for simple tasks.
	if trace.PlanSteps <= 3 {
		dimensions["efficiency"] = 1.0
	} else if trace.PlanSteps <= 6 {
		dimensions["efficiency"] = 0.8
	} else {
		dimensions["efficiency"] = 0.6
	}

	// Reliability: inverse of failure rate.
	if trace.PlanSteps > 0 {
		dimensions["reliability"] = 1.0 - (float64(trace.Failed) / float64(trace.PlanSteps))
	} else {
		dimensions["reliability"] = 0
	}

	// Speed: score based on duration (under 5s = 1.0, 5-30s = 0.7, >30s = 0.4).
	switch {
	case trace.Duration < 5*time.Second:
		dimensions["speed"] = 1.0
	case trace.Duration < 30*time.Second:
		dimensions["speed"] = 0.7
	default:
		dimensions["speed"] = 0.4
	}

	// Overall score: weighted average.
	overall := dimensions["completeness"]*0.4 +
		dimensions["reliability"]*0.3 +
		dimensions["efficiency"]*0.2 +
		dimensions["speed"]*0.1

	var failureModes []string
	if dimensions["completeness"] < 0.5 {
		failureModes = append(failureModes, "low_completeness")
	}
	if dimensions["reliability"] < 0.5 {
		failureModes = append(failureModes, "low_reliability")
	}
	if trace.Failed > 0 {
		failureModes = append(failureModes, fmt.Sprintf("%d_steps_failed", trace.Failed))
	}

	directive := ""
	if overall < s.passThreshold {
		if dimensions["completeness"] < 0.5 {
			directive = "reduce plan complexity or add fallback steps"
		} else if dimensions["reliability"] < 0.5 {
			directive = "improve tool reliability or add retry logic"
		} else {
			directive = "general quality improvement needed"
		}
	}

	output := &CriticOutput{
		ID:                   uuid.Must(uuid.NewV7()).String(),
		OverallScore:         overall,
		DimensionScores:      dimensions,
		Passed:               overall >= s.passThreshold,
		FailureModes:         failureModes,
		ImprovementDirective: directive,
		CreatedAt:            s.now(),
	}

	s.mu.Lock()
	s.history = append(s.history, *output)
	s.mu.Unlock()

	return output, nil
}

// Reflect analyses a critic output and execution trace to extract lessons.
func (s *CriticReflectorService) Reflect(criticOutput *CriticOutput, trace ExecutionTrace) (*ReflectorOutput, error) {
	if criticOutput == nil {
		return nil, fmt.Errorf("critic output is required")
	}

	output := &ReflectorOutput{}

	// Extract lessons from failure modes.
	for _, mode := range criticOutput.FailureModes {
		if strings.Contains(mode, "completeness") {
			output.LessonCandidates = append(output.LessonCandidates, LessonCandidate{
				Topic:      "plan_design",
				Lesson:     "Plans with many steps have higher failure rates; consider decomposition",
				Confidence: 0.8,
			})
		}
		if strings.Contains(mode, "reliability") {
			output.LessonCandidates = append(output.LessonCandidates, LessonCandidate{
				Topic:      "tool_reliability",
				Lesson:     "Unreliable tool calls detected; add retry or fallback strategies",
				Confidence: 0.9,
			})
		}
	}

	// Pattern detection: if the same tools keep failing, flag a pattern.
	if trace.Failed > 1 {
		output.PatternDetected = true
		output.LessonCandidates = append(output.LessonCandidates, LessonCandidate{
			Topic:      "recurring_failure",
			Lesson:     fmt.Sprintf("Multiple failures (%d) detected in single execution", trace.Failed),
			Confidence: 0.7,
		})
	}

	// Escalate to feedback if score is very low.
	if criticOutput.OverallScore < 0.3 {
		output.EscalateToFeedback = true
	}

	return output, nil
}

// GetCritiqueHistory returns all stored critique outputs.
func (s *CriticReflectorService) GetCritiqueHistory() []CriticOutput {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]CriticOutput, len(s.history))
	copy(out, s.history)
	return out
}
