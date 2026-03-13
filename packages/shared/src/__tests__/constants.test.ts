import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { SERVICE_NAMES } from '../constants/index.js';

describe('SERVICE_NAMES', () => {
  it('is a non-null object', () => {
    assert.equal(typeof SERVICE_NAMES, 'object');
    assert.ok(SERVICE_NAMES !== null);
  });

  it('has at least one entry', () => {
    assert.ok(Object.keys(SERVICE_NAMES).length > 0);
  });

  it('contains the gateway service', () => {
    assert.equal(SERVICE_NAMES.gateway, 'brevio-gateway');
  });

  it('contains the brain service', () => {
    assert.equal(SERVICE_NAMES.brain, 'brevio-brain');
  });

  it('contains the hands service', () => {
    assert.equal(SERVICE_NAMES.hands, 'brevio-hands');
  });

  it('all values are non-empty strings', () => {
    for (const [key, value] of Object.entries(SERVICE_NAMES)) {
      assert.equal(typeof value, 'string', `SERVICE_NAMES.${key} should be a string`);
      assert.ok((value as string).length > 0, `SERVICE_NAMES.${key} should be non-empty`);
    }
  });
});
