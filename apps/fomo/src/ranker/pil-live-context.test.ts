// Phase v0.5.12 — buildLivePilContext unit tests (read-side filter).
//
// LOAD-BEARING coverage:
//   C3 — Legacy scope_key='message:<id>' placeholder row produces null
//        PIL context (in-vitro mirror of BB6)
//   C7 — Cross-user contamination via memory_signals (user_id, scope_key)
//        keying — same coverage as buildPilContext since buildLivePilContext
//        delegates to it, but we re-assert here so the read-side wrapper's
//        contract is independently tested.
//   BB6 LOAD-BEARING fixture (legacy placeholder → null context)
//
// Note: the FOMO_PIL_LIVE_ENABLED kill switch is enforced at the call site
// (worker), not inside buildLivePilContext — keeps this function pure for
// testing. BB7 (kill switch off → null) is covered by the index.ts integration
// test below + by the worker-level e2e suite when it lands.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import {
  buildLivePilContext,
  CANONICAL_SCOPE_KEY_REGEX,
  type PilContextDeps
} from './pil-context.ts';

const CANONICAL_HMAC = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee';

function deps(now: () => Date): PilContextDeps {
  return {
    memoryStore: new InMemoryMemorySignalStore(),
    recency_full_days: 90,
    recency_decay_days: 90,
    now
  };
}

describe('CANONICAL_SCOPE_KEY_REGEX', () => {
  it('matches a 32-lowercase-hex string', () => {
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('a'.repeat(32)), true);
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('0123456789abcdef0123456789abcdef'), true);
  });
  it('rejects uppercase hex (canonical shape is lowercase only)', () => {
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('A'.repeat(32)), false);
  });
  it('rejects non-hex characters', () => {
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('g'.repeat(32)), false);
  });
  it('rejects wrong length (31, 33, 64)', () => {
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('a'.repeat(31)), false);
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('a'.repeat(33)), false);
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test('a'.repeat(64)), false);
  });
  it('rejects legacy message:<id> shape', () => {
    assert.equal(
      CANONICAL_SCOPE_KEY_REGEX.test('message:19e92fe1ec00b978'),
      false
    );
  });
  it('rejects empty string', () => {
    assert.equal(CANONICAL_SCOPE_KEY_REGEX.test(''), false);
  });
});

describe('buildLivePilContext — null returns', () => {
  it('returns null when sender_email_hash is null', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    const ctx = await buildLivePilContext('founder', null, d);
    assert.equal(ctx, null);
  });

  it('returns null when sender_email_hash is empty string', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    const ctx = await buildLivePilContext('founder', '', d);
    assert.equal(ctx, null);
  });

  it('BB6 LOAD-BEARING — returns null for a legacy scope_key="message:<id>" even when a row exists at that key', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    const placeholderScope = 'message:19e92fe1ec00b978';
    await d.memoryStore.upsert({
      user_id: 'founder',
      kind: 'sender_suppressed',
      scope_key: placeholderScope,
      detail: {
        suppressed: true,
        set_at: '2026-06-07T00:00:00Z',
        set_by: 'v0.5.10_legacy_path'
      },
      source: 'user_confirmed',
      confidence: 1
    });
    const ctx = await buildLivePilContext('founder', placeholderScope, d);
    assert.equal(
      ctx,
      null,
      'legacy placeholder rows MUST NOT contribute to live PIL context (BB6 LOAD-BEARING)'
    );
  });

  it('returns null for any non-canonical scope_key shape (uppercase hex, wrong length, non-hex chars)', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    // We seed a row at each non-canonical shape so the test would fail if
    // the filter were absent — the inner buildPilContext would otherwise
    // happily project the row.
    for (const badShape of [
      'A'.repeat(32),
      'a'.repeat(31),
      'a'.repeat(33),
      'g'.repeat(32),
      'message:abc',
      'sender:abc',
      ''
    ]) {
      if (badShape !== '') {
        await d.memoryStore.upsert({
          user_id: 'founder',
          kind: 'sender_importance',
          scope_key: badShape,
          detail: { score: 0.5, n_positive_events: 5, n_negative_events: 0, last_updated: '2026-06-07T00:00:00Z' },
          source: 'feedback_derived',
          confidence: 0.6
        });
      }
      const ctx = await buildLivePilContext('founder', badShape, d);
      assert.equal(ctx, null, `non-canonical scope "${badShape}" must produce null`);
    }
  });

  it('returns null when no memory_signals exist for the canonical (user, scope) tuple', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    const ctx = await buildLivePilContext('founder', CANONICAL_HMAC, d);
    assert.equal(ctx, null);
  });
});

describe('buildLivePilContext — canonical row passthrough', () => {
  it('delegates to buildPilContext when scope_key is canonical HMAC + row exists', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    await d.memoryStore.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: CANONICAL_HMAC,
      detail: {
        score: 0.3,
        n_positive_events: 3,
        n_negative_events: 0,
        last_updated: '2026-06-06T00:00:00Z',
        source_surface: 'email_alert',
        source_feedback_event_ids: [1, 2, 3]
      },
      source: 'feedback_derived',
      confidence: 0.6
    });
    const ctx = await buildLivePilContext('founder', CANONICAL_HMAC, d);
    assert.notEqual(ctx, null);
    assert.equal(ctx!.sender_suppressed, false);
    assert.equal(ctx!.sender_importance_n_events, 3);
    // Decay-factor applied = 1.0 since signal is 1d old, within full window
    assert.equal(ctx!.decay_factor_applied, 1.0);
    assert.equal(ctx!.sender_importance_score, 0.3);
  });

  it('preserves suppressed=true when the underlying sender_suppressed row carries detail.suppressed=true', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    await d.memoryStore.upsert({
      user_id: 'founder',
      kind: 'sender_suppressed',
      scope_key: CANONICAL_HMAC,
      detail: {
        suppressed: true,
        set_at: '2026-06-04T00:00:00Z',
        set_by: 'explicit_ignore_sender',
        source_surface: 'email_alert',
        source_feedback_event_ids: [42]
      },
      source: 'user_confirmed',
      confidence: 1
    });
    const ctx = await buildLivePilContext('founder', CANONICAL_HMAC, d);
    assert.notEqual(ctx, null);
    assert.equal(ctx!.sender_suppressed, true);
    assert.equal(ctx!.sender_importance_n_events, 0);
  });
});

describe('buildLivePilContext — BB4 LOAD-BEARING cross-user contamination', () => {
  it('user B asking for scope_key SAME as user A receives null (HMAC user_id keying ensures isolation; even if a DB row were inserted at the literal user A row, the (user_id, scope_key) lookup never reaches it)', async () => {
    const d = deps(() => new Date('2026-06-07T00:00:00Z'));
    await d.memoryStore.upsert({
      user_id: 'userA',
      kind: 'sender_suppressed',
      scope_key: CANONICAL_HMAC,
      detail: { suppressed: true, set_at: '2026-06-07T00:00:00Z', set_by: 'explicit_ignore_sender' },
      source: 'user_confirmed',
      confidence: 1
    });
    const ctxB = await buildLivePilContext('userB', CANONICAL_HMAC, d);
    assert.equal(ctxB, null, 'user B must NEVER read user A row at the same scope_key');
    // sanity: user A still sees their row
    const ctxA = await buildLivePilContext('userA', CANONICAL_HMAC, d);
    assert.notEqual(ctxA, null);
    assert.equal(ctxA!.sender_suppressed, true);
  });
});
