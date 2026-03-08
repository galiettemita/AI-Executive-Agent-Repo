package fastpath

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FastPathRoute maps a regex pattern to a precomputed response.
type FastPathRoute struct {
	ID                  string  `json:"id"`
	Pattern             string  `json:"pattern"`
	Response            string  `json:"response"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	HitCount            int     `json:"hit_count"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	Enabled             bool    `json:"enabled"`
	compiledPattern     *regexp.Regexp
}

// PrecomputedAnswer is a cached answer for a route.
type PrecomputedAnswer struct {
	ID         string    `json:"id"`
	PatternID  string    `json:"pattern_id"`
	Answer     string    `json:"answer"`
	ComputedAt time.Time `json:"computed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Score      float64   `json:"score"`
}

// FastPathResult is the output of a successful fast-path match.
type FastPathResult struct {
	RouteID   string  `json:"route_id"`
	Response  string  `json:"response"`
	LatencyMs float64 `json:"latency_ms"`
	FromCache bool    `json:"from_cache"`
}

// FastPathStats contains aggregate statistics for the fast-path system.
type FastPathStats struct {
	RouteCount   int     `json:"route_count"`
	TotalHits    int     `json:"total_hits"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	CacheSize    int     `json:"cache_size"`
}

// FastPathService provides System 1 fast-path reasoning with cached pattern matching.
type FastPathService struct {
	mu         sync.Mutex
	routes     map[string]FastPathRoute
	precomp    map[string]PrecomputedAnswer
	now        func() time.Time
	totalHits  int
	totalLatMs float64
}

// NewFastPathService creates a new FastPathService.
func NewFastPathService() *FastPathService {
	return &FastPathService{
		routes:  map[string]FastPathRoute{},
		precomp: map[string]PrecomputedAnswer{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// RegisterRoute adds a new fast-path route with a regex pattern.
func (s *FastPathService) RegisterRoute(pattern, response string, confidence float64) (*FastPathRoute, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if response == "" {
		return nil, fmt.Errorf("response is required")
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	route := FastPathRoute{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		Pattern:             pattern,
		Response:            response,
		ConfidenceThreshold: confidence,
		HitCount:            0,
		AvgLatencyMs:        0,
		Enabled:             true,
		compiledPattern:     compiled,
	}
	s.routes[route.ID] = route
	return &route, nil
}

// Match attempts to match input against enabled routes and returns a precomputed answer.
func (s *FastPathService) Match(input string) (*FastPathResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.now()

	for id, route := range s.routes {
		if !route.Enabled {
			continue
		}
		if route.compiledPattern == nil {
			continue
		}
		if !route.compiledPattern.MatchString(input) {
			continue
		}

		elapsed := float64(s.now().Sub(start).Microseconds()) / 1000.0

		// Check for a precomputed answer.
		response := route.Response
		fromCache := false
		if pa, ok := s.precomp[id]; ok && pa.ExpiresAt.After(s.now()) {
			response = pa.Answer
			fromCache = true
		}

		// Update stats.
		route.HitCount++
		totalLatency := route.AvgLatencyMs*float64(route.HitCount-1) + elapsed
		route.AvgLatencyMs = totalLatency / float64(route.HitCount)
		s.routes[id] = route

		s.totalHits++
		s.totalLatMs += elapsed

		return &FastPathResult{
			RouteID:   id,
			Response:  response,
			LatencyMs: elapsed,
			FromCache: fromCache,
		}, true
	}
	return nil, false
}

// RefreshPrecomputed sets or updates a precomputed answer for a route.
func (s *FastPathService) RefreshPrecomputed(routeID string, answer string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.routes[routeID]; !ok {
		return fmt.Errorf("route not found: %s", routeID)
	}
	if answer == "" {
		return fmt.Errorf("answer is required")
	}

	now := s.now()
	s.precomp[routeID] = PrecomputedAnswer{
		ID:         uuid.Must(uuid.NewV7()).String(),
		PatternID:  routeID,
		Answer:     answer,
		ComputedAt: now,
		ExpiresAt:  now.Add(ttl),
		Score:      1.0,
	}
	return nil
}

// DisableRoute disables a fast-path route.
func (s *FastPathService) DisableRoute(routeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	route, ok := s.routes[routeID]
	if !ok {
		return fmt.Errorf("route not found: %s", routeID)
	}
	route.Enabled = false
	s.routes[routeID] = route
	return nil
}

// ListRoutes returns all registered routes.
func (s *FastPathService) ListRoutes() []FastPathRoute {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]FastPathRoute, 0, len(s.routes))
	for _, r := range s.routes {
		cp := r
		cp.compiledPattern = nil // don't expose internal regex
		out = append(out, cp)
	}
	return out
}

// Stats returns aggregate statistics.
func (s *FastPathService) Stats() FastPathStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	avgLat := 0.0
	if s.totalHits > 0 {
		avgLat = s.totalLatMs / float64(s.totalHits)
	}

	return FastPathStats{
		RouteCount:   len(s.routes),
		TotalHits:    s.totalHits,
		AvgLatencyMs: avgLat,
		CacheSize:    len(s.precomp),
	}
}
