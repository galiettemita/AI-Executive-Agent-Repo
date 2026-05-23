import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  ALERT_STATES,
  INITIAL_STATE,
  type AlertState,
  getAllowedTransitions,
  getTerminalStates,
  isAlertState,
  isInvalidTransition,
  isTerminal,
  isValidTransition,
  transition
} from './state-machine.ts';

describe('ALERT_STATES', () => {
  it('declares exactly the 12 v0.1 states', () => {
    assert.equal(ALERT_STATES.length, 12);
    const expected = [
      'detected',
      'ranked',
      'gated_out',
      'queued_for_review',
      'rejected',
      'approved',
      'sent',
      'send_status_unknown',
      'failed',
      'replied',
      'snoozed',
      'ignored'
    ];
    assert.deepEqual([...ALERT_STATES].sort(), [...expected].sort());
  });

  it('INITIAL_STATE is detected', () => {
    assert.equal(INITIAL_STATE, 'detected');
  });
});

describe('isAlertState', () => {
  it('recognizes every declared state', () => {
    for (const s of ALERT_STATES) {
      assert.equal(isAlertState(s), true);
    }
  });

  it('rejects unknown strings and non-strings', () => {
    assert.equal(isAlertState('mystery'), false);
    assert.equal(isAlertState(''), false);
    assert.equal(isAlertState(null), false);
    assert.equal(isAlertState(undefined), false);
    assert.equal(isAlertState(42), false);
    assert.equal(isAlertState({}), false);
  });
});

describe('isTerminal / getTerminalStates', () => {
  it('marks gated_out, rejected, failed, snoozed, ignored as terminal', () => {
    const expected = ['gated_out', 'rejected', 'failed', 'snoozed', 'ignored'];
    for (const t of expected) {
      assert.equal(isTerminal(t as AlertState), true, `expected ${t} to be terminal`);
    }
    assert.deepEqual([...getTerminalStates()].sort(), [...expected].sort());
  });

  it('marks all non-terminal states as non-terminal', () => {
    const nonTerminal = [
      'detected',
      'ranked',
      'queued_for_review',
      'approved',
      'sent',
      'send_status_unknown',
      'replied'
    ];
    for (const s of nonTerminal) {
      assert.equal(isTerminal(s as AlertState), false, `${s} should not be terminal`);
    }
  });
});

describe('isValidTransition — spec coverage', () => {
  const valid: Array<[AlertState, AlertState]> = [
    ['detected', 'ranked'],
    ['detected', 'failed'],
    ['ranked', 'queued_for_review'],
    ['ranked', 'gated_out'],
    ['ranked', 'failed'],
    ['queued_for_review', 'approved'],
    ['queued_for_review', 'rejected'],
    ['queued_for_review', 'failed'],
    ['approved', 'sent'],
    ['approved', 'send_status_unknown'],
    ['approved', 'failed'],
    ['sent', 'replied'],
    ['sent', 'ignored'],
    ['sent', 'send_status_unknown'],
    ['send_status_unknown', 'sent'],
    ['send_status_unknown', 'failed'],
    ['replied', 'snoozed'],
    ['replied', 'ignored']
  ];

  for (const [from, to] of valid) {
    it(`allows ${from} → ${to}`, () => {
      assert.equal(isValidTransition(from, to), true);
    });
  }

  const invalid: Array<[AlertState, AlertState]> = [
    ['detected', 'sent'],
    ['detected', 'approved'],
    ['ranked', 'sent'],
    ['queued_for_review', 'sent'],
    ['approved', 'replied'],
    ['sent', 'approved'],
    ['failed', 'sent'],
    ['ignored', 'sent'],
    ['snoozed', 'replied'],
    ['rejected', 'approved']
  ];

  for (const [from, to] of invalid) {
    it(`rejects ${from} → ${to}`, () => {
      assert.equal(isValidTransition(from, to), false);
    });
  }
});

describe('terminal states have no outgoing transitions', () => {
  for (const t of ['gated_out', 'rejected', 'failed', 'snoozed', 'ignored'] as AlertState[]) {
    it(`${t} has no allowed transitions`, () => {
      assert.equal(getAllowedTransitions(t).length, 0);
    });
    for (const s of ALERT_STATES) {
      it(`${t} cannot transition to ${s}`, () => {
        assert.equal(isValidTransition(t, s), false);
      });
    }
  }
});

describe('transition()', () => {
  it('returns a frozen transition record for a valid move', () => {
    const at = new Date('2026-05-22T18:00:00.000Z');
    const t = transition('detected', 'ranked', 'classifier completed', at);
    assert.equal(isInvalidTransition(t), false);
    if (!isInvalidTransition(t)) {
      assert.equal(t.from, 'detected');
      assert.equal(t.to, 'ranked');
      assert.equal(t.at, '2026-05-22T18:00:00.000Z');
      assert.equal(t.reason, 'classifier completed');
    }
    assert.throws(() => {
      (t as unknown as { reason: string }).reason = 'mutated';
    });
  });

  it('returns InvalidTransition for an illegal move', () => {
    const t = transition('detected', 'sent', 'tried to skip');
    assert.equal(isInvalidTransition(t), true);
    if (isInvalidTransition(t)) {
      assert.equal(t.error, 'invalid_transition');
      assert.equal(t.from, 'detected');
      assert.equal(t.to, 'sent');
      assert.match(t.reason, /not a valid transition/);
      assert.match(t.reason, /allowed: \[ranked, failed\]/);
    }
  });

  it('returns InvalidTransition with terminal-specific message when from-state is terminal', () => {
    const t = transition('failed', 'sent', 'recovery attempt');
    assert.equal(isInvalidTransition(t), true);
    if (isInvalidTransition(t)) {
      assert.match(t.reason, /failed is terminal/);
    }
  });
});

describe('graph connectivity — every non-terminal state is reachable from detected', () => {
  it('detected reaches every non-terminal state via BFS over allowed transitions', () => {
    const visited = new Set<AlertState>();
    const queue: AlertState[] = [INITIAL_STATE];
    while (queue.length > 0) {
      const s = queue.shift() as AlertState;
      if (visited.has(s)) continue;
      visited.add(s);
      for (const next of getAllowedTransitions(s)) {
        if (!visited.has(next)) queue.push(next);
      }
    }
    for (const s of ALERT_STATES) {
      assert.equal(visited.has(s), true, `state ${s} is unreachable from ${INITIAL_STATE}`);
    }
  });
});

describe('graph well-formedness — every transition target is itself a declared state', () => {
  it('no transition points to an unknown state', () => {
    for (const from of ALERT_STATES) {
      for (const to of getAllowedTransitions(from)) {
        assert.ok(isAlertState(to), `${from} → ${to} targets non-declared state`);
      }
    }
  });
});
