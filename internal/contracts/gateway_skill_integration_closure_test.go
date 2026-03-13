package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewaySkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	gatewaySkillIDs := []string{
		"asr",
		"openai-tts",
		"gemini-stt",
		"sag",
		"voice-wake-say",
		"whatsapp-styling-guide",
		"vocal-chat",
		"autoresponder",
	}

	for _, skillID := range gatewaySkillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read gateway integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("gateway integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for gateway skill %s", skillID)
		}
	}
}
