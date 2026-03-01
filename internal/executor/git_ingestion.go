package executor

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const MaxRepositorySizeBytes int64 = 500 * 1024 * 1024

type RepositoryProfile struct {
	TotalFiles  int
	Languages   []string
	TechStack   []string
	Conventions []string
}

func ValidateRepositorySize(sizeBytes int64) error {
	if sizeBytes > MaxRepositorySizeBytes {
		return fmt.Errorf("BREVIO.codebase.size_exceeded.v1")
	}
	return nil
}

func BuildShallowCloneCommand(remoteURL, defaultBranch string) ([]string, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(remoteURL)), "https://") {
		return nil, fmt.Errorf("only https remotes supported")
	}
	branch := strings.TrimSpace(defaultBranch)
	if branch == "" {
		branch = "main"
	}
	return []string{"git", "clone", "--depth", "1", "--single-branch", "--branch", branch, remoteURL}, nil
}

func ShouldRetryClone(statusCode int) bool {
	return statusCode >= 500 || statusCode == 429 || statusCode == 0
}

func AnalyzeRepositoryFileTree(paths []string) RepositoryProfile {
	languages := map[string]struct{}{}
	tech := map[string]struct{}{}
	conventions := map[string]struct{}{}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			languages["go"] = struct{}{}
		case ".py":
			languages["python"] = struct{}{}
		case ".ts", ".tsx", ".js":
			languages["typescript_javascript"] = struct{}{}
		case ".tf":
			languages["terraform"] = struct{}{}
		}
		base := strings.ToLower(filepath.Base(path))
		switch base {
		case "dockerfile":
			tech["docker"] = struct{}{}
		case "go.mod":
			tech["go_modules"] = struct{}{}
		case "package.json":
			tech["nodejs"] = struct{}{}
		case ".github":
			conventions["github_workflows"] = struct{}{}
		}
		if strings.Contains(strings.ToLower(path), ".github/workflows/") {
			conventions["github_actions"] = struct{}{}
		}
	}

	return RepositoryProfile{
		TotalFiles:  len(paths),
		Languages:   sortedKeys(languages),
		TechStack:   sortedKeys(tech),
		Conventions: sortedKeys(conventions),
	}
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
