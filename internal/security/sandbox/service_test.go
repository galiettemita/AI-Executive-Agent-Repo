package sandbox

import "testing"

func TestSandboxURLFiltering(t *testing.T) {
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
