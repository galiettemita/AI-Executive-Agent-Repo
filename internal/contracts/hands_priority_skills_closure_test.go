package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandsPrioritySkillsNoLongerScaffolded(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	scriptPath := filepath.Join(root, "scripts", "skills", "generate_hands_skill_scaffolds.sh")

	schemaTokens := map[string][]string{
		"shopping-expert": {
			"query",
			"results",
			"mock_catalog",
		},
		"google-maps": {
			"origin",
			"destination",
			"distance_m",
		},
		"google-calendar": {
			"action",
			"confirmation_required",
		},
		"tavily": {
			"query",
			"results",
			"provider",
			"tavily",
		},
		"smtp-send": {
			"to",
			"subject",
			"confirmation_required",
			"confirmed",
		},
		"home-assistant": {
			"entity_id",
			"two_factor_code",
		},
		"todoist": {
			"action",
			"project_id",
			"task",
		},
		"youtube-api": {
			"mode",
			"video_id",
			"transcript",
		},
		"ynab": {
			"action",
			"budget_id",
			"total_budget_cents",
		},
		"notion": {
			"action",
			"page_id",
			"create_page",
		},
		"fal-ai": {
			"prompt",
			"image_url",
			"model_used",
		},
		"apple-contacts": {
			"query",
			"contacts",
			"apple-contacts-local",
		},
	}

	indexTokens := map[string][]string{
		"shopping-expert": {"VALIDATION_FAILED"},
		"google-maps":     {"VALIDATION_FAILED"},
		"google-calendar": {"requiredScopes", "calendar"},
		"tavily":          {"VALIDATION_FAILED"},
		"smtp-send":       {"confirmed", "confirmation_required"},
		"home-assistant":  {"SAFETY_2FA_REQUIRED", "Action requires 2FA confirmation"},
		"todoist":         {"requiredScopes", "TODOIST_CONTENT_REQUIRED"},
		"youtube-api":     {"YOUTUBE_VIDEO_ID_REQUIRED"},
		"ynab":            {"requiredScopes", "YNAB_ACCOUNT_NOT_FOUND"},
		"notion":          {"requiredScopes", "NOTION_TITLE_REQUIRED"},
		"fal-ai":          {"FAL_CONTENT_POLICY_BLOCKED"},
		"apple-contacts":  {"apple-contacts-local"},
	}

	scriptBody, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read skill scaffold script: %v", err)
	}
	scriptText := string(scriptBody)

	for skillID, tokens := range schemaTokens {
		skillDir := filepath.Join(skillsRoot, skillID)
		indexPath := filepath.Join(skillDir, "index.ts")
		schemaPath := filepath.Join(skillDir, "schema.ts")
		readmePath := filepath.Join(skillDir, "README.md")

		assertFileContainsTokens(t, indexPath, append([]string{skillID}, indexTokens[skillID]...))
		assertFileContainsTokens(t, schemaPath, tokens)

		readmeBody, readErr := os.ReadFile(readmePath)
		if readErr != nil {
			t.Fatalf("read %s readme: %v", skillID, readErr)
		}
		if strings.Contains(strings.ToLower(string(readmeBody)), "generated skill adapter scaffold") {
			t.Fatalf("priority skill %s README still contains scaffold marker", skillID)
		}

		if !strings.Contains(scriptText, skillID) {
			t.Fatalf("skill scaffold script manual preserve list missing %s", skillID)
		}
	}
}
