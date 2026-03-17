//go:build chaos

package chaos

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestChaos_DatabaseConnectionDrop(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set — skipping chaos test (requires live stack)")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig failed: %v", err)
	}
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig failed: %v", err)
	}
	defer pool.Close()

	holdCh := make(chan struct{})
	var holders sync.WaitGroup
	var mu sync.Mutex
	heldConns := make([]*pgxpool.Conn, 0, 100)

	for i := 0; i < 100; i++ {
		holders.Add(1)
		go func() {
			defer holders.Done()
			conn, acquireErr := pool.Acquire(context.Background())
			if acquireErr == nil {
				mu.Lock()
				heldConns = append(heldConns, conn)
				mu.Unlock()
				<-holdCh
				conn.Release()
			}
		}()
	}
	time.Sleep(1 * time.Second)
	t.Logf("Pool exhaustion state: %d connections held, MaxConns=%d",
		len(heldConns), cfg.MaxConns)

	shortCtx, cancelShort := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShort()
	_, queryErr := pool.Exec(shortCtx, "SELECT 1 /* chaos: simulated activity call */")

	if queryErr != nil {
		t.Logf("Retryable error confirmed under pool exhaustion: %v", queryErr)
	} else {
		t.Logf("WARNING: activity call succeeded despite exhaustion attempt "+
			"(held=%d, MaxConns=%d) — consider lowering MaxConns",
			len(heldConns), cfg.MaxConns)
	}

	close(holdCh)
	holders.Wait()
	t.Logf("All %d held connections released", len(heldConns))

	recoverDeadline := time.Now().Add(5 * time.Second)
	var recoverErr error
	for time.Now().Before(recoverDeadline) {
		recCtx, recCancel := context.WithTimeout(context.Background(), 2*time.Second)
		var val int
		recoverErr = pool.QueryRow(recCtx,
			"SELECT 1 /* chaos: activity recovery verification */").Scan(&val)
		recCancel()
		if recoverErr == nil && val == 1 {
			t.Logf("Activity eventually succeeded after pool exhaustion lifted "+
				"(SELECT 1 = %d)", val)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Activity did not eventually succeed within 5s after connections "+
		"released: last error: %v", recoverErr)
}
