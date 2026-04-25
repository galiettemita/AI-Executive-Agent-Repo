import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { describe, it } from 'node:test';

import { CircuitStore } from './circuit-store.js';

function testStatePath(): string {
  const dir = mkdtempSync(path.join(os.tmpdir(), 'brevio-hands-circuit-store-'));
  return path.join(dir, 'state.json');
}

describe('CircuitStore', () => {
  it('persists circuit state across reloads', () => {
    const statePath = testStatePath();
    try {
      const store = new CircuitStore(statePath);
      assert.equal(store.mode(), 'local_file_snapshot');
      assert.equal(store.snapshotPath(), statePath);

      const initial = store.get('skill.alpha', 3, 100);
      assert.equal(initial.state, 'CLOSED');
      assert.equal(initial.halfOpenRemaining, 3);

      store.update('skill.alpha', 3, (entry) => {
        entry.state = 'OPEN';
        entry.failureCount = 5;
        entry.openedAtMs = 250;
        entry.halfOpenRemaining = 3;
        entry.updatedAtMs = 250;
      }, 250);

      const reloaded = new CircuitStore(statePath);
      const alpha = reloaded.get('skill.alpha', 3, 300);
      assert.equal(alpha.state, 'OPEN');
      assert.equal(alpha.failureCount, 5);
      assert.equal(alpha.openedAtMs, 250);
      assert.equal(alpha.halfOpenRemaining, 3);
      assert.equal(reloaded.size(), 1);
    } finally {
      rmSync(path.dirname(statePath), { recursive: true, force: true });
    }
  });

  it('fails fast on corrupt snapshots', () => {
    const statePath = testStatePath();
    try {
      writeFileSync(statePath, JSON.stringify({ version: 1, circuits: [{ skillId: '', state: 'OPEN' }] }), 'utf8');
      assert.throws(() => new CircuitStore(statePath), /circuit state snapshot is corrupt/);
    } finally {
      rmSync(path.dirname(statePath), { recursive: true, force: true });
    }
  });
});
