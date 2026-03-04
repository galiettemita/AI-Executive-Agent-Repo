package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShoppingTransportationSkillIntegrationFixturesClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	skillIDs := []string{
		"shopping-expert",
		"buy-anything",
		"grocery-list",
		"marketplace",
		"recipe-to-list",
		"google-maps",
		"flight-tracker",
		"aviationstack-flight-tracker",
		"aerobase-skill",
		"parcel-package-tracking",
		"track17",
		"post-at",
		"spots",
		"local-places",
		"goplaces",
		"swissweather",
	}

	for _, skillID := range skillIDs {
		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read shopping/transportation integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("shopping/transportation integration test still scaffold-only for %s", skillID)
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
			t.Fatalf("missing fixture json for shopping/transportation skill %s", skillID)
		}
	}
}
