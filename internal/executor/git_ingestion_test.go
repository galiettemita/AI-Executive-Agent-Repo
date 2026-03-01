package executor

import "testing"

func TestGitIngestionHelpers(t *testing.T) {
	t.Parallel()

	if err := ValidateRepositorySize(MaxRepositorySizeBytes + 1); err == nil {
		t.Fatal("expected size-exceeded error")
	}
	cloneCmd, err := BuildShallowCloneCommand("https://github.com/org/repo.git", "main")
	if err != nil || len(cloneCmd) == 0 {
		t.Fatalf("unexpected clone command: cmd=%v err=%v", cloneCmd, err)
	}
	if _, err := BuildShallowCloneCommand("ssh://github.com/org/repo.git", "main"); err == nil {
		t.Fatal("expected non-https clone command rejection")
	}
	if !ShouldRetryClone(500) || ShouldRetryClone(401) {
		t.Fatal("unexpected clone retry policy behavior")
	}
}

func TestAnalyzeRepositoryFileTree(t *testing.T) {
	t.Parallel()

	profile := AnalyzeRepositoryFileTree([]string{
		"main.go",
		"infra/main.tf",
		".github/workflows/ci.yaml",
		"package.json",
	})
	if profile.TotalFiles != 4 || len(profile.Languages) == 0 || len(profile.TechStack) == 0 {
		t.Fatalf("unexpected repo profile: %+v", profile)
	}
}
