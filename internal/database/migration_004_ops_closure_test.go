package database

import (
	"strings"
	"testing"
)

func TestMigration004OperationalSystemsClosure(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "004_BREVIO_ops_operational_systems.sql")

	gotTables := parseIdentifiers(sql, createTableNamePattern)
	expectedTables := []string{
		"analytics_daily",
		"analytics_events",
		"eval_results",
		"invoices",
		"moderation_queue",
		"scheduled_notifications",
		"subscriptions",
		"user_feedback",
	}
	assertExactNameSet(t, "migration 004 tables", gotTables, expectedTables)
	assertWorkspaceRLSCoverage(t, sql, "migration 004")

	rlsPos := strings.Index(sql, "ENABLE ROW LEVEL SECURITY")
	indexPos := firstIndexTokenPosition(sql)
	if rlsPos < 0 {
		t.Fatal("migration 004 missing RLS statements")
	}
	if indexPos < 0 {
		t.Fatal("migration 004 missing indexes")
	}
	if rlsPos > indexPos {
		t.Fatal("migration 004 violates Rule Z ordering: indexes must follow RLS")
	}
}

func TestOperationalSystemsTablesIncludePromptVersions(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "001_BREVIO_v9_init.sql")
	if !strings.Contains(sql, "CREATE TABLE prompt_versions") {
		t.Fatal("migration 001 missing prompt_versions table required by operational systems blueprint")
	}
}
