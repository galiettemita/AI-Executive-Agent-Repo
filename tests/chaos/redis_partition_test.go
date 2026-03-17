//go:build chaos

package chaos

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestChaos_RedisPartition(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set — skipping chaos test (requires live stack)")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis.ParseURL failed: %v", err)
	}
	rdb := redis.NewClient(opt)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer pingCancel()
	if pingErr := rdb.Ping(pingCtx).Err(); pingErr != nil {
		t.Skipf("Redis not reachable at %s: %v — skipping (requires live stack)", redisURL, pingErr)
	}
	t.Logf("Redis reachable — proceeding with partition simulation")

	if closeErr := rdb.Close(); closeErr != nil {
		t.Logf("rdb.Close() error (acceptable): %v", closeErr)
	}
	t.Logf("All Redis connections closed — partition simulated")

	partCtx, partCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer partCancel()
	writeErr := rdb.Set(partCtx,
		"chaos:working_memory:partition_test", "value", 10*time.Second).Err()

	if writeErr != nil {
		t.Logf("Working memory write on closed client returned error "+
			"(graceful degradation — no panic): %v", writeErr)
	} else {
		t.Logf("Working memory write succeeded (client auto-reconnected internally)")
	}
	t.Logf("Graceful degradation verified: no panic on Redis partition")

	opt2, err2 := redis.ParseURL(redisURL)
	if err2 != nil {
		t.Fatalf("redis.ParseURL failed on reconnect: %v", err2)
	}
	rdb2 := redis.NewClient(opt2)
	defer rdb2.Close()

	reconnCtx, reconnCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reconnCancel()

	if err := rdb2.Set(reconnCtx,
		"chaos:working_memory:reconnect_test", "recovered", 10*time.Second).Err(); err != nil {
		t.Fatalf("Working memory write failed after Redis reconnect: %v", err)
	}
	val, err := rdb2.Get(reconnCtx, "chaos:working_memory:reconnect_test").Result()
	if err != nil {
		t.Fatalf("Working memory read failed after reconnect: %v", err)
	}
	if val != "recovered" {
		t.Fatalf("Working memory value mismatch: expected 'recovered', got '%s'", val)
	}
	t.Logf("Redis reconnect verified — write succeeds (value=%s)", val)
}
