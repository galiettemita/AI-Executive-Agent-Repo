#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

resolve_docker_bin() {
  if [[ -x "/Applications/Docker.app/Contents/Resources/bin/docker" ]]; then
    echo "/Applications/Docker.app/Contents/Resources/bin/docker"
    return 0
  fi
  if command -v docker >/dev/null 2>&1; then
    command -v docker
    return 0
  fi
  return 1
}

docker_bin="$(resolve_docker_bin || true)"
if [[ -z "${docker_bin}" ]]; then
  echo "[db-verify] docker is required but not available"
  exit 1
fi

container_name="brevio_pg_verify_${RANDOM}${RANDOM}"
postgres_image="${BREVIO_DB_VERIFY_IMAGE:-pgvector/pgvector:pg16}"
cleanup() {
  "$docker_bin" rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[db-verify] starting ${postgres_image} container ${container_name}"
"$docker_bin" run -d \
  --name "$container_name" \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=brevio \
  -v "$ROOT_DIR":/src \
  "$postgres_image" >/dev/null

echo "[db-verify] waiting for postgres readiness"
ready=0
for _ in $(seq 1 60); do
  if "$docker_bin" exec "$container_name" pg_isready -U postgres -d brevio >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != "1" ]]; then
  echo "[db-verify] postgres did not become ready"
  exit 1
fi

run_psql() {
  "$docker_bin" exec -i "$container_name" psql -v ON_ERROR_STOP=1 -U postgres -d brevio "$@"
}

echo "[db-verify] applying migrations 001 -> 003"
run_psql -f /src/db/migrations/001_BREVIO_v9_init.sql >/dev/null
run_psql -f /src/db/migrations/002_BREVIO_v91_soft_intelligence.sql >/dev/null
run_psql -f /src/db/migrations/003_BREVIO_v92_production_hardening.sql >/dev/null

echo "[db-verify] creating non-superuser validation role"
run_psql <<SQL >/dev/null
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'brevio_app') THEN
    CREATE ROLE brevio_app LOGIN PASSWORD 'brevio_app';
  END IF;
END
\$\$;
GRANT USAGE ON SCHEMA public TO brevio_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO brevio_app;
SQL

echo "[db-verify] checking enum count"
enum_count="$(
  run_psql -t -A -c \
    "SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typtype='e';" \
    | tr -d '[:space:]'
)"
if [[ "$enum_count" != "82" ]]; then
  echo "[db-verify] enum count mismatch: got=${enum_count} want=82"
  exit 1
fi

echo "[db-verify] checking workspace RLS coverage"
missing_rls_count="$(
  run_psql -t -A -c \
    "WITH workspace_tables AS (
       SELECT c.oid, c.relname
       FROM pg_class c
       JOIN pg_namespace n ON n.oid = c.relnamespace
       JOIN pg_attribute a ON a.attrelid = c.oid
       WHERE n.nspname='public' AND c.relkind='r' AND a.attname='workspace_id' AND a.attnum > 0 AND NOT a.attisdropped
     )
     SELECT count(*)
     FROM workspace_tables wt
     JOIN pg_class c ON c.oid = wt.oid
     WHERE NOT c.relrowsecurity;" \
    | tr -d '[:space:]'
)"
if [[ "$missing_rls_count" != "0" ]]; then
  echo "[db-verify] RLS coverage failure: ${missing_rls_count} workspace tables without RLS"
  exit 1
fi

echo "[db-verify] checking app.workspace_id unset guard"
if run_psql -c "SET ROLE brevio_app; SELECT current_setting('app.workspace_id')::uuid;" >/dev/null 2>&1; then
  echo "[db-verify] expected current_setting('app.workspace_id') to fail when unset"
  exit 1
fi

workspace_a="018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"
workspace_b="018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d30"
account_id="018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d31"
user_a="018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d32"
user_b="018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d33"

echo "[db-verify] seeding minimal account/user/workspace rows"
run_psql <<SQL >/dev/null
SET ROLE brevio_app;
INSERT INTO accounts (id, plan_key) VALUES ('${account_id}', 'free');
INSERT INTO users (id, account_id, email) VALUES
  ('${user_a}', '${account_id}', 'ws_a@example.com'),
  ('${user_b}', '${account_id}', 'ws_b@example.com');
INSERT INTO workspaces (id, account_id, owner_user_id, memory_namespace) VALUES
  ('${workspace_a}', '${account_id}', '${user_a}', 'ws_a'),
  ('${workspace_b}', '${account_id}', '${user_b}', 'ws_b');
SQL

echo "[db-verify] inserting workspace-scoped row under workspace A"
run_psql <<SQL >/dev/null
SET ROLE brevio_app;
SET app.workspace_id = '${workspace_a}';
INSERT INTO goal_items (workspace_id, title, horizon, status, priority)
VALUES ('${workspace_a}', 'phase1-step2-rls-check', 'weekly', 'active', 'medium');
SQL

count_b="$(
  run_psql -t -A -c "SET ROLE brevio_app; SET app.workspace_id='${workspace_b}'; SELECT count(*) FROM goal_items;" \
    | tail -n1 | tr -d '[:space:]'
)"
count_a="$(
  run_psql -t -A -c "SET ROLE brevio_app; SET app.workspace_id='${workspace_a}'; SELECT count(*) FROM goal_items;" \
    | tail -n1 | tr -d '[:space:]'
)"

if [[ "$count_b" != "0" ]]; then
  echo "[db-verify] cross-workspace isolation failed: workspace B saw ${count_b} rows"
  exit 1
fi
if [[ "$count_a" != "1" ]]; then
  echo "[db-verify] workspace A expected 1 row, got ${count_a}"
  exit 1
fi

echo "[db-verify] success: migrations apply cleanly with enum/RLS/isolation checks"
