package sandbox

import "testing"

func TestSandboxURLFiltering(t *testing.T) {
	t.Parallel()

	s := NewService()
	if allowed, _ := s.IsAllowedURL("https://example.com/resource"); !allowed {
		t.Fatalf("expected public URL to be allowed")
	}
	if allowed, _ := s.IsAllowedURL("http://169.254.169.254/latest/meta-data"); allowed {
		t.Fatalf("expected IMDS URL to be blocked")
	}
	if allowed, _ := s.IsAllowedURL("http://127.0.0.1/admin"); allowed {
		t.Fatalf("expected loopback URL to be blocked")
	}
	if len(s.ListViolations()) < 2 {
		t.Fatalf("expected recorded sandbox violations")
	}
}

func TestSandboxProfileEnforcement(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertProfile(Profile{
		Name:              "strict",
		EnforceHTTPS:      true,
		AllowHostSuffixes: []string{".example.com"},
	})

	if allowed, reason := s.IsAllowedURLWithProfile("strict", "http://api.example.com/path"); allowed || reason != "HTTPS_REQUIRED" {
		t.Fatalf("expected HTTPS_REQUIRED, got allowed=%v reason=%s", allowed, reason)
	}
	if allowed, reason := s.IsAllowedURLWithProfile("strict", "https://api.not-allowed.com/path"); allowed || reason != "HOST_NOT_ALLOWED" {
		t.Fatalf("expected HOST_NOT_ALLOWED, got allowed=%v reason=%s", allowed, reason)
	}
	if allowed, reason := s.IsAllowedURLWithProfile("strict", "https://api.example.com/path"); !allowed || reason != "ok" {
		t.Fatalf("expected strict profile allow, got allowed=%v reason=%s", allowed, reason)
	}
}

func TestSandboxBlocksPrivateCIDRs(t *testing.T) {
	t.Parallel()

	s := NewService()
	if allowed, reason := s.IsAllowedURL("https://10.0.0.8/service"); allowed || reason != "PRIVATE_IP_BLOCKED" {
		t.Fatalf("expected private ip block, got allowed=%v reason=%s", allowed, reason)
	}
}
