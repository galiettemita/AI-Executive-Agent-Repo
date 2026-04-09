import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { isHandsExecutableAdapter } from './plane-policy.js';

describe('hands plane policy', () => {
  it('allows hands-plane adapters', () => {
    assert.equal(isHandsExecutableAdapter({ plane: 'hands' }), true);
  });

  it('rejects gateway and brain adapters for hands execution', () => {
    assert.equal(isHandsExecutableAdapter({ plane: 'gateway' }), false);
    assert.equal(isHandsExecutableAdapter({ plane: 'brain' }), false);
    assert.equal(isHandsExecutableAdapter(null), false);
  });
});
