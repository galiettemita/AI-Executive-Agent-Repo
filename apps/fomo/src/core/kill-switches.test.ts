import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from './kill-switches.ts';

describe('loadKillSwitches — safe defaults', () => {
  it('returns all-disabled when env is empty', () => {
    const s = loadKillSwitches({});
    assert.equal(s.send_enabled, false);
    assert.equal(s.auto_send_enabled, false);
    assert.equal(s.friend_beta_enabled, false);
    assert.equal(s.polling_enabled, false);
    assert.equal(s.max_users, 1);
    assert.equal(s.polling_interval_ms, 60_000);
    assert.equal(s.polling_max_cycles, null);
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
      FOMO_FRIEND_BETA_ENABLED: '1',
      FOMO_GMAIL_POLLING_ENABLED: 'true'
    });
    assert.equal(s.send_enabled, true);
    assert.equal(s.auto_send_enabled, false);
    assert.equal(s.friend_beta_enabled, true);
    assert.equal(s.polling_enabled, true);
  });

  it('FOMO_GMAIL_POLLING_ENABLED is strict opt-in', () => {
    assert.equal(loadKillSwitches({}).polling_enabled, false);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_ENABLED: '1' }).polling_enabled, true);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_ENABLED: 'TRUE' }).polling_enabled, true);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_ENABLED: 'yes' }).polling_enabled, false);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_ENABLED: '' }).polling_enabled, false);
  });
});

describe('loadKillSwitches — FOMO_GMAIL_POLLING_MAX_CYCLES (Phase 3B.3)', () => {
  it('returns null when unset (unbounded)', () => {
    assert.equal(loadKillSwitches({}).polling_max_cycles, null);
  });

  it('accepts positive integers', () => {
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '1' }).polling_max_cycles, 1);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '3' }).polling_max_cycles, 3);
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '  10  ' }).polling_max_cycles,
      10
    );
  });

  it('returns null on invalid values (does not throw, does not fall back to a number)', () => {
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '' }).polling_max_cycles, null);
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: 'abc' }).polling_max_cycles,
      null
    );
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '0' }).polling_max_cycles, null);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '-5' }).polling_max_cycles, null);
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '3.7' }).polling_max_cycles,
      null
    );
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_MAX_CYCLES: '1e3' }).polling_max_cycles,
      null
    );
  });
});

describe('loadKillSwitches — FOMO_GMAIL_POLLING_INTERVAL_MS', () => {
  it('accepts positive integers', () => {
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '30000' }).polling_interval_ms,
      30_000
    );
    assert.equal(
      loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '  120000  ' }).polling_interval_ms,
      120_000
    );
  });

  it('falls back to default 60_000 on invalid values', () => {
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '' }).polling_interval_ms, 60_000);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: 'abc' }).polling_interval_ms, 60_000);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '0' }).polling_interval_ms, 60_000);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '-5' }).polling_interval_ms, 60_000);
    assert.equal(loadKillSwitches({ FOMO_GMAIL_POLLING_INTERVAL_MS: '3.7' }).polling_interval_ms, 60_000);
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
