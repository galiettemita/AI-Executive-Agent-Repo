package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceDocumentsSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	skillIDs := []string{
		"copilot-money",
		"expense-tracker-pro",
		"financial-market-analysis",
		"monarch-money",
		"plaid",
		"watch-my-money",
		"yahoo-finance",
		"ynab",
		"tax-professional",
		"ibkr-trading",
		"just-fucking-cancel",
		"pdf-tools",
		"resume-builder",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read finance/doc integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("finance/doc integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for finance/doc skill %s", skillID)
		}
	}
}
