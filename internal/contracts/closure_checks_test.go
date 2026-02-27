package contracts

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClosureChecksV9Section172(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("identifiers_defined_once", func(t *testing.T) {
		assertNoDuplicatePromptIDs(t, filepath.Join(root, "prompts", "seed_prompts_v9.txt"))
		assertNoDuplicatePromptIDs(t, filepath.Join(root, "prompts", "seed_prompts_v91.txt"))
		assertNoDuplicatePromptIDs(t, filepath.Join(root, "prompts", "seed_prompts_v92.txt"))
		assertNoDuplicateCreateStatements(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"), `(?mi)^CREATE TABLE\s+([a-z0-9_]+)\s*\(`)
		assertNoDuplicateCreateStatements(t, filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql"), `(?mi)^CREATE TABLE\s+([a-z0-9_]+)\s*\(`)
		assertNoDuplicateCreateStatements(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"), `(?mi)^CREATE TABLE\s+([a-z0-9_]+)\s*\(`)
	})

	t.Run("endpoints_to_schema_pointers", func(t *testing.T) {
		path := filepath.Join(root, "api", "openapi", "v9.yaml")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read openapi file: %v", err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(body, &doc); err != nil {
			t.Fatalf("parse openapi yaml: %v", err)
		}
		paths, ok := doc["paths"].(map[string]any)
		if !ok || len(paths) == 0 {
			t.Fatal("openapi paths missing")
		}
		components, ok := doc["components"].(map[string]any)
		if !ok {
			t.Fatal("openapi components missing")
		}
		schemas, ok := components["schemas"].(map[string]any)
		if !ok || len(schemas) == 0 {
			t.Fatal("openapi components.schemas missing")
		}
	})

	t.Run("prompts_to_validator_pointers", func(t *testing.T) {
		promptMapPath := filepath.Join(root, "spec", "traceability", "prompt_validator_map.csv")
		workflowMapPath := filepath.Join(root, "spec", "traceability", "workflow_state_map.csv")
		assertFileNonEmpty(t, promptMapPath)
		assertFileNonEmpty(t, workflowMapPath)

		rows := readCSVRows(t, promptMapPath)
		if len(rows) < 2 {
			t.Fatalf("prompt validator map is empty: %s", promptMapPath)
		}
		seenPrompt := map[string]struct{}{}
		for i, row := range rows {
			if i == 0 {
				continue
			}
			if len(row) < 2 {
				t.Fatalf("invalid row at %d in %s", i+1, promptMapPath)
			}
			promptID := strings.TrimSpace(row[0])
			schemaID := strings.TrimSpace(row[1])
			if promptID == "" || schemaID == "" {
				t.Fatalf("empty prompt/schema mapping at row %d", i+1)
			}
			if _, exists := seenPrompt[promptID]; exists {
				t.Fatalf("duplicate prompt mapping: %s", promptID)
			}
			seenPrompt[promptID] = struct{}{}
			assertFileNonEmpty(t, filepath.Join(root, "schemas", schemaID))
		}

		for _, promptSeed := range []string{"seed_prompts_v9.txt", "seed_prompts_v91.txt", "seed_prompts_v92.txt"} {
			seedPath := filepath.Join(root, "prompts", promptSeed)
			body, err := os.ReadFile(seedPath)
			if err != nil {
				t.Fatalf("read prompt seed: %v", err)
			}
			for _, line := range strings.Split(string(body), "\n") {
				promptID := strings.TrimSpace(line)
				if promptID == "" {
					continue
				}
				if _, ok := seenPrompt[promptID]; !ok {
					t.Fatalf("prompt missing validator mapping: %s", promptID)
				}
			}
		}
	})

	t.Run("workflows_to_state_machines", func(t *testing.T) {
		mapRows := readCSVRows(t, filepath.Join(root, "spec", "traceability", "workflow_state_map.csv"))
		workflowsSource := readFileString(t, filepath.Join(root, "internal", "workflows", "service.go"))
		for i, row := range mapRows {
			if i == 0 {
				continue
			}
			if len(row) < 2 {
				t.Fatalf("invalid workflow map row %d", i+1)
			}
			workflowID := strings.TrimSpace(row[0])
			stateMachine := strings.TrimSpace(row[1])
			if workflowID == "" || stateMachine == "" {
				t.Fatalf("invalid workflow mapping at row %d", i+1)
			}
			if !strings.Contains(workflowsSource, workflowID) {
				t.Fatalf("workflow id missing from source mapping: %s", workflowID)
			}
		}
	})

	t.Run("tables_to_migrations", func(t *testing.T) {
		assertMinCount(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"), `(?mi)^CREATE TABLE\s+`, 77)
		assertMinCount(t, filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql"), `(?mi)^CREATE TABLE\s+`, 23)
		assertMinCount(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"), `(?mi)^CREATE TABLE\s+`, 47)
	})

	t.Run("compliance_matrix_row_count_validation", func(t *testing.T) {
		assertCSVMinRows(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v9.csv"), 2)
		assertCSVMinRows(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"), 12)
		assertCSVMinRows(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"), 19)
	})
}

func assertNoDuplicatePromptIDs(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prompt file %s: %v", path, err)
	}
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(body), "\n") {
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate prompt id in %s: %s", path, id)
		}
		seen[id] = struct{}{}
	}
}

func assertNoDuplicateCreateStatements(t *testing.T, path, pattern string) {
	t.Helper()
	body := readFileString(t, path)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(body, -1)
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(match[1]))
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate create statement in %s: %s", path, name)
		}
		seen[name] = struct{}{}
	}
}

func assertMinCount(t *testing.T, path, pattern string, min int) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	count := len(re.FindAllString(readFileString(t, path), -1))
	if count < min {
		t.Fatalf("count in %s too low for pattern %s: got=%d want_at_least=%d", path, pattern, count, min)
	}
}

func assertCSVMinRows(t *testing.T, path string, minRows int) {
	t.Helper()
	rows := readCSVRows(t, path)
	if len(rows) < minRows {
		t.Fatalf("csv has too few rows %s: got=%d want_at_least=%d", path, len(rows), minRows)
	}
}

func readCSVRows(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv %s: %v", path, err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse csv %s: %v", path, err)
	}
	return rows
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(body)
}
