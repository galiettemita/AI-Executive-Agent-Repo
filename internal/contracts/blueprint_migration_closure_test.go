package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBlueprintMigrationPairsComplete(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationsDir := filepath.Join(root, "migrations")

	for i := 12; i <= 51; i++ {
		upFile := findMigrationFile(t, migrationsDir, i, "up")
		downFile := findMigrationFile(t, migrationsDir, i, "down")

		if upFile == "" {
			t.Fatalf("missing up migration for %03d", i)
		}
		if downFile == "" {
			t.Fatalf("missing down migration for %03d", i)
		}
	}
}

func TestBlueprintMigrationUpSQLValid(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationsDir := filepath.Join(root, "migrations")

	for i := 12; i <= 51; i++ {
		upFile := findMigrationFile(t, migrationsDir, i, "up")
		if upFile == "" {
			continue
		}

		body := readFileString(t, filepath.Join(migrationsDir, upFile))

		t.Run(upFile, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(body, "BEGIN;") {
				t.Fatalf("%s missing BEGIN; transaction wrapper", upFile)
			}
			if !strings.Contains(body, "COMMIT;") {
				t.Fatalf("%s missing COMMIT; transaction wrapper", upFile)
			}
		})
	}
}

func TestBlueprintMigrationDownSQLValid(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationsDir := filepath.Join(root, "migrations")

	for i := 12; i <= 51; i++ {
		downFile := findMigrationFile(t, migrationsDir, i, "down")
		if downFile == "" {
			continue
		}

		body := readFileString(t, filepath.Join(migrationsDir, downFile))

		t.Run(downFile, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(body, "BEGIN;") {
				t.Fatalf("%s missing BEGIN; transaction wrapper", downFile)
			}
			if !strings.Contains(body, "COMMIT;") {
				t.Fatalf("%s missing COMMIT; transaction wrapper", downFile)
			}
		})
	}
}

func TestBlueprintMigrationRLSConsistency(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationsDir := filepath.Join(root, "migrations")

	isServiceOrAdmin := regexp.MustCompile(`is_service_or_admin\(\)`)
	enableRLS := regexp.MustCompile(`ENABLE ROW LEVEL SECURITY`)

	for i := 12; i <= 50; i++ {
		upFile := findMigrationFile(t, migrationsDir, i, "up")
		if upFile == "" {
			continue
		}

		body := readFileString(t, filepath.Join(migrationsDir, upFile))

		t.Run(upFile, func(t *testing.T) {
			t.Parallel()

			if !enableRLS.MatchString(body) {
				return
			}

			if !isServiceOrAdmin.MatchString(body) {
				t.Fatalf("%s has RLS enabled but missing is_service_or_admin() — inconsistent pattern", upFile)
			}
		})
	}
}

func TestBlueprintMigrationDownDropsTable(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationsDir := filepath.Join(root, "migrations")

	createTableRe := regexp.MustCompile(`CREATE TABLE\s+(?:IF NOT EXISTS\s+)?(\S+)`)
	dropTableRe := regexp.MustCompile(`DROP TABLE\s+(?:IF EXISTS\s+)?(\S+)`)

	for i := 12; i <= 50; i++ {
		upFile := findMigrationFile(t, migrationsDir, i, "up")
		downFile := findMigrationFile(t, migrationsDir, i, "down")
		if upFile == "" || downFile == "" {
			continue
		}

		upBody := readFileString(t, filepath.Join(migrationsDir, upFile))
		downBody := readFileString(t, filepath.Join(migrationsDir, downFile))

		t.Run(upFile, func(t *testing.T) {
			t.Parallel()

			creates := createTableRe.FindAllStringSubmatch(upBody, -1)
			if len(creates) == 0 {
				return
			}

			for _, match := range creates {
				tableName := strings.TrimSuffix(match[1], "(")
				drops := dropTableRe.FindAllStringSubmatch(downBody, -1)
				found := false
				for _, drop := range drops {
					dropTable := strings.TrimSuffix(drop[1], ";")
					if dropTable == tableName {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("down migration for %s does not DROP TABLE %s", upFile, tableName)
				}
			}
		})
	}
}

func TestBlueprintSeedSkillCount(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	sqlPath := filepath.Join(root, "migrations", "051_seed_blueprint_skills.up.sql")
	body := readFileString(t, sqlPath)

	idPattern := regexp.MustCompile(`'([a-z]+\.[a-z_]+)'`)
	matches := idPattern.FindAllStringSubmatch(body, -1)

	seen := make(map[string]struct{})
	for _, m := range matches {
		seen[m[1]] = struct{}{}
	}

	if len(seen) != 42 {
		t.Fatalf("expected 42 unique skill IDs in seed, got %d", len(seen))
	}
}

func TestBlueprintSeedCategories(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	sqlPath := filepath.Join(root, "migrations", "051_seed_blueprint_skills.up.sql")
	body := readFileString(t, sqlPath)

	requiredCategories := []string{"browser", "marketing", "agents", "memory", "routing", "cron"}
	for _, cat := range requiredCategories {
		if !strings.Contains(body, "'"+cat+"'") {
			t.Fatalf("seed migration missing category: %s", cat)
		}
	}
}

func TestBlueprintSeedDownDeletesAll(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	upPath := filepath.Join(root, "migrations", "051_seed_blueprint_skills.up.sql")
	downPath := filepath.Join(root, "migrations", "051_seed_blueprint_skills.down.sql")
	upBody := readFileString(t, upPath)
	downBody := readFileString(t, downPath)

	idPattern := regexp.MustCompile(`'([a-z]+\.[a-z_]+)'`)
	upMatches := idPattern.FindAllStringSubmatch(upBody, -1)

	upIDs := make(map[string]struct{})
	for _, m := range upMatches {
		upIDs[m[1]] = struct{}{}
	}

	downMatches := idPattern.FindAllStringSubmatch(downBody, -1)
	downIDs := make(map[string]struct{})
	for _, m := range downMatches {
		downIDs[m[1]] = struct{}{}
	}

	for id := range upIDs {
		if _, ok := downIDs[id]; !ok {
			t.Fatalf("skill %s is in up migration but not in down migration DELETE", id)
		}
	}
}

func findMigrationFile(t *testing.T, dir string, num int, direction string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	numStr := fmt.Sprintf("%03d", num)
	suffix := "." + direction + ".sql"

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), numStr+"_") && strings.HasSuffix(entry.Name(), suffix) {
			return entry.Name()
		}
	}
	return ""
}
