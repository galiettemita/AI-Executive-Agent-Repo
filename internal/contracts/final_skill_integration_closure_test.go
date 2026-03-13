package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	skillIDs := []string{
		"content-advisory",
		"clawd-coach",
		"dexcom",
		"chromecast",
		"camsnap",
		"de-ai-ify",
		"craft",
		"gifhorse",
		"sonoscli",
		"fal-ai",
		"george",
		"gamma",
		"healthkit-sync-apple",
		"meal-planner",
		"figma",
		"pros-cons",
		"healthkit-sync",
		"home-assistant",
		"veo",
		"overseerr",
		"krea-api",
		"coloring-page",
		"mole-mac-cleanup",
		"sleep-calculator",
		"sports-ticker",
		"excalidraw-flowchart",
		"samsung-smart-tv",
		"radarr",
		"pollinations",
		"granola",
		"sonarr",
		"roku",
		"journal-to-post",
		"withings-health",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read final integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("final integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for final skill %s", skillID)
		}
	}
}
