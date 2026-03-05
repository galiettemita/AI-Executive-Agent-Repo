package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	profileSource := filepath.Join(root, "services", "brevio-profile", "src", "index.ts")
	profileReadme := filepath.Join(root, "services", "brevio-profile", "README.md")

	assertFileContainsTokens(t, profileSource, []string{
		"USER.md",
		"SOUL.md",
		"AGENTS.md",
		"profile_hash",
		"preferences",
		"knowledge",
		"hash",
		"refresh",
		"BREVIO_PROFILE_DATA_DIR",
		"profile.knowledge.updated",
		"createProfileRuntime",
	})

	assertFileContainsTokens(t, profileReadme, []string{
		"Profile and knowledge-file service",
		"GET /api/v1/profile/:user_id",
		"PUT /api/v1/profile/:user_id/knowledge/:file",
		"POST /api/v1/profile/:user_id/hash/refresh",
		"USER.md",
		"SOUL.md",
		"AGENTS.md",
	})

	body, err := os.ReadFile(profileReadme)
	if err != nil {
		t.Fatalf("read profile readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("profile README still contains scaffold marker")
	}
}
