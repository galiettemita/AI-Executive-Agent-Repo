package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestV9InfrastructureArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredTerraformModules := []string{
		"admin-frontend",
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
		"opensearch",
		"feature-flags-cache",
	}
	assertExactDirectorySet(t, filepath.Join(root, "terraform", "modules"), requiredTerraformModules)
	for _, module := range requiredTerraformModules {
		assertFileNonEmpty(t, filepath.Join(root, "terraform", "modules", module, "main.tf"))
	}
	requiredModuleTokens := map[string][]string{
		"vpc":           {"cidr_block", "nat_gateway_enabled", "security_groups"},
		"eks":           {"kubernetes_version", "irsa_enabled", "managed_node_groups"},
		"rds":           {"engine_version", "multi_az", "storage_encrypted", "pgbouncer_sidecar"},
		"elasticache":   {"transit_encryption_enabled", "automatic_failover_enabled"},
		"sqs":           {"interactive_turns.fifo", "dead_letter_queues", "redrive_policy"},
		"s3":            {"attachments", "sboms", "exports", "schemas"},
		"secrets":       {"managed_secrets", "dual_key_read_window_minutes_min"},
		"temporal":      {"namespace", "retention_days", "task_queue"},
		"observability": {"prometheus", "grafana", "loki", "jaeger", "otel_collector"},
	}
	for module, tokens := range requiredModuleTokens {
		assertFileContainsTokens(t, filepath.Join(root, "terraform", "modules", module, "main.tf"), tokens)
	}
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "sqs", "main.tf"), "fifo_queues", []string{
		"interactive_turns.fifo",
	})
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "sqs", "main.tf"), "standard_queues", []string{
		"workflow_tasks",
		"ledger_writes",
		"trajectory_writes",
		"rate_limit_ledger_writes",
	})
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "sqs", "main.tf"), "dead_letter_queues", []string{
		"interactive_turns_dlq",
		"workflow_tasks_dlq",
		"ledger_writes_dlq",
		"trajectory_writes_dlq",
		"rate_limit_ledger_writes_dlq",
	})
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "s3", "main.tf"), "buckets", []string{
		"attachments",
		"sboms",
		"exports",
		"schemas",
	})
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "observability", "main.tf"), "stack", []string{
		"prometheus",
		"grafana",
		"loki",
		"jaeger",
		"otel_collector",
	})
	assertModuleQuotedListEquals(t, filepath.Join(root, "terraform", "modules", "secrets", "main.tf"), "managed_secrets", []string{
		"app_secret",
		"encryption_keys",
		"oauth_client_secrets",
	})

	requiredModuleBlocks := []string{
		`module "vpc"`,
		`module "eks"`,
		`module "rds"`,
		`module "elasticache"`,
		`module "sqs"`,
		`module "s3"`,
		`module "secrets"`,
		`module "temporal"`,
		`module "observability"`,
		`module "opensearch"`,
		`module "admin_frontend"`,
		`module "feature_flags_cache"`,
	}
	assertFileContainsTokens(t, filepath.Join(root, "terraform", "environments", "staging", "main.tf"), requiredModuleBlocks)
	assertFileContainsTokens(t, filepath.Join(root, "terraform", "environments", "production", "main.tf"), requiredModuleBlocks)
	assertTerraformEnvironmentModuleSet(t, filepath.Join(root, "terraform", "environments", "staging", "main.tf"), []string{
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
		"opensearch",
		"admin_frontend",
		"feature_flags_cache",
	})
	assertTerraformEnvironmentModuleSet(t, filepath.Join(root, "terraform", "environments", "production", "main.tf"), []string{
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
		"opensearch",
		"admin_frontend",
		"feature_flags_cache",
	})

	coreChartsWithHPA := []string{
		"BREVIO-gateway",
		"BREVIO-brain",
		"BREVIO-control",
		"BREVIO-executor",
		"BREVIO-temporal-worker",
	}
	for _, chart := range coreChartsWithHPA {
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "Chart.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "values.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "deployment.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "service.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "hpa.yaml"))
	}
	coreChartValueTokens := map[string][]string{
		"BREVIO-gateway":         {"minReplicas: 3", "maxReplicas: 10"},
		"BREVIO-brain":           {"minReplicas: 2", "maxReplicas: 8"},
		"BREVIO-control":         {"minReplicas: 2", "maxReplicas: 6"},
		"BREVIO-executor":        {"minReplicas: 3", "maxReplicas: 12"},
		"BREVIO-canvas":          {"replicaCount: 2", "maxReplicas: 4"},
		"BREVIO-temporal-worker": {"minReplicas: 2", "maxReplicas: 8", "taskQueue: BREVIO-tasks"},
	}
	for chart, tokens := range coreChartValueTokens {
		assertFileContainsTokens(t, filepath.Join(root, "helm", chart, "values.yaml"), tokens)
	}

	canvasChart := filepath.Join(root, "helm", "BREVIO-canvas")
	assertFileNonEmpty(t, filepath.Join(canvasChart, "Chart.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "values.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "templates", "deployment.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "templates", "service.yaml"))

	assertFileNonEmpty(t, filepath.Join(root, "helm", "BREVIO-gateway", "templates", "pdb.yaml"))
}

func TestV92InfrastructureArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	allHelmCharts := []string{
		"BREVIO-gateway",
		"BREVIO-brain",
		"BREVIO-control",
		"BREVIO-executor",
		"BREVIO-canvas",
		"BREVIO-temporal-worker",
		"BREVIO-admin-api",
		"BREVIO-admin-frontend",
		"BREVIO-rag-worker",
		"BREVIO-guardrails",
		"BREVIO-health-checker",
	}
	assertExactDirectorySet(t, filepath.Join(root, "helm"), allHelmCharts)
	expectedHelmImages := map[string]string{
		"BREVIO-gateway":         "ghcr.io/brevio/gateway",
		"BREVIO-brain":           "ghcr.io/brevio/brain",
		"BREVIO-control":         "ghcr.io/brevio/control",
		"BREVIO-executor":        "ghcr.io/brevio/executor",
		"BREVIO-canvas":          "ghcr.io/brevio/canvas",
		"BREVIO-temporal-worker": "ghcr.io/brevio/temporal-worker",
		"BREVIO-admin-api":       "ghcr.io/brevio/admin-api",
		"BREVIO-admin-frontend":  "ghcr.io/brevio/admin-frontend",
		"BREVIO-rag-worker":      "ghcr.io/brevio/rag-worker",
		"BREVIO-guardrails":      "ghcr.io/brevio/guardrails",
		"BREVIO-health-checker":  "ghcr.io/brevio/health-checker",
	}
	for _, chart := range allHelmCharts {
		assertHelmImageBaseline(t, filepath.Join(root, "helm", chart, "values.yaml"), expectedHelmImages[chart], "v9.2.0")
	}

	requiredTerraformModules := []string{
		"opensearch",
		"admin-frontend",
		"feature-flags-cache",
	}
	for _, module := range requiredTerraformModules {
		path := filepath.Join(root, "terraform", "modules", module, "main.tf")
		assertFileNonEmpty(t, path)
	}
	requiredModuleTokens := map[string][]string{
		"opensearch":          {"data_nodes", "ultra_warm_enabled", "hybrid_rag"},
		"admin-frontend":      {"cloudfront", "waf_enabled", "admin_ip_allowlist_enabled"},
		"feature-flags-cache": {"dedicated_cluster", "sub_millisecond_target", "transit_encryption_enabled"},
	}
	for module, tokens := range requiredModuleTokens {
		assertFileContainsTokens(t, filepath.Join(root, "terraform", "modules", module, "main.tf"), tokens)
	}

	requiredHelmCharts := []string{
		"BREVIO-admin-api",
		"BREVIO-admin-frontend",
		"BREVIO-rag-worker",
		"BREVIO-guardrails",
		"BREVIO-health-checker",
	}
	for _, chart := range requiredHelmCharts {
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "Chart.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "values.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "deployment.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "service.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "hpa.yaml"))
	}
	requiredHelmValueTokens := map[string][]string{
		"BREVIO-admin-api":      {"minReplicas: 2", "maxReplicas: 4", "cpu: \"1\"", "memory: \"1Gi\""},
		"BREVIO-admin-frontend": {"replicaCount: 2", "cpu: \"500m\"", "memory: \"256Mi\""},
		"BREVIO-rag-worker":     {"minReplicas: 2", "maxReplicas: 6", "cpu: \"2\"", "memory: \"4Gi\""},
		"BREVIO-guardrails":     {"minReplicas: 2", "maxReplicas: 4", "cpu: \"1\"", "memory: \"1Gi\""},
		"BREVIO-health-checker": {"minReplicas: 1", "maxReplicas: 2", "cpu: \"500m\"", "memory: \"512Mi\""},
	}
	for chart, tokens := range requiredHelmValueTokens {
		assertFileContainsTokens(t, filepath.Join(root, "helm", chart, "values.yaml"), tokens)
	}

	assertFileNonEmpty(t, filepath.Join(root, "admin", "src", "pages", "Dashboard.tsx"))
	assertFileNonEmpty(t, filepath.Join(root, "admin", "src", "api", "client.ts"))
}

func TestV92RunbooksExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for i := 1; i <= 9; i++ {
		path := filepath.Join(root, "runbooks", formatV92RunbookName(i))
		assertFileNonEmpty(t, path)
	}
}

func formatV92RunbookName(index int) string {
	return fmt.Sprintf("RB-V92-%03d.md", index)
}

func assertFileNonEmpty(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("required artifact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("required artifact is empty: %s", path)
	}
}

func assertFileContainsTokens(t *testing.T, path string, required []string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	content := string(body)
	for _, token := range required {
		if !strings.Contains(content, token) {
			t.Fatalf("missing token %q in %s", token, path)
		}
	}
}

func assertExactDirectorySet(t *testing.T, path string, expected []string) {
	t.Helper()

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read directory %s: %v", path, err)
	}

	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			actual = append(actual, entry.Name())
		}
	}

	sort.Strings(actual)
	sort.Strings(expected)

	if len(actual) != len(expected) {
		t.Fatalf("directory count mismatch for %s: got=%d want=%d actual=%v expected=%v", path, len(actual), len(expected), actual, expected)
	}
	for i := range actual {
		if actual[i] != expected[i] {
			t.Fatalf("directory set mismatch for %s: actual=%v expected=%v", path, actual, expected)
		}
	}
}

func assertModuleQuotedListEquals(t *testing.T, path, key string, expected []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read module file %s: %v", path, err)
	}
	content := string(body)
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(key) + `\s*=\s*\[(.*?)\]`)
	match := pattern.FindStringSubmatch(content)
	if len(match) < 2 {
		t.Fatalf("list key %q not found in %s", key, path)
	}
	valueBlock := match[1]
	stringPattern := regexp.MustCompile(`"([^"]+)"`)
	extracted := make([]string, 0)
	for _, hit := range stringPattern.FindAllStringSubmatch(valueBlock, -1) {
		if len(hit) < 2 {
			continue
		}
		extracted = append(extracted, hit[1])
	}
	assertStringSliceSetEquals(t, extracted, expected, fmt.Sprintf("%s:%s", path, key))
}

func assertTerraformEnvironmentModuleSet(t *testing.T, path string, expected []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read environment file %s: %v", path, err)
	}
	content := string(body)
	modulePattern := regexp.MustCompile(`module\s+"([^"]+)"`)
	matches := modulePattern.FindAllStringSubmatch(content, -1)
	actual := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		actual = append(actual, match[1])
	}
	assertStringSliceSetEquals(t, actual, expected, path+":module_set")
}

func assertStringSliceSetEquals(t *testing.T, actual, expected []string, label string) {
	t.Helper()
	actualSet := map[string]struct{}{}
	for _, item := range actual {
		actualSet[item] = struct{}{}
	}
	expectedSet := map[string]struct{}{}
	for _, item := range expected {
		expectedSet[item] = struct{}{}
	}
	missing := make([]string, 0)
	for item := range expectedSet {
		if _, ok := actualSet[item]; !ok {
			missing = append(missing, item)
		}
	}
	extra := make([]string, 0)
	for item := range actualSet {
		if _, ok := expectedSet[item]; !ok {
			extra = append(extra, item)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	t.Fatalf("%s mismatch: missing=%v extra=%v", label, missing, extra)
}

func assertHelmImageBaseline(t *testing.T, path, repository, tag string) {
	t.Helper()
	content := readFileString(t, path)
	if repository == "" {
		t.Fatalf("expected repository must be provided for %s", path)
	}
	if tag == "" {
		t.Fatalf("expected tag must be provided for %s", path)
	}
	if strings.Contains(content, "repository: busybox") {
		t.Fatalf("helm chart still uses placeholder repository in %s", path)
	}
	if strings.Contains(content, "tag: latest") {
		t.Fatalf("helm chart still uses floating latest tag in %s", path)
	}
	if !strings.Contains(content, "repository: "+repository) {
		t.Fatalf("helm chart repository mismatch in %s; expected %s", path, repository)
	}
	if !strings.Contains(content, "tag: "+tag) {
		t.Fatalf("helm chart tag mismatch in %s; expected %s", path, tag)
	}
}
