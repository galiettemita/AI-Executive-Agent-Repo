import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import { type PolicyDecision } from '../core/policy-gate.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';

import {
  AuthorizedToolCall,
  createDispatchTable,
  type DispatchContext
} from './dispatcher.ts';
import {
  auditWriteExecutor,
  feedbackWriteExecutor,
  memorySignalUpsertExecutor,
  wireInternalExecutors
} from './internal-executors.ts';

const CONTEXT: DispatchContext = Object.freeze({ user_id: 'u-exec', invocation_id: 'inv-1' });

// Test helper: synthesize an allowed PolicyDecision and turn it into an
// AuthorizedToolCall. Phase 3A.1 makes this the only way to call
// dispatch.execute. Real callers go through decidePolicy() — these tests
// short-circuit because they're exercising the executor, not the gate.
function authorize(tool_id: string): AuthorizedToolCall {
  const decision: PolicyDecision = Object.freeze({
    allowed: true,
    code: 'allowed',
    reason: `test allowed ${tool_id}`,
    tool_id,
    user_id: CONTEXT.user_id
  });
  const auth = AuthorizedToolCall.fromDecision(decision);
  if (!auth) {
    throw new Error(`test setup: could not authorize ${tool_id}`);
  }
  return auth;
}

describe('auditWriteExecutor', () => {
  it('writes to the audit store with context.user_id as actor by default', async () => {
    const store = new InMemoryAuditStore();
    const table = createDispatchTable();
    table.register('audit.write', auditWriteExecutor(store));

    const result = await table.execute(
      authorize('audit.write'),
      { action: 'session.created', target: 'session:1', detail: { source: 'test' } },
      CONTEXT
    );
    assert.equal(result.ok, true);
    const entries = await store.recent(CONTEXT.user_id);
    assert.equal(entries.length, 1);
    assert.equal(entries[0]?.action, 'session.created');
    assert.equal(entries[0]?.actor_user_id, CONTEXT.user_id);
    assert.equal(entries[0]?.target, 'session:1');
    assert.equal(entries[0]?.result, 'success');
  });

  it('honors explicit actor_user_id override (null = system actor)', async () => {
    const store = new InMemoryAuditStore();
    const table = createDispatchTable();
    table.register('audit.write', auditWriteExecutor(store));

    await table.execute(
      authorize('audit.write'),
      { action: 'session.created', actor_user_id: null },
      CONTEXT
    );
    assert.equal((await store.recent(CONTEXT.user_id)).length, 0);
  });

  it('defaults result to success when not given', async () => {
    const store = new InMemoryAuditStore();
    const table = createDispatchTable();
    table.register('audit.write', auditWriteExecutor(store));
    await table.execute(authorize('audit.write'), { action: 'session.created' }, CONTEXT);
    const [entry] = await store.recent(CONTEXT.user_id);
    assert.equal(entry?.result, 'success');
  });
});

describe('feedbackWriteExecutor', () => {
  it('writes a feedback event scoped to context.user_id', async () => {
    const store = new InMemoryFeedbackStore();
    const table = createDispatchTable();
    table.register('feedback.write', feedbackWriteExecutor(store));

    const result = await table.execute(
      authorize('feedback.write'),
      {
        alert_id: 'a-1',
        sender_email: 's@x',
        kind: 'founder_approved',
        detail: { score: 0.9 }
      },
      CONTEXT
    );
    assert.equal(result.ok, true);
    const events = await store.recent(CONTEXT.user_id);
    assert.equal(events.length, 1);
    assert.equal(events[0]?.kind, 'founder_approved');
    assert.equal(events[0]?.alert_id, 'a-1');
    assert.equal(events[0]?.user_id, CONTEXT.user_id);
  });

  it('ignores any user_id in args — context.user_id is authoritative', async () => {
    const store = new InMemoryFeedbackStore();
    const table = createDispatchTable();
    table.register('feedback.write', feedbackWriteExecutor(store));

    // Caller tries to spoof user_id via args; the executor reads from
    // context only, so the spoof has no effect.
    await table.execute(
      authorize('feedback.write'),
      {
        user_id: 'u-spoof',
        alert_id: 'a-1',
        sender_email: null,
        kind: 'user_ignored'
      } as unknown as Parameters<typeof feedbackWriteExecutor>[0] extends infer T ? T : never,
      CONTEXT
    );
    assert.equal((await store.recent('u-spoof')).length, 0);
    assert.equal((await store.recent(CONTEXT.user_id)).length, 1);
  });
});

describe('memorySignalUpsertExecutor', () => {
  it('upserts a signal scoped to context.user_id', async () => {
    const store = new InMemoryMemorySignalStore();
    const table = createDispatchTable();
    table.register('memory_signal.write', memorySignalUpsertExecutor(store));

    const result = await table.execute(
      authorize('memory_signal.write'),
      {
        kind: 'sender_importance',
        scope_key: 's@x',
        detail: { importance: 'high' },
        source: 'user_confirmed'
      },
      CONTEXT
    );
    assert.equal(result.ok, true);
    const signal = await store.get(CONTEXT.user_id, 'sender_importance', 's@x');
    assert.ok(signal);
    assert.equal((signal?.detail as Record<string, unknown>).importance, 'high');
    assert.equal(signal?.source, 'user_confirmed');
  });

  it('upserts replace prior signal at same (user, kind, scope_key)', async () => {
    const store = new InMemoryMemorySignalStore();
    const table = createDispatchTable();
    table.register('memory_signal.write', memorySignalUpsertExecutor(store));

    await table.execute(
      authorize('memory_signal.write'),
      {
        kind: 'sender_importance',
        scope_key: 's@x',
        detail: { importance: 'low' },
        source: 'inferred'
      },
      CONTEXT
    );
    await table.execute(
      authorize('memory_signal.write'),
      {
        kind: 'sender_importance',
        scope_key: 's@x',
        detail: { importance: 'high' },
        source: 'user_confirmed'
      },
      CONTEXT
    );
    const signal = await store.get(CONTEXT.user_id, 'sender_importance', 's@x');
    assert.equal((signal?.detail as Record<string, unknown>).importance, 'high');
  });
});

describe('wireInternalExecutors', () => {
  it('registers all three internal-capability executors in one call', () => {
    const table = createDispatchTable();
    wireInternalExecutors(table, {
      audit: new InMemoryAuditStore(),
      feedback: new InMemoryFeedbackStore(),
      memory: new InMemoryMemorySignalStore()
    });
    assert.deepEqual(
      [...table.registeredToolIds()].sort(),
      ['audit.write', 'feedback.write', 'memory_signal.write']
    );
  });

  it('end-to-end: a wired dispatch executes each of the three writers and reads back', async () => {
    const auditStore = new InMemoryAuditStore();
    const feedbackStore = new InMemoryFeedbackStore();
    const memoryStore = new InMemoryMemorySignalStore();
    const table = createDispatchTable();
    wireInternalExecutors(table, {
      audit: auditStore,
      feedback: feedbackStore,
      memory: memoryStore
    });

    await table.execute(
      authorize('audit.write'),
      { action: 'session.created', target: 'session:test' },
      CONTEXT
    );
    await table.execute(
      authorize('feedback.write'),
      { alert_id: 'a-1', sender_email: null, kind: 'founder_approved' },
      CONTEXT
    );
    await table.execute(
      authorize('memory_signal.write'),
      { kind: 'quietness_preference', scope_key: null, detail: { max_per_day: 5 }, source: 'user_confirmed' },
      CONTEXT
    );

    assert.equal((await auditStore.recent(CONTEXT.user_id)).length, 1);
    assert.equal((await feedbackStore.recent(CONTEXT.user_id)).length, 1);
    assert.ok(await memoryStore.get(CONTEXT.user_id, 'quietness_preference'));
  });

  it('non-wired tools (gmail.read, sendblue, slack) return no_executor_for_tool', async () => {
    const table = createDispatchTable();
    wireInternalExecutors(table, {
      audit: new InMemoryAuditStore(),
      feedback: new InMemoryFeedbackStore(),
      memory: new InMemoryMemorySignalStore()
    });
    // These three tools are still 'declared' in Phase 3A so the gate
    // would deny in practice. For the executor-test perspective we
    // synthesize an authorized call to prove that even with valid
    // authorization, dispatch still refuses when no executor is bound.
    for (const externalTool of ['gmail.read', 'sendblue.send_user_message', 'slack.founder_review']) {
      const result = await table.execute(authorize(externalTool), {}, CONTEXT);
      assert.equal(result.ok, false);
      if (!result.ok) {
        assert.equal(result.code, 'no_executor_for_tool');
      }
    }
  });
});
