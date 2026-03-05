package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchedulerServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	schedulerSource := filepath.Join(root, "services", "brevio-scheduler", "src", "index.ts")
	schedulerReadme := filepath.Join(root, "services", "brevio-scheduler", "README.md")

	assertFileContainsTokens(t, schedulerSource, []string{
		"segments[1] === 'jobs'",
		"segments[1] === 'trigger'",
		"segments[1] === 'triggers'",
		"scheduler.job.created",
		"scheduler.trigger.queued",
		"job_limit_exceeded",
		"createSchedulerRuntime",
	})

	assertFileContainsTokens(t, schedulerReadme, []string{
		"Cron and trigger orchestration service",
		"POST /api/v1/scheduler/jobs",
		"POST /api/v1/scheduler/trigger",
		"GET /api/v1/scheduler/triggers",
		"BREVIO_SCHEDULER_MAX_JOBS",
	})

	body, err := os.ReadFile(schedulerReadme)
	if err != nil {
		t.Fatalf("read scheduler readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("scheduler README still contains scaffold marker")
	}
}
