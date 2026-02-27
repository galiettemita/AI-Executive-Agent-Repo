package contracts

import (
	"path/filepath"
	"testing"
)

func TestPhase4ReadinessArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	required := []string{
		filepath.Join(root, "evals", "load", "k6_interactive_turn.js"),
		filepath.Join(root, "evals", "load", "README.md"),
		filepath.Join(root, "scripts", "security", "run_security_validation.sh"),
	}
	for _, path := range required {
		assertFileNonEmpty(t, path)
	}
}
