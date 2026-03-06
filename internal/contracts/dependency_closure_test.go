package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyVersionClosure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repositoryRoot(t), "go.mod")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(body)

	requiredTokens := []string{
		"go 1.23",
		"github.com/jackc/pgx/v5 v5.7.4",
		"golang.org/x/crypto v0.33.0",
		"golang.org/x/sync v0.11.0",
		"golang.org/x/text v0.22.0",
	}
	for _, token := range requiredTokens {
		if !strings.Contains(content, token) {
			t.Fatalf("go.mod missing dependency/version token: %s", token)
		}
	}
}
