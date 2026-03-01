package executor

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"
)

type GitCloneRetryPolicy struct {
	BaseDelaySeconds int
	MaxAttempts      int
	RetryOnAuthError bool
}

func SupportedGitHosts() []string {
	return []string{"github.com", "gitlab.com", "bitbucket.org"}
}

func CloneArgs(defaultBranch string, fullClone bool) []string {
	branch := strings.TrimSpace(defaultBranch)
	if branch == "" {
		branch = "main"
	}
	if fullClone {
		return []string{"--single-branch", "--branch", branch}
	}
	return []string{"--depth", "1", "--single-branch", "--branch", branch}
}

func GitRateLimitRedisKey(host, workspaceID string) string {
	return "rl:git:" + strings.ToLower(strings.TrimSpace(host)) + ":" + workspaceID
}

func GitCloneRetryDefaults() GitCloneRetryPolicy {
	return GitCloneRetryPolicy{
		BaseDelaySeconds: 2,
		MaxAttempts:      3,
		RetryOnAuthError: false,
	}
}

func IsRepoSizeExceeded(sizeBytes int64) bool {
	const maxBytes = 500 * 1024 * 1024
	return sizeBytes > maxBytes
}

func ValidateGitRemoteURL(remoteURL string, customAllowedHosts []string) error {
	parsed, err := url.Parse(strings.TrimSpace(remoteURL))
	if err != nil {
		return fmt.Errorf("invalid remote url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("only https remotes are supported")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return fmt.Errorf("missing host")
	}

	allowedHosts := append([]string{}, SupportedGitHosts()...)
	for _, host := range customAllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowedHosts = append(allowedHosts, host)
		}
	}
	if !slices.Contains(allowedHosts, host) {
		return fmt.Errorf("remote host not allowed: %s", host)
	}

	ip := net.ParseIP(host)
	if ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if ok {
			addr = addr.Unmap()
			if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
				return fmt.Errorf("blocked private remote host: %s", host)
			}
		}
	}
	return nil
}
