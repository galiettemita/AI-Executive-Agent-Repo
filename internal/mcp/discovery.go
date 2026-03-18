package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MCPDiscoveryService fetches and catalogs tools from registered MCP servers.
type MCPDiscoveryService struct {
	httpClient  *http.Client
	serverRepo  MCPServerDiscoveryRepository
	catalogRepo MCPToolCatalogRepository
	eventBus    EventPublisher
}

// NewMCPDiscoveryService creates a discovery service.
func NewMCPDiscoveryService(
	serverRepo MCPServerDiscoveryRepository,
	catalogRepo MCPToolCatalogRepository,
	eventBus EventPublisher,
) *MCPDiscoveryService {
	if eventBus == nil {
		eventBus = &NoopEventPublisher{}
	}
	return &MCPDiscoveryService{
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		serverRepo:  serverRepo,
		catalogRepo: catalogRepo,
		eventBus:    eventBus,
	}
}

// DiscoverAll fetches tools from all registered MCP servers.
// Called on workspace provisioning and every 30 minutes via Temporal cron.
// Errors per-server are non-fatal.
func (s *MCPDiscoveryService) DiscoverAll(ctx context.Context) error {
	servers, err := s.serverRepo.ListActiveServers(ctx)
	if err != nil {
		return fmt.Errorf("list mcp servers: %w", err)
	}
	var lastErr error
	for _, server := range servers {
		if err := s.discoverServer(ctx, server); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *MCPDiscoveryService) discoverServer(ctx context.Context, server MCPDiscoveredServer) error {
	url := strings.TrimRight(server.BaseURL, "/") + "/tools/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", server.ID, err)
	}
	if server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+server.AuthToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch tools from %s: %w", server.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response from %s: %w", server.ID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server %s returned %d", server.ID, resp.StatusCode)
	}

	var toolList struct {
		Tools []MCPToolSchema `json:"tools"`
	}
	if err := json.Unmarshal(body, &toolList); err != nil {
		return fmt.Errorf("parse tool list from %s: %w", server.ID, err)
	}

	activeKeys := make([]string, 0, len(toolList.Tools))
	for _, tool := range toolList.Tools {
		if err := s.validateSchema(tool); err != nil {
			continue // skip dangerous tools
		}

		schemaJSON, _ := json.Marshal(tool)
		hash := fmt.Sprintf("%x", sha256.Sum256(schemaJSON))
		toolKey := fmt.Sprintf("%s.%s", server.ID, tool.Name)

		existing, _ := s.catalogRepo.GetByKey(ctx, toolKey)

		discovered := DiscoveredTool{
			ToolKey:        toolKey,
			ServerID:       server.ID,
			Schema:         tool,
			SchemaHash:     hash,
			SchemaJSON:     schemaJSON,
			DiscoveredAt:   time.Now(),
			LastVerifiedAt: time.Now(),
			IsActive:       true,
		}

		if err := s.catalogRepo.Upsert(ctx, discovered); err != nil {
			return fmt.Errorf("upsert tool %s: %w", toolKey, err)
		}

		if existing == nil {
			_ = s.eventBus.Publish(ctx, MCPToolChangedEvent{
				ToolKey: toolKey, ServerID: server.ID, ChangeType: "added", Timestamp: time.Now(),
			})
		} else if existing.SchemaHash != hash {
			_ = s.eventBus.Publish(ctx, MCPToolChangedEvent{
				ToolKey: toolKey, ServerID: server.ID, ChangeType: "updated", Timestamp: time.Now(),
			})
		}

		activeKeys = append(activeKeys, toolKey)
	}

	return s.catalogRepo.MarkInactiveExcept(ctx, server.ID, activeKeys)
}

func (s *MCPDiscoveryService) validateSchema(tool MCPToolSchema) error {
	schemaBytes, _ := json.Marshal(tool)
	schemaStr := strings.ToLower(string(schemaBytes))
	for _, pattern := range DangerousParamPatterns {
		if strings.Contains(schemaStr, `"name":"`+pattern) ||
			strings.Contains(schemaStr, `"`+pattern+`"`) {
			return fmt.Errorf("dangerous param pattern '%s' in tool %s", pattern, tool.Name)
		}
	}
	return nil
}
