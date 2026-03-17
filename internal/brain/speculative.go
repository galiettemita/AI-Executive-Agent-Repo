// Package brain provides the cognitive core for Brevio agents.
// This file implements speculative tool execution (Plan 07):
// pre-executing high-confidence predicted next tool calls to hide latency.
//
// Research basis: speculative execution pattern, analogous to CPU branch
// prediction. Confidence threshold 0.60 per Plan 07 specification.
package brain

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ToolExecutor is already declared in reasoning.go — not redeclared here.
// Its signature: Execute(ctx context.Context, toolKey string, params map[string]any) (map[string]any, error)

// SpeculativePrediction is a predicted next tool call and its confidence score.
// Produced by PredictFollowUpTools and consumed by PreExecute.
type SpeculativePrediction struct {
	ToolKey    string         `json:"tool_key"`
	Input      map[string]any `json:"input"`
	Confidence float64        `json:"confidence"` // 0.0–1.0; plan threshold: 0.60
	Rationale  string         `json:"rationale"`
}

// SpeculativeResult is the outcome of a pre-executed speculative tool call.
// A zero Output and nil Error means the goroutine is still in-flight.
type SpeculativeResult struct {
	ToolKey   string         `json:"tool_key"`
	Input     map[string]any `json:"input"`
	Output    any            `json:"output"`
	Error     error          `json:"-"`
	LatencyMs int64          `json:"latency_ms"`
	Used      bool           `json:"used"`
}

// SpeculativeExecutor pre-executes high-confidence predicted tool calls in the
// background, so results are available instantly when the actual call arrives.
// All exported methods are safe for concurrent use.
type SpeculativeExecutor struct {
	mu            sync.Mutex
	pending       map[string]*SpeculativeResult
	toolExecutor  ToolExecutor
	minConfidence float64
}

// NewSpeculativeExecutor creates a SpeculativeExecutor ready for use.
// minConfidence is the minimum confidence score required to trigger
// pre-execution. Plan 07 specifies 0.60 at all call sites.
func NewSpeculativeExecutor(executor ToolExecutor, minConfidence float64) *SpeculativeExecutor {
	return &SpeculativeExecutor{
		pending:       make(map[string]*SpeculativeResult),
		toolExecutor:  executor,
		minConfidence: minConfidence,
	}
}

// speculativeKey returns a deterministic, collision-resistant cache key for the
// (toolKey, input) pair. Uses SHA-256 over the JSON-serialised input so that
// semantically identical inputs produce the same key regardless of map iteration
// order. json.Marshal sorts struct fields deterministically.
func speculativeKey(toolKey string, input map[string]any) string {
	b, err := json.Marshal(input)
	if err != nil {
		return fmt.Sprintf("%s::marshal_err", toolKey)
	}
	payload := append([]byte(toolKey+":"), b...)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum)
}

// PreExecute evaluates predictions and fires a background goroutine for each
// prediction whose Confidence >= minConfidence and that is not already pending.
func (e *SpeculativeExecutor) PreExecute(ctx context.Context, predictions []SpeculativePrediction) {
	e.mu.Lock()
	var toFire []SpeculativePrediction
	for _, pred := range predictions {
		if pred.Confidence < e.minConfidence {
			continue
		}
		key := speculativeKey(pred.ToolKey, pred.Input)
		if _, exists := e.pending[key]; exists {
			continue
		}
		// Reserve the slot under lock to block duplicate goroutines.
		e.pending[key] = &SpeculativeResult{
			ToolKey: pred.ToolKey,
			Input:   pred.Input,
		}
		toFire = append(toFire, pred)
	}
	e.mu.Unlock()

	for _, pred := range toFire {
		pred := pred // capture loop variable before goroutine
		key := speculativeKey(pred.ToolKey, pred.Input)
		go func() {
			start := time.Now()
			// Independent context: survives parent request cancellation.
			execCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			out, err := e.toolExecutor.Execute(execCtx, pred.ToolKey, pred.Input)
			latencyMs := time.Since(start).Milliseconds()

			e.mu.Lock()
			e.pending[key] = &SpeculativeResult{
				ToolKey:   pred.ToolKey,
				Input:     pred.Input,
				Output:    out,
				Error:     err,
				LatencyMs: latencyMs,
				Used:      false,
			}
			e.mu.Unlock()
		}()
	}
}

// Consume retrieves a pre-computed result for (toolKey, input) if it is ready.
// Returns (nil, false) if the key is unknown or the goroutine is still in-flight.
// On a hit, the result is removed from pending and marked Used=true.
func (e *SpeculativeExecutor) Consume(toolKey string, input map[string]any) (*SpeculativeResult, bool) {
	key := speculativeKey(toolKey, input)

	e.mu.Lock()
	defer e.mu.Unlock()

	r, ok := e.pending[key]
	if !ok {
		return nil, false
	}
	if r.Output == nil && r.Error == nil {
		return nil, false // goroutine still in-flight
	}

	r.Used = true
	delete(e.pending, key)
	return r, true
}

// PredictFollowUpTools returns predicted next tool calls for the given
// completed tool key. Confidence values and tool pairings are exactly as
// specified in Plan 07 Section 6 Step 5.
func PredictFollowUpTools(completedToolKey string) []SpeculativePrediction {
	switch completedToolKey {
	case "google_calendar_read", "calendar_read":
		return []SpeculativePrediction{{
			ToolKey:    "email_read",
			Input:      map[string]any{},
			Confidence: 0.65,
			Rationale:  "calendar read commonly precedes inbox check",
		}}
	case "google_gmail_read", "email_read":
		return []SpeculativePrediction{{
			ToolKey:    "calendar_read",
			Input:      map[string]any{},
			Confidence: 0.55,
			Rationale:  "email read commonly precedes calendar check",
		}}
	case "brave_search", "web_search":
		return []SpeculativePrediction{{
			ToolKey:    "exa",
			Input:      map[string]any{},
			Confidence: 0.50,
			Rationale:  "web search commonly precedes deep search for corroboration",
		}}
	default:
		return nil
	}
}
