//go:build chaos

package chaos

import (
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestChaos_LoadShedding200Concurrent(t *testing.T) {
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8080"
	}
	t.Logf("Load shedding test: 200 concurrent requests → %s/healthz/ready", gatewayURL)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	var wg sync.WaitGroup
	latencies := make([]int64, 200)
	errors := int64(0)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := time.Now()
			resp, err := httpClient.Get(gatewayURL + "/healthz/ready")
			latencies[i] = time.Since(start).Milliseconds()
			if err != nil || resp.StatusCode != 200 {
				atomic.AddInt64(&errors, 1)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()

	sort.Slice(latencies, func(a, b int) bool { return latencies[a] < latencies[b] })
	p99 := latencies[198]

	t.Logf("Load test complete — P50=%dms P95=%dms P99=%dms errors=%d/200",
		latencies[99], latencies[189], p99, errors)

	if p99 > 5000 {
		t.Errorf("P99 %dms exceeds 5000ms SLO", p99)
	}
	if errors > 10 {
		t.Errorf(">10 failures in 200 concurrent requests")
	}
	if p99 <= 5000 && errors <= 10 {
		t.Logf("SLO PASS — P99=%dms (≤5000ms), errors=%d/200 (>95%% success rate)",
			p99, errors)
	}
}
