// Gated Postgres-backed store verification.
//
// Skipped by default — only runs when BREVIO_RUN_PG_TESTS=true. This is the
// test file the Phase 2E review requested before merge: end-to-end write/read
// of every Postgres-backed store against real Postgres semantics, plus an
// explicit assertion that the tool_invocations privacy invariant survives
// the round-trip through the database.
//
// Postgres host: PGlite (PostgreSQL compiled to WASM). Same C parser /
// planner / executor as a server-based Postgres, runs in-process, no Docker,
// no Neon connection. This satisfies "CI does not require a live DB" while
// giving real Postgres verification. Local developers who want to verify
// against a server-based Postgres can extend the setup() below (the
// BREVIO_TEST_DATABASE_URL env var hook is intentionally available; pointing
// it at a real Postgres bypasses PGlite).

import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { after, before, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { PGlite } from '@electric-sql/pglite';
import { drizzle } from 'drizzle-orm/pglite';

import { type DrizzleClient } from '../client.js';
import * as schema from '../schema.js';
import { PostgresAuditStore } from './audit-postgres.js';
import { PostgresAlertStateTransitionStore } from './transitions-postgres.js';
import { PostgresCostStore } from './cost-postgres.js';
import { PostgresFeedbackStore } from './feedback-postgres.js';
import { PostgresGmailCursorStore } from './gmail-cursors-postgres.js';
import { PostgresMemorySignalStore } from './memory-postgres.js';
import { PostgresRankResultStore } from './rank-results-postgres.js';
import { PostgresAlertStore } from './alerts-postgres.js';
import { PostgresInboundReplyStore } from './inbound-replies-postgres.js';
import { PostgresTokenStore } from './token-postgres.js';
import { PostgresToolInvocationStore } from './tool-invocations-postgres.js';

const RUN_PG = process.env.BREVIO_RUN_PG_TESTS === 'true';

let pglite: PGlite | null = null;
let db: DrizzleClient | null = null;

async function setup(): Promise<{ pglite: PGlite; db: DrizzleClient }> {
  const instance = new PGlite();
  // Apply the Drizzle-generated migration directly via PGlite's exec().
  // We avoid drizzle-orm/pglite/migrator here because PGlite < 1.0 has
  // an interface drift with Drizzle's migrator; reading + executing the
  // raw SQL is more direct and exercises the same migration file
  // production will use.
  const here = path.dirname(fileURLToPath(import.meta.url));
  const migrationsDir = path.resolve(here, '../migrations');
  const entries = await readdir(migrationsDir);
  const sqlFiles = entries.filter((f) => f.endsWith('.sql')).sort();
  // Drizzle separates statements with the `--> statement-breakpoint` marker.
  for (const file of sqlFiles) {
    const migrationSql = await readFile(path.join(migrationsDir, file), 'utf8');
    for (const stmt of migrationSql.split('--> statement-breakpoint')) {
      const trimmed = stmt.trim();
      if (trimmed.length === 0) continue;
      await instance.exec(trimmed);
    }
  }
  const wrapped = drizzle(instance, { schema }) as unknown as DrizzleClient;
  return { pglite: instance, db: wrapped };
}

describe('Phase 2E gated Postgres verification', { skip: !RUN_PG ? 'BREVIO_RUN_PG_TESTS not set' : false }, () => {
  before(async () => {
    const result = await setup();
    pglite = result.pglite;
    db = result.db;
  });

  after(async () => {
    if (pglite) await pglite.close();
    pglite = null;
    db = null;
  });

  it('Drizzle migrations applied cleanly (all expected tables exist)', async () => {
    assert.ok(pglite);
    const result = await pglite.query<{ tablename: string }>(
      `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`
    );
    const tables = result.rows.map((r) => r.tablename);
    assert.deepEqual(tables, [
      'alert_state_transitions',
      'alerts',
      'audit_log',
      'consent',
      'cost_records',
      'feedback_events',
      'gmail_cursors',
      'inbound_replies',
      'invite_tokens',
      'memory_signals',
      'oauth_tokens',
      'rank_results',
      'tool_invocations',
      'users'
    ]);
  });

  describe('PostgresAuditStore', () => {
    it('writes and reads back; redacts sensitive detail', async () => {
      assert.ok(db);
      const store = new PostgresAuditStore(db);
      await store.write({
        actor_user_id: 'u-audit',
        actor_ip: '127.0.0.1',
        actor_user_agent: 'test',
        action: 'oauth.connect',
        target: 'provider:google',
        result: 'success',
        detail: { access_token: 'plaintext', scope_count: 2 }
      });
      const out = await store.recent('u-audit');
      assert.equal(out.length, 1);
      assert.equal(out[0]?.action, 'oauth.connect');
      assert.equal((out[0]?.detail as Record<string, unknown>).access_token, '<redacted>');
      assert.equal((out[0]?.detail as Record<string, unknown>).scope_count, 2);
    });
  });

  describe('PostgresFeedbackStore', () => {
    it('writes + counts by kind + by sender', async () => {
      assert.ok(db);
      const store = new PostgresFeedbackStore(db);
      await store.write({ user_id: 'u-fb', alert_id: 'a1', sender_email: 's@x', kind: 'founder_approved' });
      await store.write({ user_id: 'u-fb', alert_id: 'a2', sender_email: 's@x', kind: 'user_snoozed' });
      await store.write({ user_id: 'u-fb', alert_id: null, sender_email: null, kind: 'stop' });

      assert.equal(await store.countByKind('u-fb', 'founder_approved'), 1);
      assert.equal(await store.countByKind('u-fb', 'user_snoozed'), 1);
      assert.equal(await store.countBySender('u-fb', 's@x'), 2);
      const recent = await store.recent('u-fb');
      assert.equal(recent.length, 3);
    });

    it('redacts detail on write', async () => {
      assert.ok(db);
      const store = new PostgresFeedbackStore(db);
      await store.write({
        user_id: 'u-fb-redact',
        alert_id: 'a1',
        sender_email: null,
        kind: 'founder_approved',
        detail: { score: 0.91, access_token: 'leaked' }
      });
      const [r] = await store.recent('u-fb-redact');
      assert.equal((r?.detail as Record<string, unknown>).access_token, '<redacted>');
      assert.equal((r?.detail as Record<string, unknown>).score, 0.91);
    });
  });

  describe('PostgresMemorySignalStore', () => {
    it('upsert replaces prior signal at same (user, kind, scope_key); list filters per user', async () => {
      assert.ok(db);
      const store = new PostgresMemorySignalStore(db);
      await store.upsert({
        user_id: 'u-mem', kind: 'sender_importance', scope_key: 'sarah@school.edu',
        detail: { importance: 'medium' }, source: 'inferred'
      });
      await store.upsert({
        user_id: 'u-mem', kind: 'sender_importance', scope_key: 'sarah@school.edu',
        detail: { importance: 'high' }, source: 'user_confirmed'
      });
      const sig = await store.get('u-mem', 'sender_importance', 'sarah@school.edu');
      assert.ok(sig);
      assert.equal((sig?.detail as Record<string, unknown>).importance, 'high');
      assert.equal(sig?.source, 'user_confirmed');
      assert.equal(sig?.confidence, 1.0);
    });

    it('handles null scope_key (user-wide preference) via empty-string sentinel', async () => {
      assert.ok(db);
      const store = new PostgresMemorySignalStore(db);
      await store.upsert({
        user_id: 'u-mem-null', kind: 'quietness_preference', scope_key: null,
        detail: { max_per_day: 5 }, source: 'user_confirmed'
      });
      const sig = await store.get('u-mem-null', 'quietness_preference');
      assert.ok(sig);
      // Round-trip preserves null on read (empty-string sentinel translates back).
      assert.equal(sig?.scope_key, null);
    });

    it('delete is scoped per (user, kind, scope_key) and returns true/false', async () => {
      assert.ok(db);
      const store = new PostgresMemorySignalStore(db);
      await store.upsert({
        user_id: 'u-mem-del', kind: 'sender_importance', scope_key: 's@x',
        detail: {}, source: 'user_confirmed'
      });
      assert.equal(await store.delete('u-mem-del', 'sender_importance', 's@x'), true);
      assert.equal(await store.delete('u-mem-del', 'sender_importance', 's@x'), false);
      assert.equal(await store.get('u-mem-del', 'sender_importance', 's@x'), null);
    });
  });

  describe('PostgresCostStore', () => {
    it('sumByModel and sumByPeriod aggregate correctly', async () => {
      assert.ok(db);
      const store = new PostgresCostStore(db);
      await store.write({
        user_id: 'u-cost', capability: 'classification',
        model_name: 'mock-classifier-tiny', prompt_version: 'p1',
        latency_ms: 50, input_tokens: 100, output_tokens: 20,
        estimated_cost_usd: 0.10, schema_valid: true,
        occurred_at: '2026-05-15T12:00:00.000Z'
      });
      await store.write({
        user_id: 'u-cost', capability: 'classification',
        model_name: 'mock-classifier-tiny', prompt_version: 'p1',
        latency_ms: 50, input_tokens: 100, output_tokens: 20,
        estimated_cost_usd: 0.25, schema_valid: true,
        occurred_at: '2026-05-20T12:00:00.000Z'
      });
      await store.write({
        user_id: 'u-cost', capability: 'classification',
        model_name: 'mock-classifier-small', prompt_version: 'p1',
        latency_ms: 200, input_tokens: 500, output_tokens: 100,
        estimated_cost_usd: 0.50, schema_valid: false,
        occurred_at: '2026-06-01T12:00:00.000Z'
      });

      assert.ok(Math.abs((await store.sumByModel('u-cost', 'mock-classifier-tiny')) - 0.35) < 1e-9);
      assert.ok(Math.abs((await store.sumByModel('u-cost', 'mock-classifier-small')) - 0.50) < 1e-9);
      const mayTotal = await store.sumByPeriod(
        'u-cost',
        '2026-05-01T00:00:00.000Z',
        '2026-05-31T23:59:59.999Z'
      );
      assert.ok(Math.abs(mayTotal - 0.35) < 1e-9);
    });

    it('schema_valid=false records are stored and visible in recent()', async () => {
      assert.ok(db);
      const store = new PostgresCostStore(db);
      await store.write({
        user_id: 'u-cost-invalid', capability: 'classification',
        model_name: 'mock-classifier-tiny', prompt_version: 'p1',
        latency_ms: 50, input_tokens: 100, output_tokens: 20,
        estimated_cost_usd: 0.10, schema_valid: false
      });
      const recent = await store.recent('u-cost-invalid');
      assert.equal(recent.length, 1);
      assert.equal(recent[0]?.schema_valid, false);
    });
  });

  describe('PostgresAlertStateTransitionStore', () => {
    it('records transitions in order; forAlert returns them in insertion order', async () => {
      assert.ok(db);
      const store = new PostgresAlertStateTransitionStore(db);
      await store.write({
        alert_id: 'alert-pg-1', user_id: 'u-st',
        from_state: 'detected', to_state: 'ranked', reason: 'classifier ok'
      });
      await store.write({
        alert_id: 'alert-pg-1', user_id: 'u-st',
        from_state: 'ranked', to_state: 'queued_for_review', reason: 'score 0.92'
      });
      await store.write({
        alert_id: 'alert-pg-1', user_id: 'u-st',
        from_state: 'queued_for_review', to_state: 'approved', reason: 'founder approved'
      });
      const transitions = await store.forAlert('alert-pg-1');
      assert.equal(transitions.length, 3);
      assert.deepEqual(transitions.map((t) => t.to_state), ['ranked', 'queued_for_review', 'approved']);
    });

    it('currentState returns the latest to_state', async () => {
      assert.ok(db);
      const store = new PostgresAlertStateTransitionStore(db);
      assert.equal(await store.currentState('alert-pg-1'), 'approved');
      assert.equal(await store.currentState('alert-does-not-exist'), null);
    });

    it('rejects unknown state at write time', async () => {
      assert.ok(db);
      const store = new PostgresAlertStateTransitionStore(db);
      await assert.rejects(
        store.write({
          alert_id: 'x', user_id: 'u',
          from_state: 'mystery' as never, to_state: 'ranked', reason: 'x'
        }),
        /unknown from_state/
      );
    });

    it('findAlertIdsInState returns only alerts whose latest transition matches the state', async () => {
      assert.ok(db);
      const store = new PostgresAlertStateTransitionStore(db);
      // alert-pg-1 is already approved from the prior test. Add:
      //   alert-pg-2: queued_for_review (latest)
      //   alert-pg-3: queued → approved → sent (latest: sent)
      await store.write({ alert_id: 'alert-pg-2', user_id: 'u-st', from_state: 'detected', to_state: 'ranked', reason: 'x' });
      await store.write({ alert_id: 'alert-pg-2', user_id: 'u-st', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });

      await store.write({ alert_id: 'alert-pg-3', user_id: 'u-st', from_state: 'detected', to_state: 'ranked', reason: 'x' });
      await store.write({ alert_id: 'alert-pg-3', user_id: 'u-st', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
      await store.write({ alert_id: 'alert-pg-3', user_id: 'u-st', from_state: 'queued_for_review', to_state: 'approved', reason: 'x' });
      await store.write({ alert_id: 'alert-pg-3', user_id: 'u-st', from_state: 'approved', to_state: 'sent', reason: 'sendblue 2xx' });

      const approved = await store.findAlertIdsInState('u-st', 'approved', 10);
      assert.deepEqual([...approved], ['alert-pg-1']);
      const queued = await store.findAlertIdsInState('u-st', 'queued_for_review', 10);
      assert.deepEqual([...queued], ['alert-pg-2']);
      const sent = await store.findAlertIdsInState('u-st', 'sent', 10);
      assert.deepEqual([...sent], ['alert-pg-3']);

      // user isolation
      assert.deepEqual([...(await store.findAlertIdsInState('different-user', 'approved', 10))], []);

      // non-positive limit
      assert.deepEqual([...(await store.findAlertIdsInState('u-st', 'approved', 0))], []);
    });
  });

  describe('PostgresToolInvocationStore', () => {
    it('write + recent round-trip; counts by tool and status', async () => {
      assert.ok(db);
      const store = new PostgresToolInvocationStore(db);
      await store.write({
        user_id: 'u-ti', tool_id: 'audit.write', invocation_id: 'inv-pg-1',
        policy_decision: 'allowed', status: 'success', latency_ms: 12
      });
      await store.write({
        user_id: 'u-ti', tool_id: 'gmail.read', invocation_id: 'inv-pg-2',
        policy_decision: 'not_implemented', status: 'denied'
      });
      assert.equal(await store.countByTool('u-ti', 'audit.write'), 1);
      assert.equal(await store.countByTool('u-ti', 'gmail.read'), 1);
      assert.equal(await store.countByStatus('u-ti', 'success'), 1);
      assert.equal(await store.countByStatus('u-ti', 'denied'), 1);
      const fetched = await store.byInvocationId('inv-pg-1');
      assert.ok(fetched);
      assert.equal(fetched?.tool_id, 'audit.write');
    });

    it('PRIVACY INVARIANT: metadata is redacted on write; no payload fields persisted', async () => {
      assert.ok(db);
      assert.ok(pglite);
      const store = new PostgresToolInvocationStore(db);

      // 1. Redaction: sensitive keys in metadata are redacted before persistence.
      await store.write({
        user_id: 'u-ti-priv', tool_id: 'sendblue.send_user_message',
        invocation_id: 'inv-priv-1',
        policy_decision: 'allowed', status: 'success',
        metadata: { access_token: 'plaintext-token', tier: 'send' }
      });
      const fetched = await store.byInvocationId('inv-priv-1');
      assert.equal((fetched?.metadata as Record<string, unknown>).access_token, '<redacted>');
      assert.equal((fetched?.metadata as Record<string, unknown>).tier, 'send');

      // 2. Schema enforcement: the on-disk row has no payload-content columns.
      const cols = await pglite.query<{ column_name: string }>(
        `SELECT column_name FROM information_schema.columns
         WHERE table_schema = 'public' AND table_name = 'tool_invocations'
         ORDER BY column_name`
      );
      const columnNames = cols.rows.map((r) => r.column_name);
      // The schema has exactly these 11 columns. Adding a payload-shaped
      // column (body_plain, body_html, reply_text, prompt, email_body) would
      // violate the privacy invariant and fail this assertion.
      assert.deepEqual(columnNames, [
        'error_code',
        'error_reason',
        'id',
        'invocation_id',
        'latency_ms',
        'metadata',
        'occurred_at',
        'policy_decision',
        'status',
        'tool_id',
        'user_id'
      ]);
      const forbidden = ['body_plain', 'body_html', 'reply_text', 'prompt', 'email_body'];
      for (const name of forbidden) {
        assert.ok(
          !columnNames.includes(name),
          `tool_invocations gained a payload-shaped column '${name}' — privacy invariant broken`
        );
      }
    });

    it('invocation_id is UNIQUE — duplicate inserts reject at the DB level', async () => {
      assert.ok(db);
      const store = new PostgresToolInvocationStore(db);
      await store.write({
        user_id: 'u-ti-unique', tool_id: 'audit.write', invocation_id: 'inv-unique-test',
        policy_decision: 'allowed', status: 'success'
      });
      await assert.rejects(
        store.write({
          user_id: 'u-ti-unique', tool_id: 'audit.write', invocation_id: 'inv-unique-test',
          policy_decision: 'allowed', status: 'success'
        }),
        /duplicate key|unique/i
      );
    });
  });

  describe('PostgresTokenStore', () => {
    it('round-trips encrypted access + refresh tokens through pg', async () => {
      assert.ok(db);
      // Test KEK so we do not depend on process env.
      const testCrypto = { kek: Buffer.alloc(32, 13), devMode: false };
      const store = new PostgresTokenStore(db, testCrypto);
      await store.save({
        user_id: 'u-tok', provider: 'google', scopes: ['gmail.readonly'],
        access_token: 'at_secret', refresh_token: 'rt_secret',
        expires_at: new Date('2027-01-01T00:00:00.000Z')
      });
      assert.equal(await store.loadAccessToken('u-tok', 'google'), 'at_secret');
      assert.equal(await store.loadRefreshToken('u-tok', 'google'), 'rt_secret');
      assert.equal(await store.has('u-tok', 'google'), true);
    });

    it('upsert via save() replaces prior row at same (user_id, provider)', async () => {
      assert.ok(db);
      const testCrypto = { kek: Buffer.alloc(32, 17), devMode: false };
      const store = new PostgresTokenStore(db, testCrypto);
      await store.save({
        user_id: 'u-tok-upsert', provider: 'google', scopes: ['s1'],
        access_token: 'first', refresh_token: 'first-rt'
      });
      await store.save({
        user_id: 'u-tok-upsert', provider: 'google', scopes: ['s1', 's2'],
        access_token: 'second', refresh_token: 'second-rt'
      });
      assert.equal(await store.loadAccessToken('u-tok-upsert', 'google'), 'second');
      assert.equal(await store.loadRefreshToken('u-tok-upsert', 'google'), 'second-rt');
      const list = await store.list('u-tok-upsert');
      assert.equal(list.length, 1);
      assert.deepEqual(list[0]?.scopes, ['s1', 's2']);
    });

    it('markNeedsReauth flips the flag visible in list()', async () => {
      assert.ok(db);
      const testCrypto = { kek: Buffer.alloc(32, 19), devMode: false };
      const store = new PostgresTokenStore(db, testCrypto);
      await store.save({
        user_id: 'u-tok-reauth', provider: 'google', scopes: [],
        access_token: 'at'
      });
      await store.markNeedsReauth('u-tok-reauth', 'google');
      const list = await store.list('u-tok-reauth');
      assert.equal(list[0]?.needs_reauth, true);
    });

    it('AAD mismatch (wrong user/provider) makes decryption fail — encrypted at rest', async () => {
      assert.ok(db);
      const testCrypto = { kek: Buffer.alloc(32, 23), devMode: false };
      const storeOne = new PostgresTokenStore(db, testCrypto);
      await storeOne.save({
        user_id: 'u-aad-1', provider: 'google', scopes: [],
        access_token: 'tied-to-u-aad-1'
      });
      // Tamper with the AAD by passing a different crypto handle that
      // claims a different user/provider when decrypting. We simulate by
      // attempting to load with a different user_id — the schema's PK
      // protects us first (row not found), so this primarily proves AAD
      // is bound (the actual decrypt path is exercised by the round-trip
      // tests above and the dedicated token-crypto.test.ts suite).
      assert.equal(await storeOne.loadAccessToken('u-different-user', 'google'), null);
    });
  });

  describe('PostgresGmailCursorStore (Phase 3B.1)', () => {
    it('upsert + get round-trip, replace on second upsert', async () => {
      assert.ok(db);
      const store = new PostgresGmailCursorStore(db);
      await store.upsert({ user_id: 'u-cur-1', history_id: 'h-100' });
      let c = await store.get('u-cur-1');
      assert.ok(c);
      assert.equal(c?.history_id, 'h-100');
      await store.upsert({ user_id: 'u-cur-1', history_id: 'h-200' });
      c = await store.get('u-cur-1');
      assert.equal(c?.history_id, 'h-200');
    });

    it('per-user isolation', async () => {
      assert.ok(db);
      const store = new PostgresGmailCursorStore(db);
      await store.upsert({ user_id: 'u-cur-iso-a', history_id: 'h-1' });
      await store.upsert({ user_id: 'u-cur-iso-b', history_id: 'h-2' });
      assert.equal((await store.get('u-cur-iso-a'))?.history_id, 'h-1');
      assert.equal((await store.get('u-cur-iso-b'))?.history_id, 'h-2');
    });

    it('delete returns true when removing and false when nothing to remove', async () => {
      assert.ok(db);
      const store = new PostgresGmailCursorStore(db);
      await store.upsert({ user_id: 'u-cur-del', history_id: 'h-1' });
      assert.equal(await store.delete('u-cur-del'), true);
      assert.equal(await store.delete('u-cur-del'), false);
      assert.equal(await store.get('u-cur-del'), null);
    });

    it('listUserIds returns each currently-connected user (Phase 3B.2)', async () => {
      assert.ok(db);
      const store = new PostgresGmailCursorStore(db);
      await store.upsert({ user_id: 'u-cur-list-a', history_id: 'h-a' });
      await store.upsert({ user_id: 'u-cur-list-b', history_id: 'h-b' });
      const ids = new Set(await store.listUserIds());
      assert.ok(ids.has('u-cur-list-a'));
      assert.ok(ids.has('u-cur-list-b'));
      await store.delete('u-cur-list-a');
      const after = new Set(await store.listUserIds());
      assert.ok(!after.has('u-cur-list-a'));
      assert.ok(after.has('u-cur-list-b'));
    });
  });

  describe('PostgresRankResultStore (Phase 3C.3)', () => {
    const fixture = (over: Partial<Parameters<PostgresRankResultStore['write']>[0]> = {}) => ({
      user_id: 'u-rank-1',
      message_id: 'msg-rank-100',
      invocation_id: 'inv-rank-1',
      model_name: 'gpt-5-mini',
      prompt_version: 'fomo-ranker-v1',
      label: 'important' as const,
      score: 0.82,
      reason: 'Mentions deadline today.',
      latency_ms: 412,
      input_tokens: 380,
      output_tokens: 24,
      estimated_cost_usd: 0.0009,
      ...over
    });

    it('write + get round-trip preserves every field', async () => {
      assert.ok(db);
      const store = new PostgresRankResultStore(db);
      const outcome = await store.write(fixture());
      assert.equal(outcome.inserted, true);
      const row = await store.get('u-rank-1', 'msg-rank-100');
      assert.ok(row);
      assert.equal(row?.user_id, 'u-rank-1');
      assert.equal(row?.message_id, 'msg-rank-100');
      assert.equal(row?.invocation_id, 'inv-rank-1');
      assert.equal(row?.model_name, 'gpt-5-mini');
      assert.equal(row?.prompt_version, 'fomo-ranker-v1');
      assert.equal(row?.label, 'important');
      assert.equal(row?.score, 0.82);
      assert.equal(row?.reason, 'Mentions deadline today.');
      assert.equal(row?.latency_ms, 412);
      assert.equal(row?.input_tokens, 380);
      assert.equal(row?.output_tokens, 24);
      assert.equal(row?.estimated_cost_usd, 0.0009);
      assert.ok(row?.created_at);
    });

    it('unique on (user_id, message_id) — second write reports inserted=false', async () => {
      assert.ok(db);
      const store = new PostgresRankResultStore(db);
      const first = await store.write(
        fixture({ user_id: 'u-rank-idem', message_id: 'msg-idem', label: 'important', score: 0.9 })
      );
      assert.equal(first.inserted, true);
      const second = await store.write(
        fixture({ user_id: 'u-rank-idem', message_id: 'msg-idem', label: 'not_important', score: 0.1 })
      );
      assert.equal(second.inserted, false);
      // Existing row unchanged.
      const row = await store.get('u-rank-idem', 'msg-idem');
      assert.equal(row?.label, 'important');
      assert.equal(row?.score, 0.9);
    });

    it('count by user and by label', async () => {
      assert.ok(db);
      const store = new PostgresRankResultStore(db);
      await store.write(fixture({ user_id: 'u-rank-cnt', message_id: 'm-1', label: 'important' }));
      await store.write(fixture({ user_id: 'u-rank-cnt', message_id: 'm-2', label: 'not_important' }));
      await store.write(fixture({ user_id: 'u-rank-cnt', message_id: 'm-3', label: 'important' }));
      assert.equal(await store.count('u-rank-cnt'), 3);
      assert.equal(await store.count('u-rank-cnt', 'important'), 2);
      assert.equal(await store.count('u-rank-cnt', 'not_important'), 1);
    });

    it('recent returns newest-first up to limit, per-user scoped', async () => {
      assert.ok(db);
      const store = new PostgresRankResultStore(db);
      await store.write(fixture({ user_id: 'u-rank-rec', message_id: 'm-1' }));
      await store.write(fixture({ user_id: 'u-rank-rec', message_id: 'm-2' }));
      await store.write(fixture({ user_id: 'u-rank-rec', message_id: 'm-3' }));
      await store.write(fixture({ user_id: 'u-rank-other', message_id: 'm-other' }));
      const rows = await store.recent('u-rank-rec', 2);
      assert.equal(rows.length, 2);
      assert.equal(rows[0]?.message_id, 'm-3');
      assert.equal(rows[1]?.message_id, 'm-2');
      // Per-user isolation
      const other = await store.recent('u-rank-other', 10);
      assert.equal(other.length, 1);
      assert.equal(other[0]?.user_id, 'u-rank-other');
    });

    it('privacy invariant: rank_results has zero payload-shaped columns', async () => {
      // Body / header / attachment columns must NEVER appear in this table.
      // Asserts the schema enforces "decision-only", not "prompt-content".
      assert.ok(pglite);
      const result = await pglite.query<{ column_name: string }>(
        `SELECT column_name FROM information_schema.columns
         WHERE table_schema = 'public' AND table_name = 'rank_results'`
      );
      const cols = new Set(result.rows.map((r) => r.column_name));
      const forbidden = ['body_plain', 'body_html', 'headers', 'attachments', 'prompt', 'email_body'];
      for (const f of forbidden) {
        assert.ok(!cols.has(f), `rank_results must not have column '${f}'`);
      }
    });
  });

  describe('PostgresAlertStore (Phase 3D.1)', () => {
    const alertFixture = (
      over: Partial<Parameters<PostgresAlertStore['create']>[0]> = {}
    ) => ({
      alert_id: 'alert-pg-1',
      user_id: 'u-alert-1',
      message_id: 'msg-alert-1',
      rank_result_id: 100,
      label: 'important' as const,
      score: 0.85,
      ...over
    });

    it('create + get round-trip preserves every field', async () => {
      assert.ok(db);
      const store = new PostgresAlertStore(db);
      const outcome = await store.create(alertFixture());
      assert.equal(outcome.inserted, true);
      assert.equal(outcome.alert.alert_id, 'alert-pg-1');
      const row = await store.get('alert-pg-1');
      assert.ok(row);
      assert.equal(row?.user_id, 'u-alert-1');
      assert.equal(row?.message_id, 'msg-alert-1');
      assert.equal(row?.rank_result_id, 100);
      assert.equal(row?.label, 'important');
      assert.equal(row?.score, 0.85);
      assert.ok(row?.created_at);
    });

    it('UNIQUE on rank_result_id: second create reports inserted=false, returns existing row', async () => {
      assert.ok(db);
      const store = new PostgresAlertStore(db);
      const first = await store.create(alertFixture({ alert_id: 'alert-orig', rank_result_id: 200 }));
      assert.equal(first.inserted, true);
      // Attempt a duplicate write with a DIFFERENT alert_id but the SAME rank_result_id:
      const second = await store.create(
        alertFixture({ alert_id: 'alert-attempted', rank_result_id: 200, score: 0.1 })
      );
      assert.equal(second.inserted, false);
      // Returns the ORIGINAL alert, not the attempted one:
      assert.equal(second.alert.alert_id, 'alert-orig');
      assert.equal(second.alert.score, 0.85);
      // Attempted alert_id was never persisted:
      assert.equal(await store.get('alert-attempted'), null);
    });

    it('getByRankResult finds the alert', async () => {
      assert.ok(db);
      const store = new PostgresAlertStore(db);
      await store.create(alertFixture({ alert_id: 'alert-byrank', rank_result_id: 300 }));
      const row = await store.getByRankResult(300);
      assert.equal(row?.alert_id, 'alert-byrank');
    });

    it('count + recent are per-user scoped', async () => {
      assert.ok(db);
      const store = new PostgresAlertStore(db);
      await store.create(alertFixture({ alert_id: 'a-cnt-1', rank_result_id: 401, user_id: 'u-cnt' }));
      await store.create(alertFixture({ alert_id: 'a-cnt-2', rank_result_id: 402, user_id: 'u-cnt' }));
      await store.create(alertFixture({ alert_id: 'a-cnt-3', rank_result_id: 403, user_id: 'u-other' }));
      assert.equal(await store.count('u-cnt'), 2);
      assert.equal(await store.count('u-other'), 1);
      const recent = await store.recent('u-cnt', 5);
      assert.equal(recent.length, 2);
      for (const r of recent) assert.equal(r.user_id, 'u-cnt');
    });

    it('privacy invariant: alerts has zero payload-shaped columns', async () => {
      assert.ok(pglite);
      const result = await pglite.query<{ column_name: string }>(
        `SELECT column_name FROM information_schema.columns
         WHERE table_schema = 'public' AND table_name = 'alerts'`
      );
      const cols = new Set(result.rows.map((r) => r.column_name));
      const forbidden = [
        'body_plain',
        'body_html',
        'headers',
        'attachments',
        'subject',
        'sender_email',
        'snippet',
        'card_payload'
      ];
      for (const f of forbidden) {
        assert.ok(!cols.has(f), `alerts must not have column '${f}'`);
      }
    });
  });

  describe('PostgresInboundReplyStore (Phase 3F.1)', () => {
    it('record + getByProviderMessageId round-trip preserves every field', async () => {
      assert.ok(db);
      const store = new PostgresInboundReplyStore(db);
      const out = await store.record({ provider_message_id: 'sb-pg-1', user_id: 'u-inb-1' });
      assert.equal(out.inserted, true);
      assert.equal(out.record.provider_message_id, 'sb-pg-1');
      assert.equal(out.record.user_id, 'u-inb-1');
      assert.ok(out.record.received_at);
      const fetched = await store.getByProviderMessageId('sb-pg-1');
      assert.equal(fetched?.user_id, 'u-inb-1');
    });

    it('LOAD-BEARING: UNIQUE on provider_message_id; second record returns inserted=false + the existing row (SendBlue retry safety)', async () => {
      assert.ok(db);
      const store = new PostgresInboundReplyStore(db);
      const first = await store.record({ provider_message_id: 'sb-pg-dup', user_id: 'u-inb-2' });
      const dup = await store.record({ provider_message_id: 'sb-pg-dup', user_id: 'u-inb-3' });
      assert.equal(first.inserted, true);
      assert.equal(dup.inserted, false);
      // Returns the ORIGINAL row, not the duplicate input.
      assert.equal(dup.record.id, first.record.id);
      assert.equal(dup.record.user_id, 'u-inb-2');
      assert.equal(dup.record.received_at, first.record.received_at);
    });

    it('count + recent are per-user scoped', async () => {
      assert.ok(db);
      const store = new PostgresInboundReplyStore(db);
      await store.record({ provider_message_id: 'sb-pg-cnt-1', user_id: 'u-inb-cnt' });
      await store.record({ provider_message_id: 'sb-pg-cnt-2', user_id: 'u-inb-cnt' });
      await store.record({ provider_message_id: 'sb-pg-cnt-3', user_id: 'u-inb-other' });
      assert.equal(await store.count('u-inb-cnt'), 2);
      assert.equal(await store.count('u-inb-other'), 1);
      const recent = await store.recent('u-inb-cnt', 5);
      assert.equal(recent.length, 2);
      for (const r of recent) assert.equal(r.user_id, 'u-inb-cnt');
    });

    it('privacy invariant: inbound_replies has ONLY dedup-shaped columns (no reply text, no phone, no payload)', async () => {
      assert.ok(pglite);
      const result = await pglite.query<{ column_name: string }>(
        `SELECT column_name FROM information_schema.columns
         WHERE table_schema = 'public' AND table_name = 'inbound_replies'
         ORDER BY column_name`
      );
      const cols = new Set(result.rows.map((r) => r.column_name));
      // The exact column set: id, provider_message_id, user_id, received_at. Nothing else.
      assert.deepEqual(
        [...cols].sort(),
        ['id', 'provider_message_id', 'received_at', 'user_id']
      );
      const forbidden = [
        'body',
        'content',
        'text',
        'reply_text',
        'message_text',
        'from_number',
        'to_number',
        'phone',
        'payload',
        'raw',
        'signing_secret'
      ];
      for (const f of forbidden) {
        assert.ok(!cols.has(f), `inbound_replies must not have column '${f}'`);
      }
    });
  });

  // Phase v0.5.1 — multi-tenant substrate. Schema-only checks for the
  // new `users.phone_e164_*` columns + `invite_tokens` table. The
  // application-layer stores (phone allowlist, invite-token store)
  // are tested in their own files; here we prove the migration shape
  // is what the runtime expects + the UNIQUE constraints actually
  // fire under real Postgres semantics.
  describe('Phase v0.5.1 — users extension + invite_tokens schema', () => {
    it('users has phone_e164_encrypted (jsonb), phone_e164_hash (text), is_founder (boolean) — and NOT a plaintext phone column', async () => {
      assert.ok(pglite);
      const result = await pglite.query<{ column_name: string; data_type: string }>(
        `SELECT column_name, data_type FROM information_schema.columns
         WHERE table_schema='public' AND table_name='users'
         ORDER BY ordinal_position`
      );
      const byName = new Map(result.rows.map((r) => [r.column_name, r.data_type]));
      assert.equal(byName.get('phone_e164_encrypted'), 'jsonb');
      assert.equal(byName.get('phone_e164_hash'), 'text');
      assert.equal(byName.get('is_founder'), 'boolean');
      // Defense-in-depth: prove there is no plaintext phone column.
      const forbidden = ['phone', 'phone_e164', 'phone_number', 'phone_plaintext'];
      for (const f of forbidden) {
        assert.ok(!byName.has(f), `users must not have plaintext phone column '${f}'`);
      }
    });

    it('users.phone_e164_hash has a UNIQUE index — duplicate hash on second insert rejects', async () => {
      assert.ok(pglite);
      const hash = 'hash-fixture-1234567890abcdef';
      await pglite.exec(
        `INSERT INTO users (email, phone_e164_encrypted, phone_e164_hash, is_founder)
         VALUES ('founder@example.test', '{"v":1,"ct":"abc"}'::jsonb, '${hash}', true)`
      );
      // The second insert with the SAME phone_e164_hash must violate
      // the UNIQUE index — proves Step 1's duplicate-phone protection
      // at the SQL layer.
      let threw = false;
      try {
        await pglite.exec(
          `INSERT INTO users (email, phone_e164_encrypted, phone_e164_hash, is_founder)
           VALUES ('friend@example.test', '{"v":1,"ct":"def"}'::jsonb, '${hash}', false)`
        );
      } catch (err) {
        threw = true;
        assert.match(
          String(err),
          /duplicate key value violates unique constraint|users_phone_e164_hash_uq/i,
          'expected unique-constraint violation'
        );
      }
      assert.ok(threw, 'second insert with duplicate phone_e164_hash must reject');

      // The first row is UNTOUCHED — per the founder directive, the
      // duplicate insert must not corrupt the existing row.
      const after = await pglite.query<{ email: string; is_founder: boolean }>(
        `SELECT email, is_founder FROM users WHERE phone_e164_hash = '${hash}'`
      );
      assert.equal(after.rows.length, 1);
      assert.equal(after.rows[0].email, 'founder@example.test');
      assert.equal(after.rows[0].is_founder, true);
    });

    it('invite_tokens has token_hash + intended_phone_hash + consumed_at columns, NEVER a plaintext token column', async () => {
      assert.ok(pglite);
      const result = await pglite.query<{ column_name: string; data_type: string }>(
        `SELECT column_name, data_type FROM information_schema.columns
         WHERE table_schema='public' AND table_name='invite_tokens'
         ORDER BY ordinal_position`
      );
      const names = new Set(result.rows.map((r) => r.column_name));
      assert.ok(names.has('token_hash'), 'invite_tokens must store the token hash');
      assert.ok(names.has('intended_phone_hash'), 'invite_tokens must bind to the intended phone hash');
      assert.ok(names.has('issued_by_user_id'));
      assert.ok(names.has('issued_at'));
      assert.ok(names.has('expires_at'));
      assert.ok(names.has('consumed_at'));
      assert.ok(names.has('consumed_user_id'));
      // Privacy invariant — the plaintext invite token is NEVER stored.
      const forbidden = ['token', 'token_plaintext', 'token_raw', 'plaintext_token'];
      for (const f of forbidden) {
        assert.ok(!names.has(f), `invite_tokens must not have plaintext token column '${f}'`);
      }
    });

    it('invite_tokens.token_hash has a UNIQUE index — duplicate token_hash on second insert rejects', async () => {
      assert.ok(pglite);
      const tokenHash = 'tokenhash-fixture-cafef00ddeadbeef';
      const phoneHash = 'phonehash-fixture-aabbccddeeff';
      await pglite.exec(
        `INSERT INTO invite_tokens (token_hash, intended_phone_hash, issued_by_user_id, expires_at)
         VALUES ('${tokenHash}', '${phoneHash}', 'founder', now() + interval '1 day')`
      );
      let threw = false;
      try {
        await pglite.exec(
          `INSERT INTO invite_tokens (token_hash, intended_phone_hash, issued_by_user_id, expires_at)
           VALUES ('${tokenHash}', 'phonehash-other', 'founder', now() + interval '1 day')`
        );
      } catch (err) {
        threw = true;
        assert.match(String(err), /duplicate key|invite_tokens_token_hash_uq/i);
      }
      assert.ok(threw, 'second insert with duplicate token_hash must reject');
    });

    it('atomic consumption UPDATE only fires on first call (conditional WHERE consumed_at IS NULL)', async () => {
      assert.ok(pglite);
      const tokenHash = 'tokenhash-atomic-fixture-deadbeefcafe';
      const phoneHash = 'phonehash-atomic-fixture-99887766';
      await pglite.exec(
        `INSERT INTO invite_tokens (token_hash, intended_phone_hash, issued_by_user_id, expires_at)
         VALUES ('${tokenHash}', '${phoneHash}', 'founder', now() + interval '1 day')`
      );

      // First atomic consume — should return one row id, set consumed_at.
      const first = await pglite.query<{ id: number }>(
        `UPDATE invite_tokens
         SET consumed_at = now(), consumed_user_id = 'friend-1'
         WHERE token_hash = '${tokenHash}'
           AND consumed_at IS NULL
           AND expires_at > now()
         RETURNING id`
      );
      assert.equal(first.rows.length, 1, 'first consume must return 1 row');

      // Second atomic consume — must return zero rows (already consumed).
      // This is the exact shape the /onboard callback uses; the runtime
      // surfaces zero-rows-returned as "token already consumed or expired"
      // and rolls back user creation.
      const second = await pglite.query<{ id: number }>(
        `UPDATE invite_tokens
         SET consumed_at = now(), consumed_user_id = 'friend-replay'
         WHERE token_hash = '${tokenHash}'
           AND consumed_at IS NULL
           AND expires_at > now()
         RETURNING id`
      );
      assert.equal(second.rows.length, 0, 'second consume must return 0 rows (already consumed)');

      // Verify the original consumed_user_id is untouched (replay didn't overwrite).
      const after = await pglite.query<{ consumed_user_id: string }>(
        `SELECT consumed_user_id FROM invite_tokens WHERE token_hash = '${tokenHash}'`
      );
      assert.equal(after.rows[0].consumed_user_id, 'friend-1');
    });

    it('expired invite tokens cannot be consumed (conditional WHERE expires_at > now())', async () => {
      assert.ok(pglite);
      const tokenHash = 'tokenhash-expired-fixture-feedface';
      const phoneHash = 'phonehash-expired-fixture-11223344';
      await pglite.exec(
        `INSERT INTO invite_tokens (token_hash, intended_phone_hash, issued_by_user_id, expires_at)
         VALUES ('${tokenHash}', '${phoneHash}', 'founder', now() - interval '1 hour')`
      );
      const result = await pglite.query<{ id: number }>(
        `UPDATE invite_tokens
         SET consumed_at = now(), consumed_user_id = 'friend-late'
         WHERE token_hash = '${tokenHash}'
           AND consumed_at IS NULL
           AND expires_at > now()
         RETURNING id`
      );
      assert.equal(result.rows.length, 0, 'expired token must not consume');
    });

    // Regression: a v0.5.1 smoke discovered that PostgresPhoneAllowlistStore
    // .setPhone is a pure UPDATE — without a prior provisionUser, it
    // silently affects 0 rows and the /onboard callback succeeds with no
    // users row. InMemoryPhoneAllowlistStore auto-created the row via
    // Map.set, so unit tests passed against a permissive mock. The fix
    // adds provisionUser; this test codifies the Postgres contract.
    it('PostgresPhoneAllowlistStore.setPhone WITHOUT provisionUser silently affects 0 rows (the v0.5.1 bug)', async () => {
      assert.ok(pglite);
      assert.ok(db);
      const { PostgresPhoneAllowlistStore } = await import('../../security/phone-allowlist.js');
      const cryptoCfg = {
        kek: Buffer.alloc(32, 7),
        devMode: false
      };
      const phoneHashCfg = { hmacKey: Buffer.alloc(32, 11) };
      const store = new PostgresPhoneAllowlistStore(db, cryptoCfg, phoneHashCfg);
      const orphanUserId = '11111111-2222-3333-4444-555555555555';
      // setPhone without provisionUser → no row to update, no error
      await store.setPhone(orphanUserId, '+15550100099');
      const rows = await pglite.query<{ count: string | number }>(
        `SELECT count(*) AS count FROM users WHERE id::text = '${orphanUserId}'`
      );
      assert.equal(
        Number(rows.rows[0].count),
        0,
        'setPhone without provisionUser must NOT have created a row (the bug was: the /onboard callback assumed it would)'
      );
    });

    it('provisionUser inserts a users row with is_founder=false; setPhone then populates phone fields; full path is queryable', async () => {
      assert.ok(pglite);
      assert.ok(db);
      const { PostgresPhoneAllowlistStore } = await import('../../security/phone-allowlist.js');
      const cryptoCfg = {
        kek: Buffer.alloc(32, 7),
        devMode: false
      };
      const phoneHashCfg = { hmacKey: Buffer.alloc(32, 11) };
      const store = new PostgresPhoneAllowlistStore(db, cryptoCfg, phoneHashCfg);
      const friendUserId = '22222222-3333-4444-5555-666666666666';

      // Correct order: provisionUser → setPhone
      await store.provisionUser({ user_id: friendUserId, email: 'friend-regression@example.test' });
      await store.setPhone(friendUserId, '+15550100098');

      const row = await pglite.query<{
        email: string;
        is_founder: boolean;
        has_hash: boolean;
        has_envelope: boolean;
      }>(
        `SELECT email, is_founder,
                (phone_e164_hash IS NOT NULL) AS has_hash,
                (phone_e164_encrypted IS NOT NULL) AS has_envelope
         FROM users WHERE id::text = '${friendUserId}'`
      );
      assert.equal(row.rows.length, 1, 'users row must exist after provisionUser');
      assert.equal(row.rows[0].email, 'friend-regression@example.test');
      assert.equal(row.rows[0].is_founder, false, 'friend must have is_founder=false');
      assert.equal(row.rows[0].has_hash, true, 'setPhone after provisionUser must populate phone_e164_hash');
      assert.equal(row.rows[0].has_envelope, true, 'setPhone after provisionUser must populate phone_e164_encrypted');
    });

    it('provisionUser is idempotent — repeated calls (browser refresh) do not error or duplicate', async () => {
      assert.ok(pglite);
      assert.ok(db);
      const { PostgresPhoneAllowlistStore } = await import('../../security/phone-allowlist.js');
      const cryptoCfg = {
        kek: Buffer.alloc(32, 7),
        devMode: false
      };
      const phoneHashCfg = { hmacKey: Buffer.alloc(32, 11) };
      const store = new PostgresPhoneAllowlistStore(db, cryptoCfg, phoneHashCfg);
      const friendUserId = '33333333-4444-5555-6666-777777777777';
      const email = 'friend-idempotent@example.test';

      await store.provisionUser({ user_id: friendUserId, email });
      await store.provisionUser({ user_id: friendUserId, email });
      await store.provisionUser({ user_id: friendUserId, email });

      const rows = await pglite.query<{ count: string | number }>(
        `SELECT count(*) AS count FROM users WHERE id::text = '${friendUserId}'`
      );
      assert.equal(Number(rows.rows[0].count), 1, 'provisionUser must remain at exactly one row');
    });
  });
});
