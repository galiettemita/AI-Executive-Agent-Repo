import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryToolInvocationStore } from './tool-invocations.ts';

describe('InMemoryToolInvocationStore — write + recent', () => {
  it('records and reads back, newest first', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1',
      tool_id: 'audit.write',
      invocation_id: 'inv-1',
      policy_decision: 'allowed',
      status: 'success',
      latency_ms: 12
    });
    await store.write({
      user_id: 'u1',
      tool_id: 'gmail.read',
      invocation_id: 'inv-2',
      policy_decision: 'not_implemented',
      status: 'denied',
      latency_ms: null
    });
    const out = await store.recent('u1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.tool_id, 'gmail.read');
    assert.equal(out[0]?.policy_decision, 'not_implemented');
    assert.equal(out[1]?.tool_id, 'audit.write');
  });

  it('isolates records per user', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-a',
      policy_decision: 'allowed', status: 'success'
    });
    await store.write({
      user_id: 'u2', tool_id: 'audit.write', invocation_id: 'inv-b',
      policy_decision: 'allowed', status: 'success'
    });
    assert.equal((await store.recent('u1')).length, 1);
    assert.equal((await store.recent('u2')).length, 1);
  });

  it('respects limit parameter', async () => {
    const store = new InMemoryToolInvocationStore();
    for (let i = 0; i < 10; i++) {
      await store.write({
        user_id: 'u1', tool_id: 'audit.write', invocation_id: `inv-${i}`,
        policy_decision: 'allowed', status: 'success'
      });
    }
    const out = await store.recent('u1', 4);
    assert.equal(out.length, 4);
  });

  it('respects capacity (oldest evicted)', async () => {
    const store = new InMemoryToolInvocationStore(3);
    for (let i = 0; i < 5; i++) {
      await store.write({
        user_id: 'u1', tool_id: 'audit.write', invocation_id: `inv-${i}`,
        policy_decision: 'allowed', status: 'success'
      });
    }
    const out = await store.recent('u1');
    assert.equal(out.length, 3);
    assert.deepEqual(out.map((r) => r.invocation_id), ['inv-4', 'inv-3', 'inv-2']);
  });

  it('uses provided occurred_at when given', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success',
      occurred_at: '2026-05-22T00:00:00.000Z'
    });
    const [r] = await store.recent('u1');
    assert.equal(r?.occurred_at, '2026-05-22T00:00:00.000Z');
  });

  it('returned records are frozen', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success'
    });
    const [r] = await store.recent('u1');
    assert.ok(r);
    assert.throws(() => {
      (r as unknown as { status: string }).status = 'mutated';
    });
  });
});

describe('InMemoryToolInvocationStore — privacy: no raw payload content', () => {
  // Load-bearing test. The Permission Gate may know about emails, model
  // prompts, reply text — but the per-dispatch invocation log MUST NOT.
  // Metadata is sanitized through safe-logger redact() on write. The
  // documented schema does not include 'body_plain' / 'body_html' /
  // 'reply_text' / 'prompt' fields, but if a careless caller drops one
  // into metadata, redact() catches the sensitive-key variants.

  it('redacts sensitive keys in metadata before persisting (parallel to audit log)', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1',
      tool_id: 'sendblue.send_user_message',
      invocation_id: 'inv-1',
      policy_decision: 'allowed',
      status: 'success',
      metadata: { access_token: 'plaintext-token', tier_used: 'send' }
    });
    const [r] = await store.recent('u1');
    const meta = r?.metadata as Record<string, unknown>;
    assert.equal(meta.access_token, '<redacted>');
    assert.equal(meta.tier_used, 'send');
  });

  it('the documented record shape contains no field for raw email body / reply text / prompt', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'gmail.read', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success'
    });
    const [r] = await store.recent('u1');
    assert.ok(r);
    // The schema lists exactly these keys. Asserting the shape catches a
    // future contributor accidentally widening the record interface to
    // accept payload content. metadata is the only free-form field, and
    // it is redacted by safe-logger.
    const allowedKeys = new Set([
      'id',
      'occurred_at',
      'user_id',
      'tool_id',
      'invocation_id',
      'policy_decision',
      'status',
      'latency_ms',
      'error_code',
      'error_reason',
      'metadata'
    ]);
    for (const key of Object.keys(r)) {
      assert.ok(
        allowedKeys.has(key),
        `tool_invocations record gained unexpected field '${key}' — check the privacy invariant`
      );
    }
    // Also explicitly assert payload-shaped keys are not present.
    assert.equal('body_plain' in r, false);
    assert.equal('body_html' in r, false);
    assert.equal('reply_text' in r, false);
    assert.equal('prompt' in r, false);
    assert.equal('email_body' in r, false);
  });

  it('null metadata is stored as null (not "{}")', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success'
    });
    const [r] = await store.recent('u1');
    assert.equal(r?.metadata, null);
  });
});

describe('InMemoryToolInvocationStore — counters + lookups', () => {
  it('countByTool totals across calls for that user+tool', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success'
    });
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-2',
      policy_decision: 'allowed', status: 'success'
    });
    await store.write({
      user_id: 'u1', tool_id: 'gmail.read', invocation_id: 'inv-3',
      policy_decision: 'not_implemented', status: 'denied'
    });
    assert.equal(await store.countByTool('u1', 'audit.write'), 2);
    assert.equal(await store.countByTool('u1', 'gmail.read'), 1);
    assert.equal(await store.countByTool('u1', 'unknown.tool'), 0);
    assert.equal(await store.countByTool('u2', 'audit.write'), 0);
  });

  it('countByStatus totals success/failure/denied per user', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 't1', invocation_id: 'inv-1',
      policy_decision: 'allowed', status: 'success'
    });
    await store.write({
      user_id: 'u1', tool_id: 't1', invocation_id: 'inv-2',
      policy_decision: 'allowed', status: 'failure'
    });
    await store.write({
      user_id: 'u1', tool_id: 't1', invocation_id: 'inv-3',
      policy_decision: 'not_implemented', status: 'denied'
    });
    await store.write({
      user_id: 'u1', tool_id: 't1', invocation_id: 'inv-4',
      policy_decision: 'send_disabled', status: 'denied'
    });
    assert.equal(await store.countByStatus('u1', 'success'), 1);
    assert.equal(await store.countByStatus('u1', 'failure'), 1);
    assert.equal(await store.countByStatus('u1', 'denied'), 2);
  });

  it('byInvocationId returns the record or null', async () => {
    const store = new InMemoryToolInvocationStore();
    await store.write({
      user_id: 'u1', tool_id: 'audit.write', invocation_id: 'inv-unique',
      policy_decision: 'allowed', status: 'success'
    });
    const found = await store.byInvocationId('inv-unique');
    assert.ok(found);
    assert.equal(found?.tool_id, 'audit.write');
    assert.equal(await store.byInvocationId('does-not-exist'), null);
  });
});
