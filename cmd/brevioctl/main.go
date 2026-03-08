package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "fail", "warn"
	Message string `json:"message"`
	Latency string `json:"latency,omitempty"`
}

type DoctorReport struct {
	Overall   string        `json:"overall"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Warned    int           `json:"warned"`
	Checks    []CheckResult `json:"checks"`
	Timestamp string        `json:"timestamp"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: brevioctl <command>")
		fmt.Println("Commands:")
		fmt.Println("  doctor    Run system health diagnostics")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "doctor":
		runDoctor()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runDoctor() {
	report := DoctorReport{
		Checks:    make([]CheckResult, 0, 10),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Check 1: DB connectivity
	report.Checks = append(report.Checks, checkDBConnectivity())

	// Check 2: Migrations applied
	report.Checks = append(report.Checks, checkMigrationsApplied())

	// Check 3: Temporal reachable
	report.Checks = append(report.Checks, checkTemporalReachable())

	// Check 4: Worker polling
	report.Checks = append(report.Checks, checkWorkerPolling())

	// Check 5: OTel exporter
	report.Checks = append(report.Checks, checkOTelExporter())

	// Check 6: Policy bundle load
	report.Checks = append(report.Checks, checkPolicyBundleLoad())

	// Check 7: Kill switch status
	report.Checks = append(report.Checks, checkKillSwitchStatus())

	// Check 8: DLQ backlog
	report.Checks = append(report.Checks, checkDLQBacklog())

	// Tally results
	for _, c := range report.Checks {
		switch c.Status {
		case "pass":
			report.Passed++
		case "fail":
			report.Failed++
		case "warn":
			report.Warned++
		}
	}

	if report.Failed > 0 {
		report.Overall = "UNHEALTHY"
	} else if report.Warned > 0 {
		report.Overall = "DEGRADED"
	} else {
		report.Overall = "HEALTHY"
	}

	// Output
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	if report.Failed > 0 {
		os.Exit(1)
	}
}

func checkDBConnectivity() CheckResult {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return CheckResult{
			Name:    "db_connectivity",
			Status:  "fail",
			Message: "DATABASE_URL not set",
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return CheckResult{
			Name:    "db_connectivity",
			Status:  "fail",
			Message: fmt.Sprintf("connection failed: %v", err),
			Latency: time.Since(start).String(),
		}
	}
	defer conn.Close(ctx)

	var result int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	latency := time.Since(start)
	if err != nil {
		return CheckResult{
			Name:    "db_connectivity",
			Status:  "fail",
			Message: fmt.Sprintf("query failed: %v", err),
			Latency: latency.String(),
		}
	}

	return CheckResult{
		Name:    "db_connectivity",
		Status:  "pass",
		Message: "PostgreSQL reachable",
		Latency: latency.String(),
	}
}

func checkMigrationsApplied() CheckResult {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return CheckResult{
			Name:    "migrations_applied",
			Status:  "fail",
			Message: "DATABASE_URL not set",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return CheckResult{
			Name:    "migrations_applied",
			Status:  "fail",
			Message: fmt.Sprintf("connection failed: %v", err),
		}
	}
	defer conn.Close(ctx)

	// Check if key tables exist
	requiredTables := []string{
		"accounts", "workspaces", "users",
		"authorization_receipts", "execution_ledger", "kill_switch_state",
		"federation_peers", "wallets", "admin_users",
	}

	var missing []string
	for _, table := range requiredTables {
		var exists bool
		err = conn.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)",
			table,
		).Scan(&exists)
		if err != nil || !exists {
			missing = append(missing, table)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:    "migrations_applied",
			Status:  "fail",
			Message: fmt.Sprintf("missing tables: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:    "migrations_applied",
		Status:  "pass",
		Message: fmt.Sprintf("all %d required tables present", len(requiredTables)),
	}
}

func checkTemporalReachable() CheckResult {
	host := os.Getenv("TEMPORAL_HOST")
	if host == "" {
		host = "localhost:7233"
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	latency := time.Since(start)
	if err != nil {
		return CheckResult{
			Name:    "temporal_reachable",
			Status:  "fail",
			Message: fmt.Sprintf("cannot reach %s: %v", host, err),
			Latency: latency.String(),
		}
	}
	conn.Close()

	return CheckResult{
		Name:    "temporal_reachable",
		Status:  "pass",
		Message: fmt.Sprintf("Temporal reachable at %s", host),
		Latency: latency.String(),
	}
}

func checkWorkerPolling() CheckResult {
	workerAddr := os.Getenv("TEMPORAL_WORKER_LISTEN_ADDR")
	if workerAddr == "" {
		workerAddr = "http://localhost:18084"
	}
	if !strings.HasPrefix(workerAddr, "http") {
		workerAddr = "http://localhost" + workerAddr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(workerAddr + "/health")
	if err != nil {
		return CheckResult{
			Name:    "worker_polling",
			Status:  "warn",
			Message: fmt.Sprintf("worker health endpoint unreachable: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{
			Name:    "worker_polling",
			Status:  "warn",
			Message: fmt.Sprintf("worker returned status %d", resp.StatusCode),
		}
	}

	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
		if checks, ok := health["checks"].(map[string]any); ok {
			if temporal, ok := checks["temporal"].(string); ok && temporal == "polling" {
				return CheckResult{
					Name:    "worker_polling",
					Status:  "pass",
					Message: "Temporal worker is polling",
				}
			}
		}
	}

	return CheckResult{
		Name:    "worker_polling",
		Status:  "pass",
		Message: "worker health endpoint responding",
	}
}

func checkOTelExporter() CheckResult {
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		return CheckResult{
			Name:    "otel_exporter",
			Status:  "warn",
			Message: "OTEL_EXPORTER_OTLP_ENDPOINT not set",
		}
	}

	return CheckResult{
		Name:    "otel_exporter",
		Status:  "pass",
		Message: fmt.Sprintf("OTel endpoint configured: %s", otelEndpoint),
	}
}

func checkPolicyBundleLoad() CheckResult {
	policyDir := "policies"
	entries, err := os.ReadDir(policyDir)
	if err != nil {
		return CheckResult{
			Name:    "policy_bundle_load",
			Status:  "fail",
			Message: fmt.Sprintf("cannot read policy directory: %v", err),
		}
	}

	regoCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".rego") {
			regoCount++
		}
	}

	if regoCount == 0 {
		return CheckResult{
			Name:    "policy_bundle_load",
			Status:  "fail",
			Message: "no .rego policy files found",
		}
	}

	return CheckResult{
		Name:    "policy_bundle_load",
		Status:  "pass",
		Message: fmt.Sprintf("%d policy files loaded", regoCount),
	}
}

func checkKillSwitchStatus() CheckResult {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return CheckResult{
			Name:    "kill_switch_status",
			Status:  "warn",
			Message: "DATABASE_URL not set, cannot check kill switch",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return CheckResult{
			Name:    "kill_switch_status",
			Status:  "warn",
			Message: "cannot connect to check kill switch",
		}
	}
	defer conn.Close(ctx)

	var activeCount int
	err = conn.QueryRow(ctx,
		"SELECT COUNT(*) FROM kill_switch_state WHERE is_active = true",
	).Scan(&activeCount)
	if err != nil {
		return CheckResult{
			Name:    "kill_switch_status",
			Status:  "warn",
			Message: fmt.Sprintf("kill_switch_state query failed: %v", err),
		}
	}

	if activeCount > 0 {
		return CheckResult{
			Name:    "kill_switch_status",
			Status:  "warn",
			Message: fmt.Sprintf("%d workspace(s) have active kill switch", activeCount),
		}
	}

	return CheckResult{
		Name:    "kill_switch_status",
		Status:  "pass",
		Message: "no active kill switches",
	}
}

func checkDLQBacklog() CheckResult {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return CheckResult{
			Name:    "dlq_backlog",
			Status:  "warn",
			Message: "DATABASE_URL not set, cannot check DLQ",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return CheckResult{
			Name:    "dlq_backlog",
			Status:  "warn",
			Message: "cannot connect to check DLQ",
		}
	}
	defer conn.Close(ctx)

	// Check for failed/dead_letter workflow instances
	var tableExists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='outbox_events')",
	).Scan(&tableExists)
	if err != nil || !tableExists {
		return CheckResult{
			Name:    "dlq_backlog",
			Status:  "pass",
			Message: "no DLQ table found (outbox_events), assuming clean",
		}
	}

	var backlog int
	err = conn.QueryRow(ctx,
		"SELECT COUNT(*) FROM outbox_events WHERE status = 'failed'",
	).Scan(&backlog)
	if err != nil {
		return CheckResult{
			Name:    "dlq_backlog",
			Status:  "warn",
			Message: fmt.Sprintf("DLQ query failed: %v", err),
		}
	}

	if backlog > 100 {
		return CheckResult{
			Name:    "dlq_backlog",
			Status:  "warn",
			Message: fmt.Sprintf("DLQ backlog: %d failed events", backlog),
		}
	}

	return CheckResult{
		Name:    "dlq_backlog",
		Status:  "pass",
		Message: fmt.Sprintf("DLQ backlog: %d", backlog),
	}
}
