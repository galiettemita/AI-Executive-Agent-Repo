package temporal

import (
	"context"
	"log"
)

// MCPToolDiscoveryActivity discovers tools from all registered MCP servers.
func (a *Activities) MCPToolDiscoveryActivity(ctx context.Context) error {
	if a.mcpDiscovery == nil {
		log.Println("[MCP Discovery] service not configured, skipping")
		return nil
	}
	return a.mcpDiscovery.DiscoverAll(ctx)
}
