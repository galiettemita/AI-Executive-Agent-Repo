import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { sha256Hex } from '../utils/index.js';

describe('sha256Hex', () => {
  it('returns a 64-character hex string', () => {
    const hash = sha256Hex('hello');
    assert.equal(typeof hash, 'string');
    assert.equal(hash.length, 64);
    assert.match(hash, /^[0-9a-f]{64}$/);
  });

  it('produces consistent output for the same input', () => {
    const a = sha256Hex('deterministic');
    const b = sha256Hex('deterministic');
    assert.equal(a, b);
  });

  it('produces different output for different inputs', () => {
    const a = sha256Hex('input-a');
    const b = sha256Hex('input-b');
    assert.notEqual(a, b);
  });

  it('matches known SHA-256 for empty string', () => {
    const hash = sha256Hex('');
    assert.equal(hash, 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855');
  });

  it('matches known SHA-256 for "hello"', () => {
    const hash = sha256Hex('hello');
    assert.equal(hash, '2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824');
  });
});
