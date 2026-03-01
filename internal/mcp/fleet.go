package mcp

import (
	"fmt"
	"sort"
	"sync"
)

type FleetVerifier struct {
	mu         sync.Mutex
	statuses   map[string]string
	callCounts map[string]int
}

func NewFleetVerifier(serverIDs []string) *FleetVerifier {
	statuses := make(map[string]string, len(serverIDs))
	callCounts := make(map[string]int, len(serverIDs))
	for _, serverID := range serverIDs {
		statuses[serverID] = "healthy"
		callCounts[serverID] = 0
	}
	return &FleetVerifier{
		statuses:   statuses,
		callCounts: callCounts,
	}
}

func (v *FleetVerifier) VerifyAllHealthy() (bool, []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	unhealthy := make([]string, 0)
	for serverID, status := range v.statuses {
		if status != "healthy" {
			unhealthy = append(unhealthy, serverID)
		}
	}
	sort.Strings(unhealthy)
	return len(unhealthy) == 0, unhealthy
}

func (v *FleetVerifier) RecordCall(serverID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	status, ok := v.statuses[serverID]
	if !ok {
		return fmt.Errorf("unknown server: %s", serverID)
	}
	if status != "healthy" {
		return fmt.Errorf("server unavailable: %s status=%s", serverID, status)
	}
	v.callCounts[serverID]++
	return nil
}

func (v *FleetVerifier) SimulateFailures(serverIDs []string) map[string]string {
	v.mu.Lock()
	defer v.mu.Unlock()
	degraded := map[string]string{}
	for _, serverID := range serverIDs {
		if _, ok := v.statuses[serverID]; !ok {
			continue
		}
		v.statuses[serverID] = "degraded"
		degraded[serverID] = "fallback_queue"
	}
	return degraded
}

func (v *FleetVerifier) Recover(serverIDs []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, serverID := range serverIDs {
		if _, ok := v.statuses[serverID]; !ok {
			continue
		}
		v.statuses[serverID] = "healthy"
	}
}

func (v *FleetVerifier) StatusSnapshot() map[string]string {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]string, len(v.statuses))
	for serverID, status := range v.statuses {
		out[serverID] = status
	}
	return out
}

func (v *FleetVerifier) CallCounts() map[string]int {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]int, len(v.callCounts))
	for serverID, count := range v.callCounts {
		out[serverID] = count
	}
	return out
}
