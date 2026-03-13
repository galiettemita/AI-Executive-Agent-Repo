package contracts

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestHandsSkillScaffoldCompletionClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "hands-runtime", "src", "skills")
	overridePath := filepath.Join(root, "config", "skill-manual-overrides.txt")

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read skills directory: %v", err)
	}

	expected := make([]string, 0, len(entries))
	expectedSet := map[string]struct{}{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "_template" {
			continue
		}
		expected = append(expected, name)
		expectedSet[name] = struct{}{}
	}
	sort.Strings(expected)

	overridesRaw, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read manual override file: %v", err)
	}

	overrideLines := strings.Split(string(overridesRaw), "\n")
	overrideSet := map[string]struct{}{}
	for _, line := range overrideLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		overrideSet[trimmed] = struct{}{}
	}

	for _, skillID := range expected {
		if _, ok := overrideSet[skillID]; !ok {
			t.Fatalf("manual override list missing skill %s", skillID)
		}
	}

	for skillID := range overrideSet {
		if _, ok := expectedSet[skillID]; !ok {
			t.Fatalf("manual override contains unknown skill %s", skillID)
		}
	}

	for _, skillID := range expected {
		readmePath := filepath.Join(skillsRoot, skillID, "README.md")
		readmeBody, readErr := os.ReadFile(readmePath)
		if readErr != nil {
			t.Fatalf("read %s README: %v", skillID, readErr)
		}
		if strings.Contains(strings.ToLower(string(readmeBody)), "generated skill adapter scaffold") {
			t.Fatalf("skill %s README still contains scaffold marker", skillID)
		}
	}
}
