import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('home-assistant unit', () => {
  it('enforces 2fa for restricted actions', async () => {
    await assert.rejects(
      runClient({
        entity_id: 'lock.front_door',
        action: 'unlock'
      }),
      /SAFETY_2FA_REQUIRED/
    );

    const output = await runClient({
      entity_id: 'lock.front_door',
      action: 'unlock',
      two_factor_code: '123456'
    });

    assert.equal(output.state, 'on');
  });
});
