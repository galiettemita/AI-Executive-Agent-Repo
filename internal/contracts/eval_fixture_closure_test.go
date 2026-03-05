package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDeterminismFixtureHashes(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "evals", "determinism_fixtures.json")
	assertFileNonEmpty(t, path)

	type fixture struct {
		ID           string `json:"id"`
		Input        string `json:"input"`
		ExpectedHash string `json:"expected_hash"`
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unable to read determinism fixtures: %v", err)
	}
	var fixtures []fixture
	if err := json.Unmarshal(blob, &fixtures); err != nil {
		t.Fatalf("unable to parse determinism fixtures: %v", err)
	}
	if len(fixtures) < 3 {
		t.Fatalf("expected >=3 determinism fixtures, got %d", len(fixtures))
	}

	hashPattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	for _, item := range fixtures {
		if strings.TrimSpace(item.ID) == "" {
			t.Fatalf("fixture has empty id: %+v", item)
		}
		if strings.TrimSpace(item.Input) == "" {
			t.Fatalf("fixture %s has empty input", item.ID)
		}
		if !hashPattern.MatchString(item.ExpectedHash) {
			t.Fatalf("fixture %s expected_hash must be 64-char lowercase hex, got %q", item.ID, item.ExpectedHash)
		}
		sum := sha256.Sum256([]byte(item.Input))
		expected := hex.EncodeToString(sum[:])
		if expected != item.ExpectedHash {
			t.Fatalf("fixture %s hash mismatch: got %s want %s", item.ID, item.ExpectedHash, expected)
		}
	}
}

func TestRAGEvalFrameworkDocIsOperational(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "evals", "rag_eval_framework.md")
	assertFileNonEmpty(t, path)
	assertFileContainsTokens(t, path, []string{
		"# RAG Eval Framework",
		"faithfulness",
		"relevance",
		"pass",
		"0.80",
		"0.75",
		"Failure Handling",
	})
}
