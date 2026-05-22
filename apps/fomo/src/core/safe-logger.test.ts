import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { containsTokenShape, redact, safeLog } from './safe-logger.ts';

describe('safe-logger.redact', () => {
  it('redacts sensitive keys at any depth', () => {
    const input = {
      access_token: 'plaintext',
      nested: { refresh_token: 'plaintext', other: 'fine' },
      list: [{ code: 'X' }, { ok: 1 }]
    };
    const out = redact(input) as Record<string, unknown>;
    assert.equal(out.access_token, '<redacted>');
    assert.equal((out.nested as Record<string, unknown>).refresh_token, '<redacted>');
    assert.equal((out.nested as Record<string, unknown>).other, 'fine');
    const list = out.list as Array<Record<string, unknown>>;
    assert.equal(list[0].code, '<redacted>');
    assert.equal(list[1].ok, 1);
  });

  it('redacts token-shaped strings within plain string values', () => {
    const input = { description: 'token=ya29.abcdefghijklmnopqrstuvwxyz1234567890ABCD' };
    const out = redact(input) as Record<string, unknown>;
    assert.match(out.description as string, /<redacted-token>/);
  });

  it('does not blow up on null and undefined', () => {
    assert.equal(redact(null), null);
    assert.equal(redact(undefined), undefined);
  });

  it('truncates excessive depth', () => {
    let nested: Record<string, unknown> = { v: 'leaf' };
    for (let i = 0; i < 12; i++) {
      nested = { inner: nested };
    }
    const out = JSON.stringify(redact(nested));
    assert.ok(out.includes('<truncated>'));
  });
});

describe('containsTokenShape', () => {
  it('detects JWT-shaped strings', () => {
    assert.equal(containsTokenShape('hello eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 world'), true);
  });
  it('detects google access token shape', () => {
    assert.equal(containsTokenShape('ya29.AbcDef0123456789ghijklmnopqr'), true);
  });
  it('returns false for normal text', () => {
    assert.equal(containsTokenShape('hello world'), false);
  });
});

describe('safeLog', () => {
  it('writes a JSON line and redacts sensitive keys', () => {
    let captured = '';
    safeLog(
      {
        service: 'fomo',
        environment: 'test',
        event: 'consent.write',
        severity: 'INFO',
        attrs: { access_token: 'X', user_id: 'u1' }
      },
      (line) => {
        captured = line;
      }
    );
    assert.match(captured, /"service":"fomo"/);
    assert.match(captured, /"event":"consent\.write"/);
    assert.match(captured, /"access_token":"<redacted>"/);
    assert.match(captured, /"user_id":"u1"/);
  });
});
