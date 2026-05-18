package database

import (
	"strings"
	"testing"
)

func TestMigrationOrderingRuleZ(t *testing.T) {
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
			assertForwardOnlyStatements(t, name, sql)
			assertRuleZOrdering(t, name, sql)
		})
	}
}

func assertForwardOnlyStatements(t *testing.T, migrationName, sql string) {
	t.Helper()

	upper := strings.ToUpper(sql)
	disallowed := []string{
		"DROP TABLE",
		"DROP TYPE",
		"TRUNCATE TABLE",
	}
	for _, token := range disallowed {
		if strings.Contains(upper, token) {
			t.Fatalf("%s contains disallowed forward-only token %q", migrationName, token)
		}
	}
}

func assertRuleZOrdering(t *testing.T, migrationName, sql string) {
	t.Helper()

	enumPos := strings.Index(sql, "CREATE TYPE")
	tablePos := strings.Index(sql, "CREATE TABLE")
	rlsPos := strings.Index(sql, "ENABLE ROW LEVEL SECURITY")
	indexPos := firstIndexTokenPosition(sql)

	if tablePos < 0 {
		t.Fatalf("%s missing table definitions", migrationName)
	}
	if rlsPos < 0 {
		t.Fatalf("%s missing RLS enable statements", migrationName)
	}
	if indexPos < 0 {
		t.Fatalf("%s missing index definitions", migrationName)
	}
	if enumPos >= 0 && enumPos > tablePos {
		t.Fatalf("%s violates Rule Z: enums must be declared before tables", migrationName)
	}
	if tablePos > rlsPos {
		t.Fatalf("%s violates Rule Z: tables must be declared before RLS policies", migrationName)
	}
	if rlsPos > indexPos {
		t.Fatalf("%s violates Rule Z: RLS policies must be declared before indexes", migrationName)
	}
}

func firstIndexTokenPosition(sql string) int {
	positions := []int{
		strings.Index(sql, "CREATE INDEX"),
		strings.Index(sql, "CREATE UNIQUE INDEX"),
	}
	first := -1
	for _, pos := range positions {
		if pos < 0 {
			continue
		}
		if first < 0 || pos < first {
			first = pos
		}
	}
	return first
}
