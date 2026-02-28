package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCanonicalEventAndMetricNaming(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	eventFiles := []string{
		filepath.Join(root, "spec", "events", "canonical_events_v9.txt"),
		filepath.Join(root, "spec", "events", "canonical_events_v91.txt"),
		filepath.Join(root, "spec", "events", "canonical_events_v92.txt"),
	}

	eventPattern := regexp.MustCompile(`^BREVIO\.[a-z0-9_]+(?:\.[a-z0-9_]+)+\.v1$`)
	for _, path := range eventFiles {
		entries := readCanonicalListFile(t, path)
		for _, eventName := range entries {
			if !eventPattern.MatchString(eventName) {
				t.Fatalf("event name violates canonical format in %s: %s", path, eventName)
			}
		}
	}

	metricFile := filepath.Join(root, "spec", "metrics", "canonical_metrics_v92.txt")
	metricPattern := regexp.MustCompile(`^BREVIO_[a-z0-9_]+$`)
	for _, metricName := range readCanonicalListFile(t, metricFile) {
		if !metricPattern.MatchString(metricName) {
			t.Fatalf("metric name violates canonical format in %s: %s", metricFile, metricName)
		}
	}
}

func readCanonicalListFile(t *testing.T, path string) []string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical list %s: %v", path, err)
	}

	out := make([]string, 0)
	for _, line := range strings.Split(string(body), "\n") {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		out = append(out, value)
	}
	return out
}
