package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductivitySkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	skillIDs := []string{
		"asana",
		"clickup-mcp",
		"jira",
		"linear",
		"omnifocus",
		"things-mac",
		"ticktick",
		"todo",
		"todoist",
		"trello",
		"calctl",
		"apple-remind-me",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read productivity integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("productivity integration test still scaffold-only for %s", skillID)
		}

		fixtureDir := filepath.Join(skillsRoot, skillID, "__tests__", "fixtures")
		entries, err := os.ReadDir(fixtureDir)
		if err != nil {
			t.Fatalf("read fixture directory for %s: %v", skillID, err)
		}

		jsonCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) == ".json" {
				jsonCount++
			}
		}
		if jsonCount == 0 {
			t.Fatalf("missing fixture json for productivity skill %s", skillID)
		}
	}
}
