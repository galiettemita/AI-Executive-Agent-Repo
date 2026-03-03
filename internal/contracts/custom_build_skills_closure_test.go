package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCustomBuildSkillScaffoldsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
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

		assertFileContainsTokens(t, filepath.Join(skillsRoot, skillID, "index.ts"), []string{
			"CUSTOM_BUILD_REQUIRED: Awaiting API partnership",
		})
	}
}
