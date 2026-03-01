package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/mcp"
)

type fleetValidationReport struct {
	GeneratedAtUTC                string   `json:"generated_at_utc"`
	ServerCount                   int      `json:"server_count"`
	AllServersHealthy             bool     `json:"all_servers_healthy"`
	Concurrent100CallsPassed      bool     `json:"concurrent_100_calls_passed"`
	FailoverKillFivePassed        bool     `json:"failover_kill_five_passed"`
	KilledServers                 []string `json:"killed_servers"`
	RecoveredAllServers           bool     `json:"recovered_all_servers"`
	ReportVersion                 string   `json:"report_version"`
	MixedCallTotal                int      `json:"mixed_call_total"`
	DegradedServersDuringFailover int      `json:"degraded_servers_during_failover"`
	RejectedCallsDuringFailover   int      `json:"rejected_calls_during_failover"`
}

func main() {
	root := repoRootFleetOrExit()
	serverIDs := loadFleetServersOrExit(filepath.Join(root, "spec", "mcp", "fleet_servers_v1.txt"))

	verifier := mcp.NewFleetVerifier(serverIDs)
	healthy, _ := verifier.VerifyAllHealthy()

	concurrentCallsPassed := runMixedCalls(verifier, serverIDs, 100)
	callTotal := 0
	for _, count := range verifier.CallCounts() {
		callTotal += count
	}
	if callTotal != 100 {
		concurrentCallsPassed = false
	}

	killed := pickDeterministicServers(serverIDs, 5)
	degraded := verifier.SimulateFailures(killed)
	rejectedCalls := 0
	for _, serverID := range killed {
		if err := verifier.RecordCall(serverID); err != nil {
			rejectedCalls++
		}
	}
	failoverPassed := len(degraded) == 5 && rejectedCalls == 5

	verifier.Recover(killed)
	recoveredHealthy, _ := verifier.VerifyAllHealthy()

	report := fleetValidationReport{
		GeneratedAtUTC:                time.Now().UTC().Format(time.RFC3339),
		ServerCount:                   len(serverIDs),
		AllServersHealthy:             healthy && len(serverIDs) == 40,
		Concurrent100CallsPassed:      concurrentCallsPassed,
		FailoverKillFivePassed:        failoverPassed,
		KilledServers:                 killed,
		RecoveredAllServers:           recoveredHealthy,
		ReportVersion:                 "mcp_fleet_validation_v1",
		MixedCallTotal:                callTotal,
		DegradedServersDuringFailover: len(degraded),
		RejectedCallsDuringFailover:   rejectedCalls,
	}

	reportPath := filepath.Join(root, "artifacts", "deploy", "mcp_fleet_validation_report.json")
	writeFleetReportOrExit(reportPath, report)

	if !report.AllServersHealthy || !report.Concurrent100CallsPassed || !report.FailoverKillFivePassed || !report.RecoveredAllServers {
		fmt.Printf("mcp fleet validation failed: %#v report=%s\n", report, reportPath)
		os.Exit(1)
	}
	fmt.Printf("mcp fleet validation passed: servers=%d report=%s\n", report.ServerCount, reportPath)
}

func repoRootFleetOrExit() string {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repo root: %v\n", err)
		os.Exit(1)
	}
	return root
}

func loadFleetServersOrExit(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read fleet server list: %v\n", err)
		os.Exit(1)
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		serverID := strings.TrimSpace(line)
		if serverID == "" || strings.HasPrefix(serverID, "#") {
			continue
		}
		if _, ok := seen[serverID]; ok {
			continue
		}
		seen[serverID] = struct{}{}
		out = append(out, serverID)
	}
	return out
}

func runMixedCalls(verifier *mcp.FleetVerifier, serverIDs []string, total int) bool {
	if total <= 0 {
		return false
	}
	errCh := make(chan error, total)
	var wg sync.WaitGroup
	for idx := 0; idx < total; idx++ {
		serverID := serverIDs[idx%len(serverIDs)]
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
			return false
		}
	}
	return true
}

func pickDeterministicServers(serverIDs []string, count int) []string {
	if count <= 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	out := make([]string, 0, count)
	seen := map[string]struct{}{}
	for len(out) < count {
		serverID := serverIDs[rng.Intn(len(serverIDs))]
		if _, ok := seen[serverID]; ok {
			continue
		}
		seen[serverID] = struct{}{}
		out = append(out, serverID)
	}
	return out
}

func writeFleetReportOrExit(path string, report fleetValidationReport) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create report directory: %v\n", err)
		os.Exit(1)
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
}
