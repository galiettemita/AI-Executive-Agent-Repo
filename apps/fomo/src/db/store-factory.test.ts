import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { InMemoryCostStore } from '../core/cost-tracking.ts';
import { InMemoryToolInvocationStore } from '../core/tool-invocations.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryGmailCursorStore } from '../memory/gmail-cursors.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import { InMemoryTokenStore } from '../security/oauth/token-store.ts';
import { closeDbClient } from './client.ts';
import { createStores } from './store-factory.ts';

// Test crypto so we never need BREVIO_TOKEN_KEK in process env.
const TEST_CRYPTO = { kek: Buffer.alloc(32, 7), devMode: false };

describe('createStores — in-memory selection (default, no DATABASE_URL)', () => {
  it('returns in-memory bundle when DATABASE_URL is missing', () => {
    const handle = createStores({ env: {}, crypto: TEST_CRYPTO });
    assert.equal(handle.backend, 'in_memory');
    assert.equal(handle.db, null);
    assert.ok(handle.stores.audit instanceof InMemoryAuditStore);
    assert.ok(handle.stores.feedback instanceof InMemoryFeedbackStore);
    assert.ok(handle.stores.memory instanceof InMemoryMemorySignalStore);
    assert.ok(handle.stores.cost instanceof InMemoryCostStore);
    assert.ok(handle.stores.transitions instanceof InMemoryAlertStateTransitionStore);
    assert.ok(handle.stores.toolInvocations instanceof InMemoryToolInvocationStore);
    assert.ok(handle.stores.tokens instanceof InMemoryTokenStore);
    assert.ok(handle.stores.gmailCursors instanceof InMemoryGmailCursorStore);
    assert.ok(handle.stores.rankResults instanceof InMemoryRankResultStore);
  });

  it('returns in-memory bundle when BREVIO_DEV_MODE=true (regardless of NODE_ENV)', () => {
    const handle = createStores({
      env: { NODE_ENV: 'production', BREVIO_DEV_MODE: 'true' },
      crypto: TEST_CRYPTO
    });
    assert.equal(handle.backend, 'in_memory');
  });
});

describe('createStores — Postgres selection when DATABASE_URL is set', () => {
  it('returns postgres bundle when DATABASE_URL is set; exposes db handle for shutdown', async () => {
    const handle = createStores({
      env: { DATABASE_URL: 'postgres://x:y@localhost:5432/nowhere' },
      crypto: TEST_CRYPTO
    });
    assert.equal(handle.backend, 'postgres');
    assert.ok(handle.db);
    assert.equal(handle.db?.ok, true);
    // None of the in-memory class instanceof checks pass for Postgres stores.
    assert.equal(handle.stores.audit instanceof InMemoryAuditStore, false);
    assert.equal(handle.stores.feedback instanceof InMemoryFeedbackStore, false);
    assert.equal(handle.stores.memory instanceof InMemoryMemorySignalStore, false);
    assert.equal(handle.stores.cost instanceof InMemoryCostStore, false);
    assert.equal(handle.stores.transitions instanceof InMemoryAlertStateTransitionStore, false);
    assert.equal(handle.stores.toolInvocations instanceof InMemoryToolInvocationStore, false);
    assert.equal(handle.stores.tokens instanceof InMemoryTokenStore, false);
    assert.equal(handle.stores.gmailCursors instanceof InMemoryGmailCursorStore, false);
    assert.equal(handle.stores.rankResults instanceof InMemoryRankResultStore, false);
    // Close the pool so the test process does not hang on the open socket.
    if (handle.db) await closeDbClient(handle.db);
  });
});

describe('createStores — production fail-closed', () => {
  it('THROWS in production when DATABASE_URL is missing (loadDbClient rejects)', () => {
    assert.throws(
      () => createStores({ env: { NODE_ENV: 'production' }, crypto: TEST_CRYPTO }),
      /DATABASE_URL required in production/
    );
  });

  it('THROWS in production with empty DATABASE_URL', () => {
    assert.throws(
      () => createStores({ env: { NODE_ENV: 'production', DATABASE_URL: '' }, crypto: TEST_CRYPTO }),
      /DATABASE_URL required in production/
    );
  });
});

describe('createStores — handle and stores are frozen', () => {
  it('handle cannot be mutated', () => {
    const handle = createStores({ env: {}, crypto: TEST_CRYPTO });
    assert.throws(() => {
      (handle as unknown as { backend: string }).backend = 'mutated';
    });
  });

  it('stores bundle cannot be mutated', () => {
    const handle = createStores({ env: {}, crypto: TEST_CRYPTO });
    assert.throws(() => {
      (handle.stores as unknown as { audit: unknown }).audit = null;
    });
  });
});
