package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV9InfrastructureArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredTerraformModules := []string{
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
	}
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
