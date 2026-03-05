import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('icloud-findmy unit', () => {
  it('returns devices', async () => {
    const output = await runClient({});
    assert.equal(output.provider, 'icloud-findmy');
    assert.ok(output.devices.length > 0);
  });

  it('filters by device name', async () => {
    const output = await runClient({ device_name: 'airpods' });
    assert.ok(output.devices.every((device) => device.name.toLowerCase().includes('airpods')));
  });
});
