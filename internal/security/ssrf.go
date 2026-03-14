package security

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// DNSResolver is the interface for hostname resolution, allowing injection of
// test doubles for DNS rebinding attack simulation.
type DNSResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// blockedPrefixes are hostname prefixes known to be internal or sensitive.
var blockedPrefixes = []string{"169.254.169.254", "127.", "::1"}

// blockedCIDRStrings lists all CIDR ranges that must be blocked from outbound
// requests to prevent SSRF attacks against internal infrastructure.
var blockedCIDRStrings = []string{
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"100.64.0.0/10",
	"198.18.0.0/15",
	"0.0.0.0/8",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"fd00::/8",
}

// BlockedCIDRs is the parsed set of blocked CIDR prefixes.
var BlockedCIDRs = func() []netip.Prefix {
	out := make([]netip.Prefix, 0, len(blockedCIDRStrings))
	for _, cidr := range blockedCIDRStrings {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		out = append(out, prefix)
	}
	return out
}()

// ValidateTargetURL performs comprehensive SSRF validation on a target URL
// using the system default DNS resolver.
func ValidateTargetURL(target string) error {
	return ValidateTargetURLWithResolver(target, net.DefaultResolver, 500*time.Millisecond)
}

// ValidateTargetURLWithResolver performs SSRF validation with a caller-supplied
// DNS resolver and timeout. Use this variant when you need to inject a custom
// resolver (e.g., for DNS rebinding tests).
func ValidateTargetURLWithResolver(target string, resolver DNSResolver, dnsTimeout time.Duration) error {
	if target == "" {
		return nil
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("ssrf: invalid target url: %w", err)
	}

	// Scheme validation: only http and https allowed.
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("ssrf: blocked scheme: %s", scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("ssrf: missing host")
	}

	// Block well-known internal hostnames.
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" {
		return fmt.Errorf("ssrf: blocked host: %s", host)
	}

	// Block by prefix (127.*, 169.254.169.254, ::1).
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(lowerHost, prefix) {
			return fmt.Errorf("ssrf: blocked host: %s", host)
		}
	}

	// If the host is a literal IP, validate it directly.
	ip := net.ParseIP(host)
	if ip != nil {
		return validateBlockedIP(host, ip)
	}

	// DNS resolution: resolve hostname and check all returned IPs.
	if resolver == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	resolved, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		// DNS resolution failed — hostname-level checks above still apply.
		// Fail open on DNS to avoid blocking legitimate hosts during DNS hiccups.
		return nil
	}
	for _, ipAddr := range resolved {
		if err := validateBlockedIP(host, ipAddr.IP); err != nil {
			return err
		}
	}
	return nil
}

// validateBlockedIP checks whether an IP address falls within blocked CIDR
// ranges or is a loopback/private/link-local address.
func validateBlockedIP(host string, ip net.IP) error {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return fmt.Errorf("ssrf: blocked loopback address: %s", host)
	}
	if addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return fmt.Errorf("ssrf: blocked private address: %s", host)
	}
	for _, prefix := range BlockedCIDRs {
		if prefix.Contains(addr) {
			if addr.String() == "169.254.169.254" {
				return fmt.Errorf("ssrf: blocked metadata address: %s", host)
			}
			return fmt.Errorf("ssrf: blocked private address: %s", host)
		}
	}
	return nil
}
