package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomBuildSkillScaffoldsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	customIDs := []string{
		"restaurant-reservations",
		"food-delivery-ordering",
		"ride-hailing",
		"hotel-vacation-booking",
		"bill-pay-p2p",
		"streaming-recommendations",
		"local-service-booking",
		"kids-family-management",
		"pharmacy-prescription",
		"pet-care",
	}
	requiredFiles := []string{
		"index.ts",
		"schema.ts",
		"client.ts",
		"types.ts",
		"README.md",
		filepath.Join("__tests__", "unit.test.ts"),
		filepath.Join("__tests__", "integration.test.ts"),
		filepath.Join("__tests__", "fixtures", ".gitkeep"),
	}

	for _, skillID := range customIDs {
		for _, file := range requiredFiles {
			path := filepath.Join(skillsRoot, skillID, file)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("missing custom-build skill scaffold file for %s: %s (%v)", skillID, path, err)
			}
			if info.IsDir() {
				t.Fatalf("expected file but found directory for %s: %s", skillID, path)
			}
		}

		integrationPath := filepath.Join(skillsRoot, skillID, "__tests__", "integration.test.ts")
		integrationBody, err := os.ReadFile(integrationPath)
		if err != nil {
			t.Fatalf("read custom-build integration test for %s: %v", skillID, err)
		}
		if strings.Contains(string(integrationBody), "scaffold compiles") {
			t.Fatalf("custom-build integration test still scaffold-only for %s", skillID)
		}

		fixtureDir := filepath.Join(skillsRoot, skillID, "__tests__", "fixtures")
		fixtureEntries, err := os.ReadDir(fixtureDir)
		if err != nil {
			t.Fatalf("read fixture directory for %s: %v", skillID, err)
		}
		jsonFixtureCount := 0
		for _, entry := range fixtureEntries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) == ".json" {
				jsonFixtureCount++
			}
		}
		if jsonFixtureCount == 0 {
			t.Fatalf("missing fixture json for custom-build skill %s", skillID)
		}

		assertFileContainsTokens(t, filepath.Join(skillsRoot, skillID, "index.ts"), []string{
			"CUSTOM_BUILD_REQUIRED: Awaiting API partnership",
		})
	}
}
