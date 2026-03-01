package executor

import "testing"

func TestGitHTTPSPolicies(t *testing.T) {
	t.Parallel()

	args := CloneArgs("main", false)
	if len(args) < 4 || args[0] != "--depth" || args[1] != "1" {
		t.Fatalf("unexpected shallow clone args: %v", args)
	}
	if IsRepoSizeExceeded(501*1024*1024) == false {
		t.Fatal("expected repository size limit rejection")
	}
	if GitRateLimitRedisKey("github.com", "ws1") != "rl:git:github.com:ws1" {
		t.Fatal("unexpected git rate limit key")
	}
}

func TestValidateGitRemoteURL(t *testing.T) {
	t.Parallel()

	if err := ValidateGitRemoteURL("ssh://github.com/org/repo.git", nil); err == nil {
		t.Fatal("expected ssh remote to be rejected")
	}
	if err := ValidateGitRemoteURL("https://github.com/org/repo.git", nil); err != nil {
		t.Fatalf("expected github https remote to pass: %v", err)
	}
	if err := ValidateGitRemoteURL("https://custom.git.example.com/org/repo.git", []string{"custom.git.example.com"}); err != nil {
		t.Fatalf("expected custom allowed host to pass: %v", err)
	}
}
