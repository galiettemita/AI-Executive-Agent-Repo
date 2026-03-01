package determinism

import (
	"crypto/sha256"
	"time"
)

// DeterministicJitterSeconds returns a stable offset in [0, 50] seconds for a
// given workspace/job pair to avoid synchronized cron thundering herds.
func DeterministicJitterSeconds(workspaceID, jobName string) int {
	sum := sha256.Sum256([]byte(workspaceID + "||" + jobName))
	return int(sum[0]) % 51
}

func ApplyDeterministicJitter(base time.Time, workspaceID, jobName string) time.Time {
	return base.Add(time.Duration(DeterministicJitterSeconds(workspaceID, jobName)) * time.Second)
}
