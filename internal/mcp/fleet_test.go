package mcp

import (
	"math/rand"
	"sync"
	"testing"
)

func fleetServerIDs() []string {
	return []string{
		"google_calendar", "google_drive", "google_gmail", "notion", "todoist", "brave_search", "github", "apple_reminders",
		"slack", "outlook_calendar", "outlook_mail", "microsoft_teams", "linear", "asana", "whatsapp_business",
		"stripe", "quickbooks", "hubspot", "salesforce", "google_sheets", "airtable", "jira", "sentry",
		"maps", "uber_lyft", "opentable_resy", "home_assistant", "spotify", "evernote", "dropbox",
		"duffel", "zoom", "calendly", "plaid", "crunchbase",
		"booking", "docusign", "canva", "instacart", "tesla",
	}
}

func TestFleetVerifierAll40ServersHealthy(t *testing.T) {
	t.Parallel()

	verifier := NewFleetVerifier(fleetServerIDs())
	healthy, unhealthy := verifier.VerifyAllHealthy()
	if !healthy {
		t.Fatalf("expected fleet healthy, unhealthy=%v", unhealthy)
	}
	if len(unhealthy) != 0 {
		t.Fatalf("expected no unhealthy servers, got %v", unhealthy)
	}
}

func TestFleetVerifierMixed100ConcurrentCalls(t *testing.T) {
	t.Parallel()

	servers := fleetServerIDs()
	verifier := NewFleetVerifier(servers)

	errCh := make(chan error, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		serverID := servers[i%len(servers)]
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			if err := verifier.RecordCall(target); err != nil {
				errCh <- err
			}
		}(serverID)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("expected no call failures, got %v", err)
		}
	}

	totalCalls := 0
	for _, count := range verifier.CallCounts() {
		totalCalls += count
	}
	if totalCalls != 100 {
		t.Fatalf("expected 100 recorded calls, got %d", totalCalls)
	}
}

func TestFleetVerifierFailoverKillFiveAndRecover(t *testing.T) {
	t.Parallel()

	servers := fleetServerIDs()
	verifier := NewFleetVerifier(servers)
	rng := rand.New(rand.NewSource(42))

	killed := make([]string, 0, 5)
	seen := map[string]struct{}{}
	for len(killed) < 5 {
		serverID := servers[rng.Intn(len(servers))]
		if _, ok := seen[serverID]; ok {
			continue
		}
		seen[serverID] = struct{}{}
		killed = append(killed, serverID)
	}

	fallbackMap := verifier.SimulateFailures(killed)
	if len(fallbackMap) != 5 {
		t.Fatalf("expected fallback map for 5 killed servers, got %d", len(fallbackMap))
	}
	for _, serverID := range killed {
		if fallbackMap[serverID] != "fallback_queue" {
			t.Fatalf("expected fallback route for %s, got %q", serverID, fallbackMap[serverID])
		}
		if err := verifier.RecordCall(serverID); err == nil {
			t.Fatalf("expected call rejection for degraded server %s", serverID)
		}
	}

	healthy, unhealthy := verifier.VerifyAllHealthy()
	if healthy || len(unhealthy) != 5 {
		t.Fatalf("expected exactly 5 degraded servers, healthy=%t unhealthy=%v", healthy, unhealthy)
	}

	verifier.Recover(killed)
	healthy, unhealthy = verifier.VerifyAllHealthy()
	if !healthy || len(unhealthy) != 0 {
		t.Fatalf("expected full fleet recovery, healthy=%t unhealthy=%v", healthy, unhealthy)
	}
}
