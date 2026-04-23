import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { isHandsExecutableAdapter } from './plane-policy.js';

describe('hands plane policy', () => {
  it('allows hands-plane adapters', () => {
    assert.equal(isHandsExecutableAdapter({ plane: 'hands' }), true);
  });

  it('allows gateway perception adapters and rejects brain-only adapters', () => {
    assert.equal(isHandsExecutableAdapter({ plane: 'gateway' }), true);
    assert.equal(isHandsExecutableAdapter({ plane: 'brain' }), false);
    assert.equal(isHandsExecutableAdapter(null), false);
  });
});
