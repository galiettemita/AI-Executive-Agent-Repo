// drizzle-kit configuration. Run `pnpm exec drizzle-kit generate` to produce
// SQL migrations from apps/fomo/src/db/schema.ts. Migrations land in
// apps/fomo/src/db/migrations/ and are committed alongside the schema as the
// single source of truth (Phase 2E replaces the legacy root migrations/
// directory).
//
// DATABASE_URL is consulted only when running drizzle-kit's push/migrate
// commands against a live database — `generate` is a pure schema-diff and
// does not need a connection.

import type { Config } from 'drizzle-kit';

const config: Config = {
  schema: './apps/fomo/src/db/schema.ts',
  out: './apps/fomo/src/db/migrations',
  dialect: 'postgresql',
  dbCredentials: {
    url: process.env.DATABASE_URL ?? 'postgres://localhost:5432/_drizzle_kit_unused'
  },
  strict: true,
  verbose: false
};

export default config;
