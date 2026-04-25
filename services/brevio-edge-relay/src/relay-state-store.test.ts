import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { describe, it } from 'node:test';

import { RelayStateStore } from './relay-state-store.js';

function testStorePath(name: string): string {
  const dir = mkdtempSync(path.join(os.tmpdir(), `brevio-edge-relay-state-${name}-`));
  return path.join(dir, 'state.json');
}

describe('RelayStateStore', () => {
  it('persists queues and marks loaded sessions disconnected on restart', () => {
    const storePath = testStorePath('persist');
    try {
      const store = new RelayStateStore(storePath);
      store.upsertSession('user-1:device-1', {
        userId: 'user-1',
        deviceId: 'device-1',
        deviceName: 'MacBook Pro',
        certFingerprint: 'fingerprint-1',
        connectedAt: 100,
        lastSeenAt: 120,
        supportedSkills: ['skill.alpha'],
        authBound: true,
        allowedSkills: ['skill.alpha'],
        connected: true
      });
      store.enqueueExecution(
        'user-1:device-1',
        {
          requestId: 'req-1',
          userId: 'user-1',
          deviceId: 'device-1',
          skillId: 'skill.alpha',
          protectedInput: {
            alg: 'aes-256-gcm',
            nonce: 'nonce',
            ciphertext: 'ciphertext'
          },
          queuedAt: 150
        },
        10,
        60_000,
        150
      );

      const reloaded = new RelayStateStore(storePath);
      assert.equal(reloaded.connectedSessionCount(), 0);
      assert.equal(reloaded.trackedSessionCount(), 1);
      const queue = reloaded.queueFor('user-1:device-1', 160, 60_000);
      assert.equal(queue.length, 1);
      assert.equal(queue[0]?.requestId, 'req-1');
    } finally {
      rmSync(path.dirname(storePath), { recursive: true, force: true });
    }
  });

  it('prunes expired queue items and enforces per-device caps', () => {
    const storePath = testStorePath('queue-cap');
    try {
      const store = new RelayStateStore(storePath);
      for (let index = 0; index < 4; index += 1) {
        store.enqueueExecution(
          'user-1:device-1',
          {
            requestId: `req-${index + 1}`,
            userId: 'user-1',
            deviceId: 'device-1',
            skillId: 'skill.alpha',
            protectedInput: {
              alg: 'aes-256-gcm',
              nonce: `nonce-${index}`,
              ciphertext: `ciphertext-${index}`
            },
            queuedAt: 100 + (index * 10)
          },
          2,
          30,
          131
        );
      }

      const queue = store.takeQueue('user-1:device-1', 131, 30);
      assert.deepEqual(
        queue.map((item) => item.requestId),
        ['req-3', 'req-4']
      );
    } finally {
      rmSync(path.dirname(storePath), { recursive: true, force: true });
    }
  });

  it('fails fast when the persisted relay snapshot is corrupt', () => {
    const storePath = testStorePath('corrupt');
    try {
      writeFileSync(storePath, JSON.stringify({ version: 1, sessions: [{ key: 'bad', session: { userId: '' } }], offlineQueues: [] }), 'utf8');
      assert.throws(() => new RelayStateStore(storePath), /relay state snapshot is corrupt/);
    } finally {
      rmSync(path.dirname(storePath), { recursive: true, force: true });
    }
  });
});
