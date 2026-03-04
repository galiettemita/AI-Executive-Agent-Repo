package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommunicationSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	communicationSkillIDs := []string{
		"smtp-send",
		"react-email-skills",
		"apple-mail",
		"apple-mail-search",
		"bluesky",
		"reddit",
		"bird",
		"outlook",
		"imap-email",
		"google-workspace",
		"slack",
	}

	for _, skillID := range communicationSkillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read communication integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("communication integration test still scaffold-only for %s", skillID)
		}

		fixtureDir := filepath.Join(skillsRoot, skillID, "__tests__", "fixtures")
		entries, err := os.ReadDir(fixtureDir)
		if err != nil {
			t.Fatalf("read fixture directory for %s: %v", skillID, err)
		}

		jsonFixtureCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) == ".json" {
				jsonFixtureCount++
			}
		}

		if jsonFixtureCount == 0 {
			t.Fatalf("missing fixture json for communication skill %s", skillID)
		}
	}
}
