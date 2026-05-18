package cognition

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// CognitiveState represents the current cognitive state for a workspace.
type CognitiveState struct {
	WorkspaceID   string    `json:"workspace_id"`
	State         string    `json:"state"` // monitoring, reflecting, adjusting, stable, alert
	CognitiveLoad float64   `json:"cognitive_load"`
	Observations  []string  `json:"observations"`
	Timestamp     time.Time `json:"timestamp"`
	errorRate     float64
	latencyMs     float64
}

// StrategyAdjustment recommends strategy changes based on cognitive state.
type StrategyAdjustment struct {
	Action     string         `json:"action"`
	Reason     string         `json:"reason"`
	Parameters map[string]any `json:"parameters"`
}

// MetacognitiveMonitor assesses and manages cognitive states.
type MetacognitiveMonitor struct {
	mu     sync.Mutex
	states map[string]*CognitiveState
}

// NewMetacognitiveMonitor creates a new MetacognitiveMonitor.
func NewMetacognitiveMonitor() *MetacognitiveMonitor {
	return &MetacognitiveMonitor{
		states: make(map[string]*CognitiveState),
	}
}

// Monitor assesses the cognitive state for a workspace.
func (m *MetacognitiveMonitor) Monitor(workspaceID string, taskComplexity float64, errorRate float64, latencyMs float64) (*CognitiveState, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cognitiveLoad := computeCognitiveLoad(taskComplexity, errorRate, latencyMs)
	state := determineCognitiveState(cognitiveLoad, errorRate)

	cs := &CognitiveState{
		WorkspaceID:   workspaceID,
		State:         state,
		CognitiveLoad: cognitiveLoad,
		Observations:  []string{},
		Timestamp:     time.Now().UTC(),
		errorRate:     errorRate,
		latencyMs:     latencyMs,
	}

	existing, ok := m.states[workspaceID]
	if ok {
		cs.Observations = append(cs.Observations, existing.Observations...)
	}

	m.states[workspaceID] = cs
	return cs, nil
}

func computeCognitiveLoad(taskComplexity, errorRate, latencyMs float64) float64 {
	load := taskComplexity*0.4 + errorRate*0.35 + (latencyMs/5000.0)*0.25
	if load > 1.0 {
		load = 1.0
	}
	if load < 0.0 {
		load = 0.0
	}
	return load
}

func determineCognitiveState(cognitiveLoad, errorRate float64) string {
	if errorRate > 0.5 {
		return "alert"
	}
	if cognitiveLoad > 0.8 {
		return "reflecting"
	}
	if errorRate > 0.3 {
		return "adjusting"
	}
	if cognitiveLoad > 0.5 {
		return "monitoring"
	}
	return "stable"
}

// ShouldReflect determines whether reflection is warranted.
func (m *MetacognitiveMonitor) ShouldReflect(workspaceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	cs, ok := m.states[workspaceID]
	if !ok {
		return false
	}
	return cs.errorRate > 0.3 || cs.CognitiveLoad > 0.8
}

// GetState returns the current cognitive state for a workspace.
func (m *MetacognitiveMonitor) GetState(workspaceID string) (*CognitiveState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cs, ok := m.states[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no state found for workspace: %s", workspaceID)
	}
	return cs, nil
}

// RecordObservation adds an observation to the cognitive state.
func (m *MetacognitiveMonitor) RecordObservation(workspaceID, observation string) error {
	if strings.TrimSpace(observation) == "" {
		return fmt.Errorf("observation is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cs, ok := m.states[workspaceID]
	if !ok {
		return fmt.Errorf("no state found for workspace: %s", workspaceID)
	}

	cs.Observations = append(cs.Observations, observation)
	return nil
}

// AdjustStrategy recommends strategy changes based on the current cognitive state.
func (m *MetacognitiveMonitor) AdjustStrategy(workspaceID string) (*StrategyAdjustment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cs, ok := m.states[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no state found for workspace: %s", workspaceID)
	}

	switch cs.State {
	case "alert":
		return &StrategyAdjustment{
			Action: "reduce_complexity",
			Reason: "high error rate detected, simplifying approach",
			Parameters: map[string]any{
				"max_concurrent_tasks": 1,
				"use_fallback":         true,
			},
		}, nil
	case "reflecting":
		return &StrategyAdjustment{
			Action: "pause_and_review",
			Reason: "cognitive load is high, review current approach",
			Parameters: map[string]any{
				"review_interval_ms": 5000,
			},
		}, nil
	case "adjusting":
		return &StrategyAdjustment{
			Action: "increase_monitoring",
			Reason: "error rate is elevated, increasing observation frequency",
			Parameters: map[string]any{
				"monitor_frequency_ms": 2000,
			},
		}, nil
	default:
		return &StrategyAdjustment{
			Action: "maintain_course",
			Reason: "system is operating within normal parameters",
			Parameters: map[string]any{
				"status": "ok",
			},
		}, nil
	}
}
