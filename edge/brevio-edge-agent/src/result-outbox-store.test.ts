import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { ResultOutboxStore } from './result-outbox-store.js';

function testPath(name: string): string {
  return path.join(mkdtempSync(path.join(tmpdir(), 'brevio-edge-outbox-')), `${name}.json`);
}

describe('ResultOutboxStore', () => {
  it('persists pending results until the relay acknowledges them', () => {
    const store = new ResultOutboxStore(testPath('outbox'));
    store.enqueue({
      requestId: 'req-1',
      resultReceiptId: 'result-1',
      queuedAt: 100,
      result: {
        type: 'skill_result',
        request_id: 'req-1',
        skill_id: 'voice-wake-say',
        status: 'SUCCESS',
        latency_ms: 12,
        dispatch_receipt_id: 'dispatch-1',
        result_receipt_id: 'result-1'
      }
    });

    assert.equal(store.pending(120, 1000).length, 1);
    store.markSent('result-1', 130);
    assert.equal(store.pending(140, 1000)[0]?.sentAt, 130);
    store.markAcked('result-1');
    assert.equal(store.pending(150, 1000).length, 0);
  });
});
