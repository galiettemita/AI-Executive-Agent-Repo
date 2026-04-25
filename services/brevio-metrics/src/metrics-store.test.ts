import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { MetricsStore } from './metrics-store.js';

function testStatePath(name: string): string {
  return path.join(mkdtempSync(path.join(tmpdir(), 'brevio-metrics-')), `${name}.json`);
}

describe('MetricsStore', () => {
  it('persists counters, gauges, and histograms when a snapshot path is configured', () => {
    const statePath = testStatePath('state');
    const store = new MetricsStore(statePath);

    store.incrementCounter('brevio_messages_total::channel=API', 2);
    store.setGauge('brevio_active_sessions::channel=API', 3);
    store.observeHistogram('brevio_message_latency_ms::channel=API', 120, [50, 100, 250]);

    const reloaded = new MetricsStore(statePath);
    assert.deepEqual(reloaded.counterEntries(), [['brevio_messages_total::channel=API', 2]]);
    assert.deepEqual(reloaded.gaugeEntries(), [['brevio_active_sessions::channel=API', 3]]);
    assert.equal(reloaded.histogramEntries().length, 1);
    assert.equal(reloaded.stats().histograms, 1);
  });

  it('fails fast when the persisted metrics snapshot is corrupt', () => {
    const statePath = testStatePath('corrupt');
    writeFileSync(statePath, JSON.stringify({ version: 1, counters: [['metric', 'bad']] }), 'utf8');

    assert.throws(() => new MetricsStore(statePath), /metrics snapshot is corrupt/);
  });
});
