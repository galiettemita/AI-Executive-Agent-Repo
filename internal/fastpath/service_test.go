package fastpath

import (
	"testing"
	"time"
)

func TestRegisterRoute(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	route, err := s.RegisterRoute("(?i)hello", "Hi there!", 0.9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.ID == "" {
		t.Fatal("expected non-empty route ID")
	}
	if !route.Enabled {
		t.Fatal("expected route to be enabled")
	}
}

func TestRegisterRouteInvalidRegex(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	_, err := s.RegisterRoute("[invalid", "response", 0.9)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestRegisterRouteEmptyPattern(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	_, err := s.RegisterRoute("", "response", 0.9)
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestMatchHit(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	route, _ := s.RegisterRoute("(?i)what time", "Check your clock", 0.8)
	result, ok := s.Match("What time is it?")
	if !ok {
		t.Fatal("expected match")
	}
	if result.RouteID != route.ID {
		t.Fatalf("unexpected route ID: %s", result.RouteID)
	}
	if result.Response != "Check your clock" {
		t.Fatalf("unexpected response: %s", result.Response)
	}
}

func TestMatchMiss(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	_, _ = s.RegisterRoute("(?i)hello", "Hi!", 0.8)
	_, ok := s.Match("goodbye")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestMatchWithPrecomputed(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	route, _ := s.RegisterRoute("(?i)weather", "Default weather response", 0.8)
	_ = s.RefreshPrecomputed(route.ID, "Sunny, 72F", 5*time.Minute)

	result, ok := s.Match("What is the weather?")
	if !ok {
		t.Fatal("expected match")
	}
	if result.Response != "Sunny, 72F" {
		t.Fatalf("expected precomputed answer, got: %s", result.Response)
	}
	if !result.FromCache {
		t.Fatal("expected FromCache to be true")
	}
}

func TestDisableRoute(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	route, _ := s.RegisterRoute("(?i)hello", "Hi!", 0.8)
	_ = s.DisableRoute(route.ID)
	_, ok := s.Match("hello")
	if ok {
		t.Fatal("expected no match for disabled route")
	}
}

func TestListRoutes(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	_, _ = s.RegisterRoute("a", "A", 0.8)
	_, _ = s.RegisterRoute("b", "B", 0.8)

	routes := s.ListRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	s := NewFastPathService()

	_, _ = s.RegisterRoute("(?i)test", "OK", 0.8)
	_, _ = s.Match("test input")
	_, _ = s.Match("test again")

	stats := s.Stats()
	if stats.RouteCount != 1 {
		t.Fatalf("expected 1 route, got %d", stats.RouteCount)
	}
	if stats.TotalHits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.TotalHits)
	}
}
