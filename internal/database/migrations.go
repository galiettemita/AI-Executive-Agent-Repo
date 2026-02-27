package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Migration struct {
	Version int
	Name    string
	Path    string
}

var migrationPattern = regexp.MustCompile(`^(\d+)_([a-zA-Z0-9_\-]+)\.sql$`)

func DiscoverMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.Contains(strings.ToLower(name), "down") {
			return nil, fmt.Errorf("down migration is not allowed: %s", name)
		}

		matches := migrationPattern.FindStringSubmatch(name)
		if len(matches) != 3 {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version for %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    matches[2],
			Path:    filepath.Join(dir, name),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			return nil, fmt.Errorf("migration versions must be strictly increasing: %d then %d", migrations[i-1].Version, migrations[i].Version)
		}
	}

	return migrations, nil
}
