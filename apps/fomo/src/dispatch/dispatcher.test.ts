import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { createDispatchTable, type DispatchContext } from './dispatcher.ts';

const TEST_CONTEXT: DispatchContext = Object.freeze({
  user_id: 'u-test',
  invocation_id: 'inv-test'
});

describe('createDispatchTable — fail-closed paths', () => {
  it('denies unknown_tool when the id is not in the registry', async () => {
    const table = createDispatchTable();
    const result = await table.execute('booking.flights', {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unknown_tool');
      assert.match(result.reason, /not in the v0\.1 registry/);
    }
  });

  it('denies no_executor_for_tool when the tool is registered but no executor is bound', async () => {
    const table = createDispatchTable();
    // gmail.read is a real ToolId, but Phase 3A does not bind an executor.
    const result = await table.execute('gmail.read', {}, TEST_CONTEXT);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'no_executor_for_tool');
      assert.match(result.reason, /no executor registered for tool 'gmail\.read'/);
    }
  });

  it('denies executor_error when the executor throws (catches the exception)', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => {
      throw new Error('store unreachable');
    });
    const result = await table.execute('audit.write', {}, TEST_CONTEXT);
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
    const ok = await table.execute('audit.write', {}, TEST_CONTEXT);
    assert.ok(ok.ok && ok.latency_ms >= 5);
    const unknown = await table.execute('booking.flights', {}, TEST_CONTEXT);
    assert.equal(unknown.ok, false);
    assert.ok(typeof (unknown as { latency_ms: number }).latency_ms === 'number');
  });
});

describe('createDispatchTable — success path', () => {
  it('returns the executor output as ok=true', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async (args: unknown) => {
      return { echoed: args };
    });
    const result = await table.execute('audit.write', { hello: 'world' }, TEST_CONTEXT);
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
    await table.execute(
      'feedback.write',
      { kind: 'founder_approved' },
      { user_id: 'u1', invocation_id: 'inv-42' }
    );
    assert.deepEqual(receivedArgs, { kind: 'founder_approved' });
    assert.deepEqual(receivedContext, { user_id: 'u1', invocation_id: 'inv-42' });
  });

  it('returns result objects that are frozen', async () => {
    const table = createDispatchTable();
    table.register('audit.write', async () => 'output');
    const result = await table.execute('audit.write', {}, TEST_CONTEXT);
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = false;
    });
  });
});

describe('createDispatchTable — introspection', () => {
  it('hasExecutor reports registered tools only', () => {
    const table = createDispatchTable();
    assert.equal(table.hasExecutor('audit.write'), false);
    table.register('audit.write', async () => undefined);
    assert.equal(table.hasExecutor('audit.write'), true);
    assert.equal(table.hasExecutor('gmail.read'), false);
    // hasExecutor is false for unknown ids too (not in registry at all).
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
    const result = await table.execute<string>('audit.write', {}, TEST_CONTEXT);
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.output, 'second');
    }
  });
});
