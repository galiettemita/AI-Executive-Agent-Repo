package llm

import (
	"reflect"
	"testing"
)

func TestRouteToSpecialist(t *testing.T) {
	t.Parallel()

	specialists := []SpecialistAgent{
		{Name: "Travel", IsActive: true, TriggerKeywords: []string{"flight", "hotel"}},
		{Name: "Code Reviewer", IsActive: true, TriggerRegexes: []string{`(?i)review.*pr`}},
	}

	specialist, ok, reason := RouteToSpecialist("book a flight to nyc", "", "", specialists)
	if !ok || specialist.Name != "Travel" || reason != "pattern_match" {
		t.Fatalf("unexpected keyword routing result: ok=%v reason=%s specialist=%+v", ok, reason, specialist)
	}

	specialist, ok, reason = RouteToSpecialist("anything", "@code reviewer", "", specialists)
	if !ok || specialist.Name != "Code Reviewer" || reason != "explicit_invocation" {
		t.Fatalf("unexpected explicit routing result: ok=%v reason=%s specialist=%+v", ok, reason, specialist)
	}

	_, ok, reason = RouteToSpecialist("anything", "", "unknown", specialists)
	if ok || reason != "default_brain" {
		t.Fatalf("unexpected default route result: ok=%v reason=%s", ok, reason)
	}
}

func TestFilterToolsForSpecialist(t *testing.T) {
	t.Parallel()

	filtered := FilterToolsForSpecialist(
		[]string{"calendar.create", "web.search", "travel.book"},
		[]string{"travel.book", "web.search"},
	)
	expected := []string{"travel.book", "web.search"}
	if !reflect.DeepEqual(filtered, expected) {
		t.Fatalf("unexpected filtered tools: got=%v want=%v", filtered, expected)
	}
}
