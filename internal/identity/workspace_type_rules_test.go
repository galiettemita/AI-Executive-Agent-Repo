package identity

import (
	"reflect"
	"testing"
)

func TestWorkspaceTypeRules(t *testing.T) {
	t.Parallel()

	if !IsTwoManRuleActive("professional", true) {
		t.Fatal("expected two-man rule for professional workspace with admins")
	}
	if IsTwoManRuleActive("personal", true) {
		t.Fatal("did not expect two-man rule for personal workspace")
	}
	if got := MaxAutonomyForWorkspaceType("delegation", "A3"); got != "A2" {
		t.Fatalf("unexpected delegation cap: %s", got)
	}
	if got := MaxAutonomyForWorkspaceType("professional", "A1"); got != "A4" {
		t.Fatalf("unexpected professional cap: %s", got)
	}
}

func TestDelegationFilters(t *testing.T) {
	t.Parallel()

	memoryKeys := []string{"mem:one", "mem:two"}
	filteredMemory := FilterMemoryKeysForWorkspaceType("delegation", memoryKeys, []string{"mem:two"})
	if !reflect.DeepEqual(filteredMemory, []string{"mem:two"}) {
		t.Fatalf("unexpected delegation memory filter: %v", filteredMemory)
	}

	tools := []string{"calendar.create", "financial.transfer"}
	filteredTools := FilterToolsForWorkspaceType("delegation", tools, []string{"calendar.create"})
	if !reflect.DeepEqual(filteredTools, []string{"calendar.create"}) {
		t.Fatalf("unexpected delegation tool filter: %v", filteredTools)
	}

	if DelegateFinancialAccessGranted([]string{"tasks"}, []string{"calendar.create"}) {
		t.Fatal("did not expect financial access without financial grant")
	}
	if !DelegateFinancialAccessGranted([]string{"tasks"}, []string{"stripe.create_payment"}) {
		t.Fatal("expected financial access when financial tool is explicitly granted")
	}
}
