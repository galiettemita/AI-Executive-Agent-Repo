package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchResearchSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	skillIDs := []string{
		"brave-search",
		"exa",
		"firecrawl-search",
		"kagi-search",
		"last30days",
		"literature-review",
		"news-aggregator",
		"perplexity",
		"serpapi",
		"tavily",
		"gemini-deep-research",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read search/research integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("search/research integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for search/research skill %s", skillID)
		}
	}
}
