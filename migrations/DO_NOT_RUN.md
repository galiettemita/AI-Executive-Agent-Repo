# LEGACY - Do not run these migrations

This directory contains the original up/down migration set from an early
prototype of the Brevio schema. It is kept for historical reference only.

These files must not be executed against any database.

## Canonical migrations

All production migrations live in db/migrations/.

The migration runner (internal/database/migrations.go) and all Makefile
targets reference db/migrations/ exclusively. This is documented in
DECISIONS.md as decision D6.

## Why this directory still exists

The migrations/ schema uses different table names and a different design
than the current db/migrations/ schema. It is preserved so that the design
history is not lost. If you need to port a table definition from here, do so
manually into a new db/migrations/NNN_BREVIO_*.sql file.
