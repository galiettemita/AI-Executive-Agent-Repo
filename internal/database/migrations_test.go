package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMigrationsRejectsDown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_ok.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_down.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	_, err := DiscoverMigrations(dir)
	if err == nil {
		t.Fatal("expected down migration rejection")
	}
}

func TestDiscoverMigrationsSorted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "002_b.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_a.sql"), []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	migrations, err := DiscoverMigrations(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("unexpected migration count: %d", len(migrations))
	}
	if migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("unexpected migration order: %+v", migrations)
	}
}
