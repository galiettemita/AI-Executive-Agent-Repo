package mcp

import (
	"fmt"
	"sort"
	"strings"
)

const executorRuntimeValuesPath = "artifacts/deploy/executor-mcp-runtime-values.yaml"

type ExecutorRolloutConfig struct {
	ImageRepository string
	ImageTag        string
	Namespace       string
	ReleaseName     string
	ChartPath       string
}

type ShellCommand struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

func (c ShellCommand) String() string {
	if len(c.Args) == 0 {
		return c.Program
	}
	return c.Program + " " + strings.Join(c.Args, " ")
}

type ExecutorRolloutPlan struct {
	ServerIDs       []string     `json:"server_ids"`
	ServerCount     int          `json:"server_count"`
	AllowlistCSV    string       `json:"allowlist_csv"`
	ImageReference  string       `json:"image_reference"`
	ValuesFilePath  string       `json:"values_file_path"`
	ValuesYAML      string       `json:"values_yaml"`
	DockerBuild     ShellCommand `json:"docker_build"`
	DockerPush      ShellCommand `json:"docker_push"`
	HelmUpgrade     ShellCommand `json:"helm_upgrade"`
	DockerBuildLine string       `json:"docker_build_line"`
	DockerPushLine  string       `json:"docker_push_line"`
	HelmUpgradeLine string       `json:"helm_upgrade_line"`
}

func BuildExecutorRolloutPlan(serverIDs []string, cfg ExecutorRolloutConfig) (ExecutorRolloutPlan, error) {
	normalizedIDs, err := normalizeServerIDs(serverIDs)
	if err != nil {
		return ExecutorRolloutPlan{}, err
	}
	if strings.TrimSpace(cfg.ImageRepository) == "" {
		return ExecutorRolloutPlan{}, fmt.Errorf("image_repository is required")
	}
	if strings.TrimSpace(cfg.ImageTag) == "" {
		return ExecutorRolloutPlan{}, fmt.Errorf("image_tag is required")
	}
	if strings.TrimSpace(cfg.Namespace) == "" {
		return ExecutorRolloutPlan{}, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(cfg.ReleaseName) == "" {
		return ExecutorRolloutPlan{}, fmt.Errorf("release_name is required")
	}
	if strings.TrimSpace(cfg.ChartPath) == "" {
		return ExecutorRolloutPlan{}, fmt.Errorf("chart_path is required")
	}

	imageReference := strings.TrimSpace(cfg.ImageRepository) + ":" + strings.TrimSpace(cfg.ImageTag)
	allowlistCSV := strings.Join(normalizedIDs, ",")
	valuesYAML := renderExecutorRuntimeValuesYAML(strings.TrimSpace(cfg.ImageRepository), strings.TrimSpace(cfg.ImageTag), allowlistCSV, len(normalizedIDs))

	dockerBuild := ShellCommand{
		Program: "docker",
		Args: []string{
			"build",
			"--build-arg", "SERVICE=executor",
			"-t", imageReference,
			".",
		},
	}
	dockerPush := ShellCommand{
		Program: "docker",
		Args: []string{
			"push", imageReference,
		},
	}
	helmUpgrade := ShellCommand{
		Program: "helm",
		Args: []string{
			"upgrade", "--install", strings.TrimSpace(cfg.ReleaseName), strings.TrimSpace(cfg.ChartPath),
			"-n", strings.TrimSpace(cfg.Namespace),
			"--create-namespace",
			"-f", executorRuntimeValuesPath,
		},
	}

	return ExecutorRolloutPlan{
		ServerIDs:       normalizedIDs,
		ServerCount:     len(normalizedIDs),
		AllowlistCSV:    allowlistCSV,
		ImageReference:  imageReference,
		ValuesFilePath:  executorRuntimeValuesPath,
		ValuesYAML:      valuesYAML,
		DockerBuild:     dockerBuild,
		DockerPush:      dockerPush,
		HelmUpgrade:     helmUpgrade,
		DockerBuildLine: dockerBuild.String(),
		DockerPushLine:  dockerPush.String(),
		HelmUpgradeLine: helmUpgrade.String(),
	}, nil
}

func normalizeServerIDs(serverIDs []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(serverIDs))
	for _, raw := range serverIDs {
		serverID := strings.TrimSpace(raw)
		if serverID == "" {
			continue
		}
		if !serverIDPattern.MatchString(serverID) {
			return nil, fmt.Errorf("invalid server_id: %s", serverID)
		}
		if _, ok := seen[serverID]; ok {
			continue
		}
		seen[serverID] = struct{}{}
		out = append(out, serverID)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no server_ids provided")
	}
	return out, nil
}

func renderExecutorRuntimeValuesYAML(imageRepository, imageTag, allowlistCSV string, serverCount int) string {
	lines := []string{
		"image:",
		fmt.Sprintf("  repository: %s", imageRepository),
		fmt.Sprintf("  tag: %s", imageTag),
		"extraEnv:",
		"  - name: MCP_RUNTIME_MODE",
		"    value: \"enabled\"",
		"  - name: MCP_SERVER_ALLOWLIST",
		fmt.Sprintf("    value: %q", allowlistCSV),
		"  - name: MCP_SERVER_COUNT",
		fmt.Sprintf("    value: %q", fmt.Sprintf("%d", serverCount)),
	}
	return strings.Join(lines, "\n") + "\n"
}
