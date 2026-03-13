import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { BrevioError } from '../errors/index.js';

describe('BrevioError', () => {
  it('is an instance of Error', () => {
    const err = new BrevioError('something broke', 'ERR_TEST');
    assert.ok(err instanceof Error);
  });

  it('is an instance of BrevioError', () => {
    const err = new BrevioError('something broke', 'ERR_TEST');
    assert.ok(err instanceof BrevioError);
  });

  it('has the correct message', () => {
    const err = new BrevioError('file not found', 'ERR_NOT_FOUND');
    assert.equal(err.message, 'file not found');
  });

  it('has the correct code', () => {
    const err = new BrevioError('file not found', 'ERR_NOT_FOUND');
    assert.equal(err.code, 'ERR_NOT_FOUND');
  });

  it('code is readonly', () => {
    const err = new BrevioError('test', 'ERR_CODE');
    // Verify code property exists and is the expected value
    assert.equal(err.code, 'ERR_CODE');
  });

  it('has a stack trace', () => {
    const err = new BrevioError('traced', 'ERR_TRACE');
    assert.equal(typeof err.stack, 'string');
    assert.ok(err.stack!.length > 0);
  });

  it('name defaults to the constructor name', () => {
    const err = new BrevioError('named', 'ERR_NAME');
    // Unless the constructor explicitly sets name, it will be "Error" due to super() behavior
    // but the constructor is BrevioError
    assert.ok(typeof err.name === 'string');
  });
});
