package temporal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// AgentHeartbeatActivity pings all registered agents and marks stale ones inactive.
func (a *Activities) AgentHeartbeatActivity(ctx context.Context) error {
	if a.agentMarketplace == nil {
		return nil
	}

	agents, err := a.agentMarketplace.GetAllAgents(ctx)
	if err != nil {
		return fmt.Errorf("heartbeat: list agents: %w", err)
	}

	for _, agent := range agents {
		if agent.Status != "active" {
			continue
		}
		if err := pingAgent(ctx, agent.BaseURL); err != nil {
			log.Printf("[a2a heartbeat] agent %s unreachable: %v", agent.AgentID, err)
		} else {
			_ = a.agentMarketplace.RecordHeartbeat(ctx, agent.AgentID, time.Now())
		}
	}

	return a.agentMarketplace.MarkInactiveStaleAgents(ctx, 15*time.Minute)
}

func pingAgent(ctx context.Context, baseURL string) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, baseURL+"/.well-known/agent.json", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat: unexpected status %d", resp.StatusCode)
	}
	return nil
}
