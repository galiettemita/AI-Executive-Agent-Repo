package sandbox

import (
	"net"
	"net/url"
	"strings"
	"sync"
)

type Violation struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type Service struct {
	mu         sync.RWMutex
	violations []Violation
}

func NewService() *Service {
	return &Service{
		violations: []Violation{},
	}
}

func (s *Service) IsAllowedURL(rawURL string) (bool, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		s.recordViolation(rawURL, "INVALID_URL")
		return false, "INVALID_URL"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		s.recordViolation(rawURL, "SCHEME_NOT_ALLOWED")
		return false, "SCHEME_NOT_ALLOWED"
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		s.recordViolation(rawURL, "LOCALHOST_BLOCKED")
		return false, "LOCALHOST_BLOCKED"
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			s.recordViolation(rawURL, "PRIVATE_IP_BLOCKED")
			return false, "PRIVATE_IP_BLOCKED"
		}
		if ip.String() == "169.254.169.254" {
			s.recordViolation(rawURL, "IMDS_BLOCKED")
			return false, "IMDS_BLOCKED"
		}
	}
	return true, "ok"
}

func (s *Service) ListViolations() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Violation, len(s.violations))
	copy(out, s.violations)
	return out
}

func (s *Service) recordViolation(target, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.violations = append(s.violations, Violation{
		Target: target,
		Reason: reason,
	})
}
