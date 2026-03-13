package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOpenClawSeedMigrationCountsAndModes(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	sqlPath := filepath.Join(root, "migrations", "006_seed_skills.up.sql")
	body := readFileString(t, sqlPath)

	seedIDs := extractSeedIDs(t, body)
	if len(seedIDs) != 153 {
		t.Fatalf("expected 153 seeded skills, got %d", len(seedIDs))
	}

	planeBranches := extractCaseBranchesForAlias(t, body, "plane")
	deploymentBranches := extractCaseBranchesForAlias(t, body, "deployment_mode")
	gateway := branchIDs(t, planeBranches, "gateway")
	brain := branchIDs(t, planeBranches, "brain")
	local := branchIDs(t, deploymentBranches, "local_mac")
	mcp := branchIDs(t, deploymentBranches, "mcp")

	assertSubset(t, gateway, seedIDs, "gateway")
	assertSubset(t, brain, seedIDs, "brain")
	assertSubset(t, local, seedIDs, "local_mac")
	assertSubset(t, mcp, seedIDs, "mcp")

	if got := len(gateway); got != 8 {
		t.Fatalf("expected 8 gateway skills, got %d", got)
	}
	if got := len(brain); got != 16 {
		t.Fatalf("expected 16 brain skills, got %d", got)
	}
	if got := len(seedIDs) - len(union(gateway, brain)); got != 129 {
		t.Fatalf("expected 129 hands skills, got %d", got)
	}
	if got := len(local); got != 24 {
		t.Fatalf("expected 24 local_mac skills, got %d", got)
	}
	if got := len(mcp); got != 2 {
		t.Fatalf("expected 2 mcp skills, got %d", got)
	}
	if got := len(seedIDs) - len(union(local, mcp)); got != 127 {
		t.Fatalf("expected 127 cloud skills, got %d", got)
	}
	if overlap := intersection(gateway, brain); len(overlap) != 0 {
		t.Fatalf("gateway/brain overlap not allowed: %v", setKeys(overlap))
	}
	if overlap := intersection(local, mcp); len(overlap) != 0 {
		t.Fatalf("local_mac/mcp overlap not allowed: %v", setKeys(overlap))
	}
}

func TestOpenClawHandsSkillScaffoldsExistForAllSeededSkills(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	sqlPath := filepath.Join(root, "migrations", "006_seed_skills.up.sql")
	body := readFileString(t, sqlPath)
	seedIDs := extractSeedIDs(t, body)

	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	requiredFiles := []string{
		"index.ts",
		"schema.ts",
		"client.ts",
		"types.ts",
		"README.md",
		filepath.Join("__tests__", "unit.test.ts"),
		filepath.Join("__tests__", "integration.test.ts"),
		filepath.Join("__tests__", "fixtures", ".gitkeep"),
	}

	for skillID := range seedIDs {
		skillDir := filepath.Join(skillsRoot, skillID)
		for _, file := range requiredFiles {
			path := filepath.Join(skillDir, file)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("missing skill scaffold file for %s: %s (%v)", skillID, path, err)
			}
			if info.IsDir() {
				t.Fatalf("expected file but found directory for %s: %s", skillID, path)
			}
		}
	}
}

func TestOpenClawHandsSkillRegistryContainsAllSeededSkills(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	sqlPath := filepath.Join(root, "migrations", "006_seed_skills.up.sql")
	body := readFileString(t, sqlPath)
	seedIDs := extractSeedIDs(t, body)

	registryPath := filepath.Join(root, "services", "hands-runtime", "src", "skills", "index.ts")
	registryBody := readFileString(t, registryPath)
	for skillID := range seedIDs {
		token := "'" + skillID + "':"
		if !strings.Contains(registryBody, token) {
			t.Fatalf("skill registry missing seeded skill mapping token: %s", token)
		}
	}
}

func extractSeedIDs(t *testing.T, sql string) map[string]struct{} {
	t.Helper()

	reUnnest := regexp.MustCompile(`(?s)unnest\(ARRAY\[(.*?)\]\) AS id`)
	reID := regexp.MustCompile(`'([a-z0-9\-]+)'`)
	ids := map[string]struct{}{}
	for _, match := range reUnnest.FindAllStringSubmatch(sql, -1) {
		for _, idMatch := range reID.FindAllStringSubmatch(match[1], -1) {
			ids[idMatch[1]] = struct{}{}
		}
	}
	if len(ids) == 0 {
		t.Fatal("no seed ids extracted from migration")
	}
	return ids
}

func extractCaseBranchesForAlias(t *testing.T, sql, alias string) map[string]map[string]struct{} {
	t.Helper()

	endToken := "END AS " + alias
	endIdx := strings.Index(sql, endToken)
	if endIdx < 0 {
		t.Fatalf("missing CASE block for alias %q", alias)
	}
	startIdx := strings.LastIndex(sql[:endIdx], "CASE")
	if startIdx < 0 || startIdx >= endIdx {
		t.Fatalf("missing CASE block for alias %q", alias)
	}
	caseBody := sql[startIdx+len("CASE") : endIdx]

	reCase := regexp.MustCompile(`(?s)WHEN id = ANY\(ARRAY\[(.*?)\]::text\[\]\) THEN '([a-z_]+)'::text`)
	reID := regexp.MustCompile(`'([a-z0-9\-]+)'`)
	out := map[string]map[string]struct{}{}
	for _, match := range reCase.FindAllStringSubmatch(caseBody, -1) {
		branch := match[2]
		if _, ok := out[branch]; !ok {
			out[branch] = map[string]struct{}{}
		}
		for _, idMatch := range reID.FindAllStringSubmatch(match[1], -1) {
			out[branch][idMatch[1]] = struct{}{}
		}
	}
	return out
}

func branchIDs(t *testing.T, branches map[string]map[string]struct{}, branch string) map[string]struct{} {
	t.Helper()

	ids, ok := branches[branch]
	if !ok {
		t.Fatalf("missing CASE branch %q", branch)
	}
	return ids
}

func assertSubset(t *testing.T, subset, superset map[string]struct{}, name string) {
	t.Helper()
	for id := range subset {
		if _, ok := superset[id]; !ok {
			t.Fatalf("%s skill %q missing from seed list", name, id)
		}
	}
}

func union(a, b map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

func intersection(a, b map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range a {
		if _, ok := b[k]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func setKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
