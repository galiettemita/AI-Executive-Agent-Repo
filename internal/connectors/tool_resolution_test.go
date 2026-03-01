package connectors

import (
	"reflect"
	"testing"
)

func TestToolResolutionCatalogs(t *testing.T) {
	t.Parallel()

	inventory := []ToolInventoryItem{
		{ToolKey: "b.tool"},
		{ToolKey: "a.tool"},
	}
	catalog := PlannerToolCatalog(inventory)
	if catalog[0].ToolKey != "a.tool" {
		t.Fatalf("expected lexical planner sort, got %+v", catalog)
	}

	definitions := []ConnectorToolDefinition{
		{ConnectorKey: "google_calendar", ToolKey: "calendar.list"},
		{ConnectorKey: "google_calendar", ToolKey: "calendar.create"},
		{ConnectorKey: "slack", ToolKey: "message.send"},
	}
	execCatalog := ConnectorExecutionCatalog(definitions, "google_calendar")
	if len(execCatalog) != 2 || execCatalog[0].ToolKey != "calendar.create" {
		t.Fatalf("unexpected execution catalog: %+v", execCatalog)
	}

	definition, ok := ResolveConnectorTool(definitions, "slack", "message.send")
	if !ok || definition.ToolKey != "message.send" {
		t.Fatalf("expected connector tool resolution: ok=%v def=%+v", ok, definition)
	}
}

func TestValidateInventoryBindings(t *testing.T) {
	t.Parallel()

	missing := ValidateInventoryBindings(
		[]ToolInventoryItem{{ToolKey: "calendar.list"}},
		[]ConnectorToolDefinition{
			{ConnectorKey: "google_calendar", ToolKey: "calendar.list"},
			{ConnectorKey: "google_calendar", ToolKey: "calendar.create"},
		},
	)
	expected := []string{"calendar.create"}
	if !reflect.DeepEqual(missing, expected) {
		t.Fatalf("unexpected missing bindings: got=%v want=%v", missing, expected)
	}
}
