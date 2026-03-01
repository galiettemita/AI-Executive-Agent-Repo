package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/mcp"
)

type runtimeRolloutReport struct {
	GeneratedAtUTC string                  `json:"generated_at_utc"`
	ReportVersion  string                  `json:"report_version"`
	ExecuteMode    bool                    `json:"execute_mode"`
	Plan           mcp.ExecutorRolloutPlan `json:"plan"`
}

func main() {
	execute := flag.Bool("execute", false, "execute docker build/push and helm upgrade")
	flag.Parse()

	root := repoRootOrExit()
	serverIDs := loadFleetServersOrExit(filepath.Join(root, "spec", "mcp", "fleet_servers_v1.txt"))

	config := mcp.ExecutorRolloutConfig{
		ImageRepository: envOrDefault("EXECUTOR_IMAGE_REPOSITORY", "ghcr.io/brevio/executor"),
		ImageTag:        envOrDefault("EXECUTOR_IMAGE_TAG", "v9.2.0"),
		Namespace:       envOrDefault("KUBE_NAMESPACE", "default"),
		ReleaseName:     envOrDefault("HELM_RELEASE", "brevio-executor"),
		ChartPath:       envOrDefault("HELM_CHART_PATH", "./helm/BREVIO-executor"),
	}

	plan, err := mcp.BuildExecutorRolloutPlan(serverIDs, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build runtime rollout plan: %v\n", err)
		os.Exit(1)
	}

	valuesPath := filepath.Join(root, plan.ValuesFilePath)
	writeFileOrExit(valuesPath, plan.ValuesYAML)

	report := runtimeRolloutReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		ReportVersion:  "mcp_runtime_rollout_v1",
		ExecuteMode:    *execute,
		Plan:           plan,
	}
	reportPath := filepath.Join(root, "artifacts", "deploy", "mcp_runtime_rollout_plan.json")
	writeJSONOrExit(reportPath, report)

	fmt.Printf("mcp runtime rollout plan generated: servers=%d report=%s values=%s\n", plan.ServerCount, reportPath, valuesPath)
	fmt.Printf("build:  %s\n", plan.DockerBuildLine)
	fmt.Printf("push:   %s\n", plan.DockerPushLine)
	fmt.Printf("deploy: %s\n", plan.HelmUpgradeLine)

	if !*execute {
		return
	}

	runCommandOrExit(root, plan.DockerBuild)
	runCommandOrExit(root, plan.DockerPush)
	runCommandOrExit(root, plan.HelmUpgrade)
	fmt.Println("mcp runtime rollout execution completed")
}

func repoRootOrExit() string {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repo root: %v\n", err)
		os.Exit(1)
	}
	return root
}

func loadFleetServersOrExit(path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read fleet servers: %v\n", err)
		os.Exit(1)
	}
	lines := strings.Split(string(body), "\n")
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

func runCommandOrExit(root string, command mcp.ShellCommand) {
	cmd := exec.Command(command.Program, command.Args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "command failed (%s): %v\n", command.String(), err)
		os.Exit(1)
	}
}

func writeJSONOrExit(path string, value any) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal json: %v\n", err)
		os.Exit(1)
	}
	writeFileOrExit(path, string(body)+"\n")
}

func writeFileOrExit(path, contents string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write file: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
