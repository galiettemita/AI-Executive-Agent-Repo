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

describe('pg pool error handler (Phase v0.5.3 item #3 — Neon ECONNRESET resilience)', () => {
  // Regression for the v0.5.2 incident: pg pool's 'error' event from
  // a transient Neon connection drop went unhandled, crashing the
  // dev server for 18+ hours mid-smoke. Per founder correction #3:
  // log structured sanitized error FIRST (works when DB is down),
  // audit best-effort SECOND (wrapped, never re-throws), NEVER crash.

  it('attaches an error listener so an idle-connection error does NOT cascade to a process crash', () => {
    const captured: string[] = [];
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/z' },
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100,
      stderrWrite: (line) => captured.push(line)
    });
    assert.ok(result.ok);
    if (!result.ok) return;

    // BEFORE v0.5.3 fix: pg pool's 'error' event with no listener
    // = Node unhandled-error → process.exit(1). The mere presence of
    // a listener prevents the crash.
    assert.ok(result.pool.listenerCount('error') > 0, 'pool must have an error listener attached');

    // Manually fire an 'error' event with a synthetic ECONNRESET. If
    // the handler is missing or throws, this test process would die.
    const err = new Error('read ECONNRESET') as NodeJS.ErrnoException;
    err.code = 'ECONNRESET';
    assert.doesNotThrow(() => result.pool.emit('error', err));

    // Structured log to stderr fired with sanitized detail.
    assert.equal(captured.length, 1);
    const logged = JSON.parse(captured[0]);
    assert.equal(logged.event, 'fomo.db.connection_error');
    assert.equal(logged.severity, 'ERROR');
    assert.equal(logged.attrs.error_code, 'ECONNRESET');
    assert.match(logged.attrs.message, /ECONNRESET/);
    // NEVER the connection string.
    assert.equal(captured[0].includes('postgres://x:y@'), false);

    void closeDbClient(result);
  });

  it('onPoolError callback fires after the stderr log; a thrown callback does NOT crash the process', () => {
    const captured: string[] = [];
    let auditCalls = 0;
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/z' },
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100,
      stderrWrite: (line) => captured.push(line),
      onPoolError: () => {
        auditCalls++;
        // Adversarial: a throwing callback must NOT cascade to a crash.
        throw new Error('audit-store unavailable (e.g. DB also down)');
      }
    });
    assert.ok(result.ok);
    if (!result.ok) return;

    const err = new Error('Connection terminated unexpectedly') as NodeJS.ErrnoException;
    err.code = 'ECONNRESET';
    assert.doesNotThrow(() => result.pool.emit('error', err));

    // Stderr log fired first.
    assert.equal(captured.length, 1);
    // Callback fired second (and threw — but we caught it).
    assert.equal(auditCalls, 1);

    void closeDbClient(result);
  });

  it('onPoolError callback that returns a rejected Promise does NOT crash the process', () => {
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/z' },
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100,
      stderrWrite: () => undefined,
      onPoolError: () => Promise.reject(new Error('audit-store down'))
    });
    assert.ok(result.ok);
    if (!result.ok) return;

    // The handler's wrapper attaches .catch() to the returned Promise,
    // preventing the rejection from becoming an unhandled-rejection.
    const err = new Error('idle client error');
    assert.doesNotThrow(() => result.pool.emit('error', err));

    void closeDbClient(result);
  });

  it('sanitizes the error message to <=200 chars (defense-in-depth against pg leaking creds/long strings)', () => {
    const captured: string[] = [];
    const result = loadDbClient({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/z' },
      idleTimeoutMs: 100,
      connectionTimeoutMs: 100,
      stderrWrite: (line) => captured.push(line)
    });
    assert.ok(result.ok);
    if (!result.ok) return;

    // 1000-char error message including what looks like a secret. The
    // canary near the end must be truncated out of the logged detail.
    const longMsg = 'connection error: ' + 'x'.repeat(800) + ' BREVIO_TOKEN_KEK_LEAK_CANARY';
    const err = new Error(longMsg) as NodeJS.ErrnoException;
    err.code = 'EPIPE';
    result.pool.emit('error', err);

    const logged = JSON.parse(captured[0]);
    assert.ok(logged.attrs.message.length <= 200, `message must be bounded; got length ${logged.attrs.message.length}`);
    assert.equal(logged.attrs.message.includes('BREVIO_TOKEN_KEK_LEAK_CANARY'), false);

    void closeDbClient(result);
  });
});
