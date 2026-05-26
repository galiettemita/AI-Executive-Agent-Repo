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

describe('InMemoryAlertStateTransitionStore — findAlertIdsInState (Phase 3E.1)', () => {
  it('returns alerts whose LATEST transition lands them in the requested state', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    // a1: detected → ranked → queued_for_review → approved  (CURRENT: approved)
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'founder approved' });
    // a2: detected → ranked → queued_for_review  (CURRENT: queued_for_review)
    await store.write({ alert_id: 'a2', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    // a3: detected → ranked → queued_for_review → approved → sent  (CURRENT: sent)
    await store.write({ alert_id: 'a3', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a3', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a3', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'founder approved' });
    await store.write({ alert_id: 'a3', user_id: 'u1', from_state: 'approved', to_state: 'sent', reason: 'sendblue 2xx' });

    const approved = await store.findAlertIdsInState('u1', 'approved', 50);
    assert.deepEqual([...approved], ['a1']);

    const queued = await store.findAlertIdsInState('u1', 'queued_for_review', 50);
    assert.deepEqual([...queued], ['a2']);

    const sent = await store.findAlertIdsInState('u1', 'sent', 50);
    assert.deepEqual([...sent], ['a3']);
  });

  it('filters by user_id (one user does not see anothers approved alerts)', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u2', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u2', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u2', from_state: 'queued_for_review', to_state: 'approved', reason: 'x' });

    assert.deepEqual([...(await store.findAlertIdsInState('u1', 'approved', 50))], ['a1']);
    assert.deepEqual([...(await store.findAlertIdsInState('u2', 'approved', 50))], ['a2']);
    assert.deepEqual([...(await store.findAlertIdsInState('u3', 'approved', 50))], []);
  });

  it('orders oldest-first by the transition that moved the alert into the requested state', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    // a1 approved first, then a2.
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a2', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'first' });
    await store.write({ alert_id: 'a2', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'second' });

    const approved = await store.findAlertIdsInState('u1', 'approved', 50);
    assert.deepEqual([...approved], ['a1', 'a2']);
  });

  it('respects the limit', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    for (let i = 0; i < 5; i++) {
      await store.write({ alert_id: `a${i}`, user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
      await store.write({ alert_id: `a${i}`, user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
      await store.write({ alert_id: `a${i}`, user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'x' });
    }
    const approved = await store.findAlertIdsInState('u1', 'approved', 2);
    assert.equal(approved.length, 2);
  });

  it('returns empty for non-positive or non-integer limit', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'detected', to_state: 'ranked', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'ranked', to_state: 'queued_for_review', reason: 'x' });
    await store.write({ alert_id: 'a1', user_id: 'u1', from_state: 'queued_for_review', to_state: 'approved', reason: 'x' });
    assert.equal((await store.findAlertIdsInState('u1', 'approved', 0)).length, 0);
    assert.equal((await store.findAlertIdsInState('u1', 'approved', -1)).length, 0);
    assert.equal((await store.findAlertIdsInState('u1', 'approved', 1.5)).length, 0);
  });

  it('returns empty when no alert is currently in the requested state', async () => {
    const store = new InMemoryAlertStateTransitionStore();
    assert.equal((await store.findAlertIdsInState('u1', 'approved', 10)).length, 0);
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
