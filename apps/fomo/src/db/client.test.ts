import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { closeDbClient, loadDbClient } from './client.ts';

describe('loadDbClient — env-driven selection', () => {
  it('returns no_database_url_dev_mode when DATABASE_URL is missing and not in production', () => {
    const result = loadDbClient({ env: {} });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.reason, 'no_database_url_dev_mode');
    }
  });

  it('returns no_database_url_dev_mode in default (no NODE_ENV) when DATABASE_URL missing', () => {
    const result = loadDbClient({ env: { NODE_ENV: 'development' } });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.reason, 'no_database_url_dev_mode');
    }
  });

  it('returns in_memory_dev_mode when DATABASE_URL missing but BREVIO_DEV_MODE=true', () => {
    const result = loadDbClient({ env: { BREVIO_DEV_MODE: 'true' } });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.reason, 'in_memory_dev_mode');
    }
  });
});

describe('loadDbClient — fail-closed in production', () => {
  it('THROWS when NODE_ENV=production and DATABASE_URL is missing (no dev mode escape)', () => {
    assert.throws(
      () => loadDbClient({ env: { NODE_ENV: 'production' } }),
      /DATABASE_URL required in production/
    );
  });

  it('THROWS in production with empty DATABASE_URL', () => {
    assert.throws(
      () => loadDbClient({ env: { NODE_ENV: 'production', DATABASE_URL: '' } }),
      /DATABASE_URL required in production/
    );
  });

  it('THROWS in production with whitespace-only DATABASE_URL', () => {
    assert.throws(
      () => loadDbClient({ env: { NODE_ENV: 'production', DATABASE_URL: '   ' } }),
      /DATABASE_URL required in production/
    );
  });

  it('does NOT throw in production when BREVIO_DEV_MODE=true (explicit dev escape hatch)', () => {
    const result = loadDbClient({
      env: { NODE_ENV: 'production', BREVIO_DEV_MODE: 'true' }
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.reason, 'in_memory_dev_mode');
    }
  });

  it('the production-throw message mentions BREVIO_DEV_MODE escape hatch', () => {
    try {
      loadDbClient({ env: { NODE_ENV: 'production' } });
      assert.fail('expected throw');
    } catch (err) {
      assert.ok(err instanceof Error);
      assert.match(err.message, /BREVIO_DEV_MODE=true/);
      assert.match(err.message, /in-memory.*dev only/i);
    }
  });
});

describe('loadDbClient — returns a Drizzle client when DATABASE_URL is set', () => {
  // We never connect during loadDbClient; pg.Pool is lazy. So even a bogus
  // URL gives us a real result that doesn't touch the network. Tests that
  // actually exercise the Postgres-backed stores are gated behind
  // BREVIO_RUN_PG_TESTS and live alongside each store.

  it('returns ok with a Drizzle client and a pg.Pool', async () => {
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://nobody:nothing@localhost:5432/nowhere_db' },
      // Keep idleTimeoutMs short so the pool is easy to close cleanly.
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100
    });
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.ok(result.client);
      assert.ok(result.pool);
      // Close the pool so node:test does not hang on the open connection
      // handle. Without this, the test process keeps the libpq socket alive.
      await closeDbClient(result);
    }
  });

  it('result is frozen', () => {
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/z' },
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100
    });
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = false;
    });
    if (result.ok) {
      void closeDbClient(result);
    }
  });
});

describe('closeDbClient', () => {
  it('is a no-op when the result is in-memory', async () => {
    const result = loadDbClient({ env: { BREVIO_DEV_MODE: 'true' } });
    await closeDbClient(result); // should not throw
  });
});
