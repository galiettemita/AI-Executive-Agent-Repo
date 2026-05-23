import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { type PolicyDecision } from '../core/policy-gate.ts';
import { AuthorizedToolCall, createDispatchTable, type DispatchContext } from './dispatcher.ts';

const TEST_CONTEXT: DispatchContext = Object.freeze({
  user_id: 'u-test',
  invocation_id: 'inv-test'
});

// Helper: synthesize an "allowed" PolicyDecision shape for a tool that
// exists in the registry. Mirrors what decidePolicy() returns on allow.
function allowedDecision(tool_id: string): PolicyDecision {
  return Object.freeze({
    allowed: true,
    code: 'allowed',
    reason: `tool ${tool_id} allowed for test`,
    tool_id,
    user_id: TEST_CONTEXT.user_id
  });
}

function deniedDecision(tool_id: string, code: PolicyDecision['code']): PolicyDecision {
  return Object.freeze({
    allowed: false,
    code,
    reason: `tool ${tool_id} denied for test (${code})`,
    tool_id,
    user_id: TEST_CONTEXT.user_id
  });
}

/* ====================================================================== *
 * AuthorizedToolCall.fromDecision — only allowed decisions mint a call.  *
 * ====================================================================== */

describe('AuthorizedToolCall.fromDecision — authorization gate', () => {
  it('returns an AuthorizedToolCall for an allowed decision on a known tool', () => {
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth, 'fromDecision should produce an AuthorizedToolCall');
    assert.ok(auth instanceof AuthorizedToolCall);
    assert.equal(auth?.tool_id, 'audit.write');
    assert.equal(auth?.user_id, TEST_CONTEXT.user_id);
    assert.ok(auth?.authorized_at);
  });

  it('returns null when decision.allowed is false (any deny code)', () => {
    for (const code of [
      'not_implemented',
      'unknown_tool',
      'send_disabled',
      'auto_send_disabled',
      'consent_missing',
      'oauth_not_connected',
      'policy_check_error',
      'unknown_tier'
    ] as const) {
      const auth = AuthorizedToolCall.fromDecision(deniedDecision('audit.write', code));
      assert.equal(auth, null, `fromDecision should refuse a deny with code='${code}'`);
    }
  });

  it("returns null even when allowed=true but code !== 'allowed' (defense-in-depth)", () => {
    // A malformed decision that says allowed:true but code:'unknown_tool'
    // — the factory still refuses because it checks code, not just allowed.
    const malformed: PolicyDecision = Object.freeze({
      allowed: true,
      code: 'unknown_tool',
      reason: 'malformed',
      tool_id: 'audit.write',
      user_id: TEST_CONTEXT.user_id
    });
    assert.equal(AuthorizedToolCall.fromDecision(malformed), null);
  });

  it('returns null when tool_id is not in the v0.1 registry (defense-in-depth)', () => {
    // Even if the gate ever returned an allow for an unknown tool, the
    // factory refuses.
    const decisionForUnknown: PolicyDecision = Object.freeze({
      allowed: true,
      code: 'allowed',
      reason: 'somehow allowed',
      tool_id: 'booking.flights',
      user_id: TEST_CONTEXT.user_id
    });
    assert.equal(AuthorizedToolCall.fromDecision(decisionForUnknown), null);
  });

  it('returned AuthorizedToolCall is frozen', () => {
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    assert.throws(() => {
      (auth as unknown as { tool_id: string }).tool_id = 'gmail.read';
    });
  });
});

/* ====================================================================== *
 * dispatch.execute — only AuthorizedToolCall instances are executed.     *
 * ====================================================================== */

describe('dispatch.execute — structural authorization (Phase 3A.1)', () => {
  it('executes successfully when given an AuthorizedToolCall from an allowed decision', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async (args: unknown) => ({ echoed: args }));
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const result = await table.execute(auth, { hello: 'world' }, TEST_CONTEXT);
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.deepEqual(result.output, { echoed: { hello: 'world' } });
    }
  });

  it('rejects a forged plain-object AuthorizedToolCall with code unauthorized (runtime guard)', async () => {
    // TypeScript prevents `dispatch.execute('gmail.read', ...)` at compile
    // time. But a determined caller could write
    //   const forged = { tool_id: 'audit.write', user_id: 'u', authorized_at: '' } as unknown as AuthorizedToolCall;
    // The runtime instanceof check catches that.
    const table = createDispatchTable();
    table.register('audit.write', async () => 'should not run');
    const forged = {
      tool_id: 'audit.write',
      user_id: TEST_CONTEXT.user_id,
      authorized_at: new Date().toISOString()
    } as unknown as AuthorizedToolCall;
    const result = await table.execute(forged, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unauthorized');
      assert.match(result.reason, /AuthorizedToolCall.fromDecision/);
    }
  });

  it('rejects null cast as AuthorizedToolCall (runtime guard)', async () => {
    const table = createDispatchTable();
    const result = await table.execute(null as unknown as AuthorizedToolCall, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unauthorized');
    }
  });

  it('rejects undefined cast as AuthorizedToolCall (runtime guard)', async () => {
    const table = createDispatchTable();
    const result = await table.execute(undefined as unknown as AuthorizedToolCall, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unauthorized');
    }
  });

  it('a denied gate decision cannot be turned into an executable call', async () => {
    // Full end-to-end proof: gate denies → fromDecision returns null →
    // dispatch.execute is structurally unreachable (TS would not compile
    // dispatch.execute(null, ...) without the runtime guard catching it).
    const table = createDispatchTable();
    table.register('audit.write', async () => 'should not run');
    const denied = deniedDecision('audit.write', 'not_implemented');
    const auth = AuthorizedToolCall.fromDecision(denied);
    assert.equal(auth, null);
    // The only way to PROCEED past this point is to bypass TS — which the
    // runtime guard catches:
    const result = await table.execute(auth as unknown as AuthorizedToolCall, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unauthorized');
    }
  });
});

/* ====================================================================== *
 * dispatch.execute — downstream fail-closed paths (unchanged behaviors)  *
 * ====================================================================== */

describe('dispatch.execute — downstream fail-closed paths', () => {
  it('denies no_executor_for_tool when authorized but no executor is registered', async () => {
    const table = createDispatchTable();
    // gmail.read would never produce an allowed decision in practice
    // (Phase 3A keeps it declared), but the test synthesizes one to prove
    // that even with valid authorization, dispatch still refuses when no
    // executor is bound.
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('gmail.read'));
    assert.ok(auth);
    const result = await table.execute(auth, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'no_executor_for_tool');
      assert.match(result.reason, /no executor registered for tool 'gmail\.read'/);
    }
  });

  it('denies executor_error when the executor throws', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => {
      throw new Error('store unreachable');
    });
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const result = await table.execute(auth, {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'executor_error');
      assert.match(result.reason, /store unreachable/);
    }
  });

  it('reports latency_ms for both success and failure paths', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => {
      await new Promise((resolve) => setTimeout(resolve, 5));
    });
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const ok = await table.execute(auth, {}, TEST_CONTEXT);
    assert.ok(ok.ok && ok.latency_ms >= 5);
    const unauthorized = await table.execute(
      {} as unknown as AuthorizedToolCall,
      {},
      TEST_CONTEXT
    );
    assert.equal(unauthorized.ok, false);
    assert.ok(typeof (unauthorized as { latency_ms: number }).latency_ms === 'number');
  });
});

describe('dispatch.execute — success path identity (args + context passthrough)', () => {
  it('returns the executor output as ok=true', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async (args: unknown) => {
      return { echoed: args };
    });
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const result = await table.execute(auth, { hello: 'world' }, TEST_CONTEXT);
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.deepEqual(result.output, { echoed: { hello: 'world' } });
    }
  });

  it('passes args + context through to the executor unchanged', async () => {
    const table = createDispatchTable();
    let receivedArgs: unknown = null;
    let receivedContext: DispatchContext | null = null;
    table.register('feedback.write', async (args, context) => {
      receivedArgs = args;
      receivedContext = context;
    });
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('feedback.write'));
    assert.ok(auth);
    await table.execute(
      auth,
      { kind: 'founder_approved' },
      { user_id: 'u1', invocation_id: 'inv-42' }
    );
    assert.deepEqual(receivedArgs, { kind: 'founder_approved' });
    assert.deepEqual(receivedContext, { user_id: 'u1', invocation_id: 'inv-42' });
  });

  it('returns result objects that are frozen', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => 'output');
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const result = await table.execute(auth, {}, TEST_CONTEXT);
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = false;
    });
  });
});

describe('dispatch — introspection', () => {
  it('hasExecutor reports registered tools only', () => {
    const table = createDispatchTable();
    assert.equal(table.hasExecutor('audit.write'), false);
    table.register('audit.write', async () => undefined);
    assert.equal(table.hasExecutor('audit.write'), true);
    assert.equal(table.hasExecutor('gmail.read'), false);
    assert.equal(table.hasExecutor('booking.flights'), false);
  });

  it('registeredToolIds returns the bound set, frozen', () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => undefined);
    table.register('feedback.write', async () => undefined);
    const ids = table.registeredToolIds();
    assert.deepEqual([...ids].sort(), ['audit.write', 'feedback.write']);
    assert.throws(() => {
      (ids as unknown as string[]).push('mutated');
    });
  });

  it('register replaces a prior executor for the same tool id (last write wins)', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => 'first');
    table.register('audit.write', async () => 'second');
    const auth = AuthorizedToolCall.fromDecision(allowedDecision('audit.write'));
    assert.ok(auth);
    const result = await table.execute<string>(auth, {}, TEST_CONTEXT);
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.output, 'second');
    }
  });
});
