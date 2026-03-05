package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	brainSkillIDs := []string{
		"doing-tasks",
		"plan-my-day",
		"daily-rhythm",
		"morning-manifesto",
		"personal-shopper",
		"clawringhouse",
		"smart-expense-tracker",
		"card-optimizer",
		"refund-radar",
		"contract-reviewer",
		"meeting-autopilot",
		"proactive-research",
		"focus-mode",
		"thinking-partner",
		"relationship-skills",
		"self-improvement",
	}

	for _, skillID := range brainSkillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read brain integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("brain integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for brain skill %s", skillID)
		}
	}
}
