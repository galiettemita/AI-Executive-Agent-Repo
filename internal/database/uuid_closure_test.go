package database

import (
	"regexp"
	"strings"
	"testing"
)

var idPrimaryKeyPattern = regexp.MustCompile(`(?mi)^\s*id\s+uuid\s+PRIMARY\s+KEY[^\n]*$`)

func TestMigrationUUIDv7DefaultClosure(t *testing.T) {
	t.Parallel()

	migrations := []string{
		"001_BREVIO_v9_init.sql",
		"002_BREVIO_v91_soft_intelligence.sql",
		"003_BREVIO_v92_production_hardening.sql",
		"004_BREVIO_ops_operational_systems.sql",
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

				idLine := idPrimaryKeyPattern.FindString(tableBody)
				if idLine == "" {
					continue
				}
				if !strings.Contains(idLine, "DEFAULT uuid_v7_generate()") {
					t.Fatalf("%s table %s has uuid primary key without uuid_v7 default: %s", name, tableName, strings.TrimSpace(idLine))
				}
			}
		})
	}
}
