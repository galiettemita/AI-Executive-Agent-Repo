import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from './kill-switches.ts';

describe('loadKillSwitches — safe defaults', () => {
  it('returns all-disabled when env is empty', () => {
    const s = loadKillSwitches({});
    assert.equal(s.send_enabled, false);
    assert.equal(s.auto_send_enabled, false);
    assert.equal(s.friend_beta_enabled, false);
    assert.equal(s.max_users, 1);
  });

  it('SAFE_DEFAULT_KILL_SWITCHES matches the empty-env result', () => {
    const live = loadKillSwitches({});
    assert.deepEqual({ ...live }, { ...SAFE_DEFAULT_KILL_SWITCHES });
  });
});

describe('loadKillSwitches — boolean parsing is strict opt-in', () => {
  for (const value of ['true', 'TRUE', 'True', '1', ' true ']) {
    it(`treats ${JSON.stringify(value)} as true`, () => {
      const s = loadKillSwitches({ FOMO_SEND_ENABLED: value });
      assert.equal(s.send_enabled, true);
    });
  }

  for (const value of ['false', 'FALSE', '0', '', 'yes', 'on', 'enabled', '2', 'truthy', 'TRUE\n garbage']) {
    it(`treats ${JSON.stringify(value)} as false (strict opt-in)`, () => {
      const s = loadKillSwitches({ FOMO_SEND_ENABLED: value });
      assert.equal(s.send_enabled, false);
    });
  }

  it('each flag parses independently', () => {
    const s = loadKillSwitches({
      FOMO_SEND_ENABLED: 'true',
      FOMO_AUTO_SEND_ENABLED: 'false',
      FOMO_FRIEND_BETA_ENABLED: '1'
    });
    assert.equal(s.send_enabled, true);
    assert.equal(s.auto_send_enabled, false);
    assert.equal(s.friend_beta_enabled, true);
  });
});

describe('loadKillSwitches — FOMO_MAX_USERS', () => {
  it('accepts positive integers', () => {
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '5' }).max_users, 5);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '  20  ' }).max_users, 20);
  });

  it('falls back to default on invalid values (does not throw)', () => {
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '' }).max_users, 1);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: 'abc' }).max_users, 1);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '0' }).max_users, 1);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '-5' }).max_users, 1);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '3.7' }).max_users, 1);
    assert.equal(loadKillSwitches({ FOMO_MAX_USERS: '1e3' }).max_users, 1); // not an integer per Number.isInteger
  });
});

describe('loadKillSwitches — immutability', () => {
  it('returned object is frozen', () => {
    const s = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    assert.throws(() => {
      (s as unknown as { send_enabled: boolean }).send_enabled = false;
    });
  });

  it('SAFE_DEFAULT_KILL_SWITCHES is frozen', () => {
    assert.throws(() => {
      (SAFE_DEFAULT_KILL_SWITCHES as unknown as { max_users: number }).max_users = 999;
    });
  });
});

describe('loadKillSwitches — defaults to process.env when no arg given', () => {
  it('reads from process.env', () => {
    const prev = process.env.FOMO_SEND_ENABLED;
    try {
      process.env.FOMO_SEND_ENABLED = 'true';
      assert.equal(loadKillSwitches().send_enabled, true);
    } finally {
      if (prev === undefined) delete process.env.FOMO_SEND_ENABLED;
      else process.env.FOMO_SEND_ENABLED = prev;
    }
  });
});
