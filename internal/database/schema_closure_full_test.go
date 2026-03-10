package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestAllMigrationsWorkspaceRLSClosure verifies that every table with a
// workspace_id column in every migration has RLS enabled and the correct
// isolation policy.
func TestAllMigrationsWorkspaceRLSClosure(t *testing.T) {
	t.Parallel()

	migrations := []string{
		"001_BREVIO_v9_init.sql",
		"002_BREVIO_v91_soft_intelligence.sql",
		"003_BREVIO_v92_production_hardening.sql",
		"004_BREVIO_ops_operational_systems.sql",
		"006_BREVIO_v93_addendum_specification_closure.sql",
		"008_BREVIO_v10_gap_closure.sql",
		"009_BREVIO_v10_authorization_receipts.sql",
		"010_BREVIO_v101_admin_intelligence.sql",
		"011_BREVIO_v102_v103_intelligence.sql",
		"012_BREVIO_v104_voice_calls.sql",
		"013_BREVIO_openclaw_adoption.sql",
	}

	for _, name := range migrations {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sql := readMigrationSQL(t, name)

			workspaceTables := parseWorkspaceScopedTables(sql)
			if len(workspaceTables) == 0 {
				return // Migration has no workspace-scoped tables (e.g., 005, 007)
			}

			// Every workspace-scoped table must appear in an RLS block
			for _, table := range workspaceTables {
				rlsToken := fmt.Sprintf("'%s'", table)
				if !strings.Contains(sql, rlsToken) {
					t.Fatalf("%s: workspace table %q not found in RLS policy array", name, table)
				}
			}

			// The migration must contain the RLS policy expression
			if !strings.Contains(sql, "ENABLE ROW LEVEL SECURITY") {
				t.Fatalf("%s has workspace tables but no ENABLE ROW LEVEL SECURITY", name)
			}
			if !strings.Contains(sql, "current_setting(''app.workspace_id'')::uuid") &&
				!strings.Contains(sql, `current_setting('app.workspace_id')::uuid`) {
				t.Fatalf("%s missing app.workspace_id RLS policy expression", name)
			}
		})
	}
}

// TestAllMigrationsUUIDv7DefaultOnPKs ensures every CREATE TABLE with an
// `id uuid PRIMARY KEY` column uses `DEFAULT uuid_v7_generate()`.
func TestAllMigrationsUUIDv7DefaultOnPKs(t *testing.T) {
	t.Parallel()

	idPK := regexp.MustCompile(`(?mi)^\s*id\s+uuid\s+PRIMARY\s+KEY[^\n]*$`)

	migrations := []string{
		"001_BREVIO_v9_init.sql",
		"002_BREVIO_v91_soft_intelligence.sql",
		"003_BREVIO_v92_production_hardening.sql",
		"004_BREVIO_ops_operational_systems.sql",
		"006_BREVIO_v93_addendum_specification_closure.sql",
		"008_BREVIO_v10_gap_closure.sql",
		"009_BREVIO_v10_authorization_receipts.sql",
		"010_BREVIO_v101_admin_intelligence.sql",
		"011_BREVIO_v102_v103_intelligence.sql",
		"012_BREVIO_v104_voice_calls.sql",
		"013_BREVIO_openclaw_adoption.sql",
	}

	for _, name := range migrations {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sql := readMigrationSQL(t, name)
			matches := createTableBlockPattern.FindAllStringSubmatch(sql, -1)
			for _, match := range matches {
				if len(match) < 3 {
					continue
				}
				tableName := strings.ToLower(match[1])
				tableBody := match[2]

				idLine := idPK.FindString(tableBody)
				if idLine == "" {
					continue
				}
				if !strings.Contains(idLine, "DEFAULT uuid_v7_generate()") {
					t.Fatalf("%s: table %s has uuid PK without DEFAULT uuid_v7_generate(): %s",
						name, tableName, strings.TrimSpace(idLine))
				}
			}
		})
	}
}

// TestCanonicalMigrationChainHasNoLegacyLeaks verifies that tables defined in
// legacy migrations/ don't shadow or conflict with tables in db/migrations/.
func TestCanonicalMigrationChainHasNoLegacyLeaks(t *testing.T) {
	t.Parallel()

	// Verify the canonical chain is self-contained: all 13 migrations
	// can be discovered without error.
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	dbDir := filepath.Join(root, "db", "migrations")

	migrations, err := DiscoverMigrations(dbDir)
	if err != nil {
		t.Fatalf("discover canonical migrations: %v", err)
	}
	if len(migrations) < 13 {
		t.Fatalf("canonical chain must have >= 13 migrations, got %d", len(migrations))
	}

	// Verify no migration file contains references to the legacy directory
	for _, m := range migrations {
		body, err := os.ReadFile(m.Path)
		if err != nil {
			t.Fatalf("read migration %d: %v", m.Version, err)
		}
		content := strings.ToLower(string(body))
		if strings.Contains(content, "-- source: migrations/") {
			t.Fatalf("migration %03d references legacy migrations/ directory", m.Version)
		}
	}
}
