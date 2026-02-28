package sandbox

import (
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Violation struct {
	Target    string    `json:"target"`
	Reason    string    `json:"reason"`
	Profile   string    `json:"profile"`
	Timestamp time.Time `json:"timestamp"`
}

type Profile struct {
	Name              string   `json:"name"`
	EnforceHTTPS      bool     `json:"enforce_https"`
	AllowHostSuffixes []string `json:"allow_host_suffixes"`
	DenyHostSuffixes  []string `json:"deny_host_suffixes"`
}

type Service struct {
	mu           sync.RWMutex
	violations   []Violation
	profiles     map[string]Profile
	blockedCIDRs []*net.IPNet
	now          func() time.Time
}

func NewService() *Service {
	cidrs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
	}
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			parsed = append(parsed, network)
		}
	}

	return &Service{
		violations: []Violation{},
		profiles: map[string]Profile{
			"default": {
				Name:              "default",
				EnforceHTTPS:      false,
				AllowHostSuffixes: []string{},
				DenyHostSuffixes:  []string{".internal", ".local"},
			},
		},
		blockedCIDRs: parsed,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) UpsertProfile(profile Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = "default"
	}
	profile.Name = name
	s.profiles[name] = profile
}

func (s *Service) IsAllowedURL(rawURL string) (bool, string) {
	return s.IsAllowedURLWithProfile("default", rawURL)
}

func (s *Service) IsAllowedURLWithProfile(profileName, rawURL string) (bool, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		s.recordViolation(rawURL, profileName, "INVALID_URL")
		return false, "INVALID_URL"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		s.recordViolation(rawURL, profileName, "SCHEME_NOT_ALLOWED")
		return false, "SCHEME_NOT_ALLOWED"
	}

	s.mu.RLock()
	profile, ok := s.profiles[profileName]
	if !ok {
		profile = s.profiles["default"]
	}
	blockedCIDRs := append([]*net.IPNet(nil), s.blockedCIDRs...)
	s.mu.RUnlock()

	if profile.EnforceHTTPS && parsed.Scheme != "https" {
		s.recordViolation(rawURL, profile.Name, "HTTPS_REQUIRED")
		return false, "HTTPS_REQUIRED"
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		s.recordViolation(rawURL, profile.Name, "HOST_MISSING")
		return false, "HOST_MISSING"
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		s.recordViolation(rawURL, profile.Name, "LOCALHOST_BLOCKED")
		return false, "LOCALHOST_BLOCKED"
	}
	if host == "metadata.google.internal" || host == "169.254.169.254" {
		s.recordViolation(rawURL, profile.Name, "IMDS_BLOCKED")
		return false, "IMDS_BLOCKED"
	}

	for _, suffix := range profile.DenyHostSuffixes {
		suffix = strings.ToLower(strings.TrimSpace(suffix))
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(host, suffix) {
			s.recordViolation(rawURL, profile.Name, "HOST_SUFFIX_DENIED")
			return false, "HOST_SUFFIX_DENIED"
		}
	}
	if len(profile.AllowHostSuffixes) > 0 {
		allowed := false
		for _, suffix := range profile.AllowHostSuffixes {
			suffix = strings.ToLower(strings.TrimSpace(suffix))
			if suffix == "" {
				continue
			}
			if host == suffix || strings.HasSuffix(host, suffix) {
				allowed = true
				break
			}
		}
		if !allowed {
			s.recordViolation(rawURL, profile.Name, "HOST_NOT_ALLOWED")
			return false, "HOST_NOT_ALLOWED"
		}
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			s.recordViolation(rawURL, profile.Name, "LOOPBACK_BLOCKED")
			return false, "LOOPBACK_BLOCKED"
		}
		for _, blockedCIDR := range blockedCIDRs {
			if blockedCIDR.Contains(ip) {
				reason := "PRIVATE_IP_BLOCKED"
				if ip.String() == "169.254.169.254" {
					reason = "IMDS_BLOCKED"
				}
				s.recordViolation(rawURL, profile.Name, reason)
				return false, reason
			}
		}
	}

	return true, "ok"
}

func (s *Service) ListViolations() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Violation, len(s.violations))
	copy(out, s.violations)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].Target < out[j].Target
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func (s *Service) recordViolation(target, profile, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.violations = append(s.violations, Violation{
		Target:    target,
		Reason:    reason,
		Profile:   profile,
		Timestamp: s.now(),
	})
}
