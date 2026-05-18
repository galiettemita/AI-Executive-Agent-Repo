package cognitive

import (
	"sync"
	"time"
)

// CognitiveState captures the monitored cognitive state for a workspace.
type CognitiveState struct {
	WorkspaceID      string
	CognitiveLoad    float64 // 0-1
	ReasoningQuality float64 // 0-1
	UncertaintyLevel float64 // 0-1
	State            string  // monitoring, reflecting, adjusting, stable, alert
	Observations     []string
	Timestamp        time.Time
}

// MetacognitiveMonitor observes and regulates the system's own cognitive processes.
type MetacognitiveMonitor struct {
	mu           sync.RWMutex
	history      map[string][]CognitiveState
	observations map[string][]string
}

// NewMetacognitiveMonitor creates a new MetacognitiveMonitor.
func NewMetacognitiveMonitor() *MetacognitiveMonitor {
	return &MetacognitiveMonitor{
		history:      make(map[string][]CognitiveState),
		observations: make(map[string][]string),
	}
}

// Monitor evaluates the cognitive state given task metrics.
func (m *MetacognitiveMonitor) Monitor(workspaceID string, taskComplexity float64, reasoningSteps int, errors int) *CognitiveState {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cognitive load increases with complexity and reasoning steps.
	load := taskComplexity * 0.6
	if reasoningSteps > 0 {
		stepLoad := float64(reasoningSteps) / 20.0
		if stepLoad > 0.4 {
			stepLoad = 0.4
		}
		load += stepLoad
	}
	if load > 1.0 {
		load = 1.0
	}

	// Reasoning quality decreases with errors.
	quality := 1.0
	if reasoningSteps > 0 {
		quality = 1.0 - (float64(errors) / float64(reasoningSteps))
	}
	if quality < 0.0 {
		quality = 0.0
	}

	// Uncertainty rises with load and errors.
	uncertainty := (load*0.3 + (1.0-quality)*0.7)
	if uncertainty > 1.0 {
		uncertainty = 1.0
	}

	state := "monitoring"
	if load > 0.8 {
		state = "alert"
	} else if quality < 0.5 {
		state = "reflecting"
	} else if uncertainty > 0.6 {
		state = "adjusting"
	} else if load < 0.4 && quality > 0.8 {
		state = "stable"
	}

	obs := m.observations[workspaceID]

	cs := CognitiveState{
		WorkspaceID:      workspaceID,
		CognitiveLoad:    load,
		ReasoningQuality: quality,
		UncertaintyLevel: uncertainty,
		State:            state,
		Observations:     append([]string{}, obs...),
		Timestamp:        time.Now(),
	}

	m.history[workspaceID] = append(m.history[workspaceID], cs)

	return &cs
}

// ShouldEscalate returns true if cognitive state exceeds escalation thresholds.
func (m *MetacognitiveMonitor) ShouldEscalate(state *CognitiveState) bool {
	return state.CognitiveLoad > 0.8 || state.ReasoningQuality < 0.5 || state.UncertaintyLevel > 0.7
}

// SuggestStrategy recommends a strategy based on the current cognitive state.
func (m *MetacognitiveMonitor) SuggestStrategy(state *CognitiveState) string {
	if state.CognitiveLoad > 0.9 && state.ReasoningQuality < 0.3 {
		return "abort"
	}
	if state.CognitiveLoad > 0.8 {
		return "simplify"
	}
	if state.UncertaintyLevel > 0.7 {
		return "seek_clarification"
	}
	if state.ReasoningQuality < 0.5 {
		return "decompose"
	}
	return "proceed"
}

// RecordObservation adds an observation for a workspace.
func (m *MetacognitiveMonitor) RecordObservation(workspaceID, observation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.observations[workspaceID] = append(m.observations[workspaceID], observation)
}

// GetCognitiveHistory returns the most recent cognitive states for a workspace.
func (m *MetacognitiveMonitor) GetCognitiveHistory(workspaceID string, limit int) []CognitiveState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.history[workspaceID]
	if len(history) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}
	start := len(history) - limit
	result := make([]CognitiveState, limit)
	copy(result, history[start:])
	return result
}
