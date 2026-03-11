package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/database"
)

// TestMigrationChainAuthorityOnlyDbMigrations asserts that db/migrations/ is
// the sole production migration chain. The legacy migrations/ directory must
// not be referenced by any production deploy or CI script.
func TestMigrationChainAuthorityOnlyDbMigrations(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	// 1. db/migrations/ must exist and contain at least 13 forward-only migrations.
	dbMigrationsDir := filepath.Join(root, "db", "migrations")
	migrations, err := database.DiscoverMigrations(dbMigrationsDir)
	if err != nil {
		t.Fatalf("discover canonical migrations: %v", err)
	}
	if len(migrations) < 13 {
		t.Fatalf("canonical migration chain must have >= 13 migrations, got %d", len(migrations))
	}

	// 2. Migration runner must use db/migrations exclusively.
	migrateRunner := filepath.Join(root, "scripts", "database", "migrate.sh")
	runnerBody, err := os.ReadFile(migrateRunner)
	if err != nil {
		t.Fatalf("read migration runner: %v", err)
	}
	if !strings.Contains(string(runnerBody), "db/migrations") {
		t.Fatal("migration runner does not reference db/migrations")
	}

	// 3. Production deploy scripts must not reference legacy migrations/ directory
	//    as their migration source.
	productionScripts := []string{
		"scripts/database/migrate.sh",
		"scripts/database/verify_postgres_migrations.sh",
		"scripts/deploy/helm_rollout.sh",
	}
	for _, script := range productionScripts {
		path := filepath.Join(root, script)
		body, err := os.ReadFile(path)
		if err != nil {
			// Script may not exist yet — skip.
			continue
		}
		content := string(body)
		// Check for bare "migrations/" that isn't "db/migrations/"
		lines := strings.Split(content, "\n")
		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue // skip comments
			}
			if strings.Contains(line, "migrations/") &&
				!strings.Contains(line, "db/migrations") &&
				!strings.Contains(line, "schema_migrations") &&
				!strings.Contains(line, "quarantine") {
				t.Fatalf("%s:%d references legacy migrations/ directory instead of db/migrations/: %s",
					script, lineNum+1, trimmed)
			}
		}
	}

	// 4. Verify migration ordering is strictly increasing.
	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			t.Fatalf("migration chain not strictly increasing: version %d follows %d",
				migrations[i].Version, migrations[i-1].Version)
		}
	}
}

// TestMigrationChainForwardOnlyNoDrops scans all canonical migrations for
// forbidden destructive statements.
func TestMigrationChainForwardOnlyNoDrops(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	dbMigrationsDir := filepath.Join(root, "db", "migrations")
	migrations, err := database.DiscoverMigrations(dbMigrationsDir)
	if err != nil {
		t.Fatalf("discover migrations: %v", err)
	}

	forbidden := []string{"DROP TABLE", "DROP TYPE", "TRUNCATE TABLE"}

	for _, m := range migrations {
		body, err := os.ReadFile(m.Path)
		if err != nil {
			t.Fatalf("read migration %d: %v", m.Version, err)
		}
		upper := strings.ToUpper(string(body))
		for _, token := range forbidden {
			if strings.Contains(upper, token) {
				t.Fatalf("migration %03d contains forbidden statement %q — forward-only policy violated", m.Version, token)
			}
		}
	}
}

// TestMigrationChainCompleteness asserts the exact set of expected migration
// versions exists in the canonical chain.
func TestMigrationChainCompleteness(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	dbMigrationsDir := filepath.Join(root, "db", "migrations")
	migrations, err := database.DiscoverMigrations(dbMigrationsDir)
	if err != nil {
		t.Fatalf("discover migrations: %v", err)
	}

	expectedVersions := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
	if len(migrations) != len(expectedVersions) {
		t.Fatalf("migration count mismatch: got=%d want=%d", len(migrations), len(expectedVersions))
	}
	for i, expected := range expectedVersions {
		if migrations[i].Version != expected {
			t.Fatalf("migration version at position %d: got=%d want=%d", i, migrations[i].Version, expected)
		}
	}
}
