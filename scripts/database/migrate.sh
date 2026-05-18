#!/usr/bin/env bash
# Forward-only migration runner for BREVIO production.
# Applies db/migrations/*.sql in version order against a PostgreSQL database.
#
# Usage:
#   DATABASE_URL=postgres://... bash scripts/database/migrate.sh
#
# This script:
#   1. Discovers all SQL files in db/migrations/ using version ordering
#   2. Creates a schema_migrations tracking table if absent
#   3. Applies only unapplied migrations in strictly increasing order
#   4. Fails on any error (no partial migration state)
#   5. Rejects any file with "down" in the filename
#
# Per DECISIONS.md D6: db/migrations/ is the only production migration chain.
# Legacy migrations/ directory is quarantined as pre-v9 schema.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATION_DIR="${ROOT_DIR}/db/migrations"

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "[migrate] ERROR: DATABASE_URL is required"
  exit 1
fi

if [[ ! -d "${MIGRATION_DIR}" ]]; then
  echo "[migrate] ERROR: migration directory not found: ${MIGRATION_DIR}"
  exit 1
fi

run_psql() {
  psql -v ON_ERROR_STOP=1 "${DATABASE_URL}" "$@"
}

echo "[migrate] ensuring schema_migrations tracking table exists"
run_psql <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL

echo "[migrate] discovering migrations in db/migrations/"
applied_versions="$(run_psql -t -A -c "SELECT version FROM schema_migrations ORDER BY version;")"

migration_files=()
while IFS= read -r -d '' file; do
  migration_files+=("$file")
done < <(find "${MIGRATION_DIR}" -maxdepth 1 -name '*.sql' -print0 | sort -z)

applied=0
skipped=0

for file in "${migration_files[@]}"; do
  filename="$(basename "$file")"

  # Reject down migrations
  if echo "$filename" | grep -qi "down"; then
    echo "[migrate] ERROR: down migration detected and rejected: ${filename}"
    exit 1
  fi

  # Extract version number
  version="$(echo "$filename" | grep -oE '^[0-9]+' | sed 's/^0*//')"
  if [[ -z "$version" ]]; then
    echo "[migrate] WARN: skipping non-versioned file: ${filename}"
    continue
  fi

  # Check if already applied
  if echo "$applied_versions" | grep -qx "$version"; then
    skipped=$((skipped + 1))
    continue
  fi

  echo "[migrate] applying ${filename} (version ${version})"
  run_psql -f "$file"
  run_psql -c "INSERT INTO schema_migrations (version, name) VALUES (${version}, '${filename}');"
  applied=$((applied + 1))
done

echo "[migrate] complete: ${applied} applied, ${skipped} already applied"
