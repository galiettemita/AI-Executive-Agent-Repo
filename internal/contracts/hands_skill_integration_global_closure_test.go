package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandsSkillIntegrationFixturesGlobalClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read skills root: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no skill directories found under brevio-hands")
	}

	checked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		skillID := entry.Name()
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, readErr := os.ReadFile(integrationPath)
		if readErr != nil {
			t.Fatalf("read integration test for %s: %v", skillID, readErr)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("integration test still scaffold-only for %s", skillID)
		}

		fixtureDir := filepath.Join(skillsRoot, skillID, "__tests__", "fixtures")
		fixtureEntries, dirErr := os.ReadDir(fixtureDir)
		if dirErr != nil {
			t.Fatalf("read fixture directory for %s: %v", skillID, dirErr)
		}

		jsonCount := 0
		for _, fixtureEntry := range fixtureEntries {
			if fixtureEntry.IsDir() {
				continue
			}
			if filepath.Ext(fixtureEntry.Name()) == ".json" {
				jsonCount++
			}
		}
		if jsonCount == 0 {
			t.Fatalf("missing fixture json for skill %s", skillID)
		}

		checked++
	}

	if checked == 0 {
		t.Fatal("no skill directories were validated")
	}
}
