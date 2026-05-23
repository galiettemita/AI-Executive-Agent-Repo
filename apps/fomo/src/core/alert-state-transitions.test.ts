import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAlertStateTransitionStore } from './alert-state-transitions.ts';

describe('InMemoryAlertStateTransitionStore — write + read', () => {
  it('records and reads back transitions for an alert in insertion order', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'classifier completed'
    });
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'ranked', to_state: 'queued_for_review', reason: 'score 0.91'
    });
    const out = await store.forAlert('a1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.to_state, 'ranked');
    assert.equal(out[1]?.to_state, 'queued_for_review');
  });

  it('forAlert returns only transitions for the requested alert_id', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'x'
    });
    await store.write({
      alert_id: 'a2', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'y'
    });
    assert.equal((await store.forAlert('a1')).length, 1);
    assert.equal((await store.forAlert('a2')).length, 1);
    assert.equal((await store.forAlert('a3')).length, 0);
  });

  it('recentForUser returns newest first', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'first'
    });
    await store.write({
      alert_id: 'a2', user_id: 'u1',
      from_state: 'detected', to_state: 'gated_out', reason: 'second'
    });
    const out = await store.recentForUser('u1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.alert_id, 'a2');
    assert.equal(out[1]?.alert_id, 'a1');
  });

  it('isolates transitions per user (recentForUser filters by user_id)', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'x'
    });
    await store.write({
      alert_id: 'a2', user_id: 'u2',
      from_state: 'detected', to_state: 'ranked', reason: 'y'
    });
    assert.equal((await store.recentForUser('u1')).length, 1);
    assert.equal((await store.recentForUser('u2')).length, 1);
  });

  it('respects limit on recentForUser', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    for (let i = 0; i < 8; i++) {
      await store.write({
        alert_id: `a${i}`, user_id: 'u1',
        from_state: 'detected', to_state: 'ranked', reason: `iter ${i}`
      });
    }
    assert.equal((await store.recentForUser('u1', 3)).length, 3);
  });
});

describe('InMemoryAlertStateTransitionStore — currentState', () => {
  it('returns the most recent to_state for the alert', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'x'
    });
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'ranked', to_state: 'queued_for_review', reason: 'y'
    });
    assert.equal(await store.currentState('a1'), 'queued_for_review');
  });

  it('returns null when no transitions have been recorded for the alert', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    assert.equal(await store.currentState('unknown-alert'), null);
  });
});

describe('InMemoryAlertStateTransitionStore — validation', () => {
  it('throws on unknown from_state', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await assert.rejects(
      store.write({
        alert_id: 'a1', user_id: 'u1',
        from_state: 'mystery' as never, to_state: 'ranked', reason: 'x'
      }),
      /unknown from_state/
    );
  });

  it('throws on unknown to_state', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await assert.rejects(
      store.write({
        alert_id: 'a1', user_id: 'u1',
        from_state: 'detected', to_state: 'mystery' as never, reason: 'x'
      }),
      /unknown to_state/
    );
  });
});

describe('InMemoryAlertStateTransitionStore — capacity + immutability', () => {
  it('respects capacity (oldest evicted)', async () => {
    const store = new InMemoryAlertStateTransitionStore(3);
    for (let i = 0; i < 5; i++) {
      await store.write({
        alert_id: `a${i}`, user_id: 'u1',
        from_state: 'detected', to_state: 'ranked', reason: `iter ${i}`
      });
    }
    const out = await store.recentForUser('u1');
    assert.equal(out.length, 3);
  });

  it('returned records are frozen', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({
      alert_id: 'a1', user_id: 'u1',
      from_state: 'detected', to_state: 'ranked', reason: 'x'
    });
    const [r] = await store.recentForUser('u1');
    assert.ok(r);
    assert.throws(() => {
      (r as unknown as { reason: string }).reason = 'mutated';
    });
  });
});
