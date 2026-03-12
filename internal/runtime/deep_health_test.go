package runtime

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDeepDependencyChecksWithOptionsNotConfigured(t *testing.T) {
	t.Parallel()

	checks := DeepDependencyChecksWithOptions(DeepHealthProbeOptions{
		Getenv: func(_ string) string { return "" },
		DialTimeout: func(_, _ string, _ time.Duration) (net.Conn, error) {
			t.Fatal("dial should not execute when vars are missing")
			return nil, nil
		},
	})

	for _, key := range []string{"db", "redis", "temporal"} {
		if checks[key] != DependencyStatusNotConfigured {
			t.Fatalf("unexpected status for %s: got=%s want=%s", key, checks[key], DependencyStatusNotConfigured)
		}
	}
}

func TestDeepDependencyChecksWithOptionsConnectivity(t *testing.T) {
	t.Parallel()

	var dialed []string
	checks := DeepDependencyChecksWithOptions(DeepHealthProbeOptions{
		Getenv: func(key string) string {
			switch key {
			case "DATABASE_URL":
				return "postgres://user@db.internal:5432/brevio"
			case "REDIS_URL":
				return "redis://redis.internal:6379/0"
			case "TEMPORAL_HOST":
				return "temporal.internal:7233"
			default:
				return ""
			}
		},
		DialTimeout: func(_, address string, _ time.Duration) (net.Conn, error) {
			dialed = append(dialed, address)
			if address == "redis.internal:6379" {
				return nil, errors.New("redis down")
			}
			left, right := net.Pipe()
			_ = right.Close()
			return left, nil
		},
	})

	if checks["db"] != DependencyStatusOK {
		t.Fatalf("unexpected db status: %s", checks["db"])
	}
	if checks["redis"] != DependencyStatusUnreachable {
		t.Fatalf("unexpected redis status: %s", checks["redis"])
	}
	if checks["temporal"] != DependencyStatusOK {
		t.Fatalf("unexpected temporal status: %s", checks["temporal"])
	}

	if len(dialed) != 3 {
		t.Fatalf("unexpected dial count: got=%d want=3", len(dialed))
	}
}

func TestDeepDependencyChecksWithOptionsInvalidConfig(t *testing.T) {
	t.Parallel()

	checks := DeepDependencyChecksWithOptions(DeepHealthProbeOptions{
		Getenv: func(key string) string {
			switch key {
			case "DATABASE_URL":
				return "dbname=brevio user=svc"
			case "REDIS_URL":
				return "redis://"
			case "TEMPORAL_HOST":
				return "temporal.internal:"
			default:
				return ""
			}
		},
		DialTimeout: func(_, _ string, _ time.Duration) (net.Conn, error) {
			t.Fatal("dial should not execute for invalid config")
			return nil, nil
		},
	})

	if checks["db"] != DependencyStatusInvalidConfig {
		t.Fatalf("unexpected db status: %s", checks["db"])
	}
	if checks["redis"] != DependencyStatusInvalidConfig {
		t.Fatalf("unexpected redis status: %s", checks["redis"])
	}
	if checks["temporal"] != DependencyStatusInvalidConfig {
		t.Fatalf("unexpected temporal status: %s", checks["temporal"])
	}
}

// TestDeepDependencyChecksWithRedisPinger validates that when a RedisPinger is
// provided, the health check uses PING instead of TCP dial for Redis.
func TestDeepDependencyChecksWithRedisPinger(t *testing.T) {
	t.Parallel()

	var dialed []string
	checks := DeepDependencyChecksWithOptions(DeepHealthProbeOptions{
		Getenv: func(key string) string {
			switch key {
			case "DATABASE_URL":
				return "postgres://user@db.internal:5432/brevio"
			case "REDIS_URL":
				return "redis://redis.internal:6379/0"
			case "TEMPORAL_HOST":
				return "temporal.internal:7233"
			default:
				return ""
			}
		},
		DialTimeout: func(_, address string, _ time.Duration) (net.Conn, error) {
			dialed = append(dialed, address)
			left, right := net.Pipe()
			_ = right.Close()
			return left, nil
		},
		RedisPinger: &fakePinger{err: nil},
	})

	if checks["redis"] != DependencyStatusOK {
		t.Fatalf("expected redis OK via PING, got %s", checks["redis"])
	}
	// Redis should NOT have been TCP-dialed.
	for _, addr := range dialed {
		if addr == "redis.internal:6379" {
			t.Fatal("Redis should not be TCP-dialed when RedisPinger is set")
		}
	}
	// Only db and temporal should be dialed.
	if len(dialed) != 2 {
		t.Fatalf("expected 2 dials (db+temporal), got %d: %v", len(dialed), dialed)
	}
}

// TestDeepDependencyChecksWithRedisPingerFailure verifies that a failed PING
// reports unreachable.
func TestDeepDependencyChecksWithRedisPingerFailure(t *testing.T) {
	t.Parallel()

	checks := DeepDependencyChecksWithOptions(DeepHealthProbeOptions{
		Getenv: func(key string) string {
			if key == "REDIS_URL" {
				return "redis://redis.internal:6379"
			}
			return ""
		},
		RedisPinger: &fakePinger{err: errors.New("connection refused")},
	})

	if checks["redis"] != DependencyStatusUnreachable {
		t.Fatalf("expected redis unreachable on PING failure, got %s", checks["redis"])
	}
}

type fakePinger struct {
	err error
}

func (f *fakePinger) Ping(_ context.Context) error {
	return f.err
}

func TestParseDatabaseAddress(t *testing.T) {
	t.Parallel()

	address, err := parseDatabaseAddress("host=db.internal port=6432 dbname=brevio", "5432")
	if err != nil {
		t.Fatalf("parse key value dsn: %v", err)
	}
	if address != "db.internal:6432" {
		t.Fatalf("unexpected parsed address: %s", address)
	}

	address, err = parseDatabaseAddress("postgres://svc@db.internal/brevio", "5432")
	if err != nil {
		t.Fatalf("parse url dsn: %v", err)
	}
	if address != "db.internal:5432" {
		t.Fatalf("unexpected parsed url address: %s", address)
	}
}
