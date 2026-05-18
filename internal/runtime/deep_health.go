package runtime

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	DependencyStatusOK            = "ok"
	DependencyStatusNotConfigured = "not_configured"
	DependencyStatusInvalidConfig = "invalid_config"
	DependencyStatusUnreachable   = "unreachable"
)

type DialTimeoutFunc func(network, address string, timeout time.Duration) (net.Conn, error)

// RedisPinger validates Redis connectivity beyond TCP dial.
// Implementations should issue a PING command and return nil on PONG.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

type DeepHealthProbeOptions struct {
	Getenv      func(string) string
	DialTimeout DialTimeoutFunc
	Timeout     time.Duration
	// RedisPinger, when set, is used instead of TCP dial for the Redis check.
	// This allows deep health to validate Redis at the protocol level (PING/PONG).
	RedisPinger RedisPinger
}

type dependencyTarget struct {
	envKey      string
	defaultPort string
	parse       func(raw, defaultPort string) (string, error)
}

func DeepDependencyChecks(getenv func(string) string) map[string]string {
	return DeepDependencyChecksWithOptions(DeepHealthProbeOptions{Getenv: getenv})
}

func DeepDependencyChecksWithOptions(options DeepHealthProbeOptions) map[string]string {
	if options.Getenv == nil {
		options.Getenv = func(_ string) string { return "" }
	}
	if options.DialTimeout == nil {
		options.DialTimeout = net.DialTimeout
	}
	if options.Timeout <= 0 {
		options.Timeout = 300 * time.Millisecond
	}

	targets := map[string]dependencyTarget{
		"db": {
			envKey:      "DATABASE_URL",
			defaultPort: "5432",
			parse:       parseDatabaseAddress,
		},
		"redis": {
			envKey:      "REDIS_URL",
			defaultPort: "6379",
			parse:       parseURLAddress,
		},
		"temporal": {
			envKey:      "TEMPORAL_HOST",
			defaultPort: "7233",
			parse:       parseHostPortAddress,
		},
	}

	checks := make(map[string]string, len(targets))
	for checkKey, target := range targets {
		raw := strings.TrimSpace(options.Getenv(target.envKey))
		if raw == "" {
			checks[checkKey] = DependencyStatusNotConfigured
			continue
		}

		// Redis: use protocol-level PING when a RedisPinger is available.
		if checkKey == "redis" && options.RedisPinger != nil {
			ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
			err := options.RedisPinger.Ping(ctx)
			cancel()
			if err != nil {
				checks[checkKey] = DependencyStatusUnreachable
			} else {
				checks[checkKey] = DependencyStatusOK
			}
			continue
		}

		address, err := target.parse(raw, target.defaultPort)
		if err != nil {
			checks[checkKey] = DependencyStatusInvalidConfig
			continue
		}

		conn, err := options.DialTimeout("tcp", address, options.Timeout)
		if err != nil {
			checks[checkKey] = DependencyStatusUnreachable
			continue
		}
		_ = conn.Close()
		checks[checkKey] = DependencyStatusOK
	}

	return checks
}

func parseDatabaseAddress(raw, defaultPort string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty database dsn")
	}
	if strings.Contains(trimmed, "://") {
		return parseURLAddress(trimmed, defaultPort)
	}

	host := ""
	port := defaultPort
	for _, token := range strings.Fields(trimmed) {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "host":
			host = value
		case "port":
			if value != "" {
				port = value
			}
		}
	}

	if host == "" {
		return "", fmt.Errorf("database dsn missing host")
	}
	return normalizeHostPort(host, port)
}

func parseURLAddress(raw, defaultPort string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	return normalizeHostPort(parsed.Host, defaultPort)
}

func parseHostPortAddress(raw, defaultPort string) (string, error) {
	return normalizeHostPort(strings.TrimSpace(raw), defaultPort)
}

func normalizeHostPort(hostPort, defaultPort string) (string, error) {
	trimmed := strings.TrimSpace(hostPort)
	if trimmed == "" {
		return "", fmt.Errorf("missing host")
	}

	if host, port, err := net.SplitHostPort(trimmed); err == nil {
		if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
			return "", fmt.Errorf("invalid host:port")
		}
		return net.JoinHostPort(host, port), nil
	}

	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return net.JoinHostPort(strings.Trim(trimmed, "[]"), defaultPort), nil
	}

	if strings.Count(trimmed, ":") > 1 {
		return net.JoinHostPort(trimmed, defaultPort), nil
	}

	if strings.Contains(trimmed, ":") {
		host, port, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
			return "", fmt.Errorf("invalid host:port")
		}
		return net.JoinHostPort(host, port), nil
	}

	return net.JoinHostPort(trimmed, defaultPort), nil
}
