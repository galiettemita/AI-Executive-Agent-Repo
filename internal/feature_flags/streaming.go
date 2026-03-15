package feature_flags

import "os"

// StreamingEnabled returns true when SSE streaming is active for synthesis output.
// Set FEATURE_STREAMING_ENABLED=true in the environment to enable.
// When false (default), the existing non-streaming synthesis path is used.
func StreamingEnabled() bool {
	return os.Getenv("FEATURE_STREAMING_ENABLED") == "true"
}
