package connectors

import "strings"

// IsPlaceholderMCPURL reports whether a URL is unsuitable for production use.
// Blocks: empty, unconfigured.local, localhost, 127.x.x.x, 0.0.0.0.
func IsPlaceholderMCPURL(rawURL string) bool {
	if strings.TrimSpace(rawURL) == "" {
		return true
	}
	lower := strings.ToLower(rawURL)

	if strings.Contains(lower, "unconfigured.local") {
		return true
	}
	if strings.Contains(lower, "localhost") {
		return true
	}
	if strings.Contains(lower, "[::1]") {
		return true
	}
	if strings.Contains(lower, "://127.") {
		return true
	}
	if strings.Contains(lower, "://0.0.0.0") {
		return true
	}
	return false
}
