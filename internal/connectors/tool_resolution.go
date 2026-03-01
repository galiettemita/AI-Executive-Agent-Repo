package connectors

import (
	"sort"
	"strings"
)

type ToolInventoryItem struct {
	ToolKey              string
	DisplayName          string
	Domain               string
	EstimatedCostPerCall float64
	Description          string
	IsWrite              bool
	RiskLevel            string
}

type ConnectorToolDefinition struct {
	ConnectorKey  string
	ToolKey       string
	InputSchema   string
	OutputSchema  string
	AutonomyFloor string
	IsWrite       bool
	IsReversible  bool
}

func PlannerToolCatalog(inventory []ToolInventoryItem) []ToolInventoryItem {
	out := append([]ToolInventoryItem(nil), inventory...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ToolKey < out[j].ToolKey
	})
	return out
}

func ConnectorExecutionCatalog(definitions []ConnectorToolDefinition, connectorKey string) []ConnectorToolDefinition {
	key := strings.TrimSpace(connectorKey)
	out := make([]ConnectorToolDefinition, 0)
	for _, definition := range definitions {
		if definition.ConnectorKey == key {
			out = append(out, definition)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ToolKey < out[j].ToolKey
	})
	return out
}

func ResolveConnectorTool(definitions []ConnectorToolDefinition, connectorKey, toolKey string) (ConnectorToolDefinition, bool) {
	for _, definition := range definitions {
		if definition.ConnectorKey == connectorKey && definition.ToolKey == toolKey {
			return definition, true
		}
	}
	return ConnectorToolDefinition{}, false
}

func ValidateInventoryBindings(inventory []ToolInventoryItem, definitions []ConnectorToolDefinition) []string {
	inventoryKeys := map[string]struct{}{}
	for _, item := range inventory {
		inventoryKeys[item.ToolKey] = struct{}{}
	}
	missing := map[string]struct{}{}
	for _, definition := range definitions {
		if _, ok := inventoryKeys[definition.ToolKey]; !ok {
			missing[definition.ToolKey] = struct{}{}
		}
	}
	out := make([]string, 0, len(missing))
	for toolKey := range missing {
		out = append(out, toolKey)
	}
	sort.Strings(out)
	return out
}
