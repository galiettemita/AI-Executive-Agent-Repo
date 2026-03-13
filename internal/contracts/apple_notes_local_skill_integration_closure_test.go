package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppleNotesLocalSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	skillIDs := []string{
		"alter-actions",
		"apple-contacts",
		"apple-media",
		"apple-notes",
		"apple-notes-skill",
		"apple-photos",
		"bear-notes",
		"better-notion",
		"gkeep",
		"google-calendar",
		"icloud-findmy",
		"notion",
		"obsidian",
		"reflect",
		"second-brain",
		"shortcuts-generator",
		"get-focus-mode",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read apple/notes/local integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("apple/notes/local integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for apple/notes/local skill %s", skillID)
		}
	}
}
