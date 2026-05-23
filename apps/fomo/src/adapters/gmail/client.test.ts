import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  GmailApiError,
  GmailClient,
  GmailUnauthorizedError,
  GMAIL_READONLY_SCOPE,
  projectGmailMessage
} from './client.ts';

function mockFetch(
  handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown }>
): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString();
    const result = await handler(url, init ?? {});
    return new Response(JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

describe('GmailClient — read-only scope is hardcoded', () => {
  it('GMAIL_READONLY_SCOPE is the gmail.readonly URL', () => {
    assert.equal(GMAIL_READONLY_SCOPE, 'https://www.googleapis.com/auth/gmail.readonly');
  });
});

describe('GmailClient.getProfile', () => {
  it('returns parsed profile on 200', async () => {
    let receivedAuth = '';
    const fetchImpl = mockFetch(async (url, init) => {
      assert.match(url, /\/users\/me\/profile$/);
      receivedAuth = (init.headers as Record<string, string>).authorization;
      return {
        status: 200,
        body: {
          emailAddress: 'a@b.com',
          historyId: '12345',
          messagesTotal: 100,
          threadsTotal: 30
        }
      };
    });
    const client = new GmailClient({ fetchImpl });
    const profile = await client.getProfile('at_secret');
    assert.equal(profile.emailAddress, 'a@b.com');
    assert.equal(profile.historyId, '12345');
    assert.equal(profile.messagesTotal, 100);
    assert.equal(receivedAuth, 'Bearer at_secret');
  });

  it('throws GmailUnauthorizedError on 401', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 401, body: { error: { code: 401, message: 'invalid' } } }));
    const client = new GmailClient({ fetchImpl });
    await assert.rejects(client.getProfile('at_bad'), GmailUnauthorizedError);
  });

  it('throws GmailApiError on 500 (retryable=true)', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 500, body: { error: { code: 500, message: 'oops' } } }));
    const client = new GmailClient({ fetchImpl });
    await assert.rejects(
      client.getProfile('at'),
      (err: Error) => err instanceof GmailApiError && (err as GmailApiError).retryable === true
    );
  });

  it('throws GmailApiError on 400 (retryable=false)', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 400, body: { error: { code: 400, message: 'bad req' } } }));
    const client = new GmailClient({ fetchImpl });
    await assert.rejects(
      client.getProfile('at'),
      (err: Error) => err instanceof GmailApiError && (err as GmailApiError).retryable === false
    );
  });

  it('throws GmailApiError on 429 (retryable=true)', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 429, body: { error: { code: 429, message: 'rate' } } }));
    const client = new GmailClient({ fetchImpl });
    await assert.rejects(
      client.getProfile('at'),
      (err: Error) => err instanceof GmailApiError && err.retryable === true
    );
  });
});

describe('GmailClient.listHistorySince', () => {
  it('returns latest_history_id + added_message_ids', async () => {
    const fetchImpl = mockFetch(async (url) => {
      assert.match(url, /\/users\/me\/history\?/);
      assert.match(url, /startHistoryId=12345/);
      assert.match(url, /historyTypes=messageAdded/);
      return {
        status: 200,
        body: {
          historyId: '12999',
          history: [
            { id: '12500', messagesAdded: [{ message: { id: 'm-1', threadId: 't-1' } }] },
            { id: '12700', messagesAdded: [{ message: { id: 'm-2', threadId: 't-1' } }, { message: { id: 'm-3', threadId: 't-2' } }] }
          ]
        }
      };
    });
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '12345');
    assert.equal(result.latest_history_id, '12999');
    assert.deepEqual([...result.added_message_ids], ['m-1', 'm-2', 'm-3']);
  });

  it('returns empty added_message_ids when no history items', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: { historyId: '12345' } }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '12345');
    assert.equal(result.latest_history_id, '12345');
    assert.equal(result.added_message_ids.length, 0);
  });

  it('forwards 401 as GmailUnauthorizedError', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 401, body: { error: { code: 401, message: 'invalid' } } }));
    const client = new GmailClient({ fetchImpl });
    await assert.rejects(client.listHistorySince('at', '12345'), GmailUnauthorizedError);
  });
});

describe('GmailClient.getMessage + projectGmailMessage', () => {
  it('projects a typical multipart message into RawEmailContext', async () => {
    const fetchImpl = mockFetch(async (url) => {
      assert.match(url, /\/users\/me\/messages\/msg-1\?format=full$/);
      return {
        status: 200,
        body: {
          id: 'msg-1',
          threadId: 'thr-1',
          internalDate: '1716401400000',
          payload: {
            mimeType: 'multipart/alternative',
            headers: [
              { name: 'From', value: 'Sarah Johnson <sarah.j@school.edu>' },
              { name: 'Subject', value: 'Interview form due tonight' },
              { name: 'Authentication-Results', value: 'spf=pass' }
            ],
            parts: [
              {
                mimeType: 'text/plain',
                body: { size: 64, data: Buffer.from('Hi Albert, submit the form. — Sarah', 'utf8').toString('base64url') }
              },
              {
                mimeType: 'text/html',
                body: { size: 100, data: Buffer.from('<p>Hi Albert</p>', 'utf8').toString('base64url') }
              },
              {
                filename: 'form.pdf',
                mimeType: 'application/pdf',
                body: { size: 12345 }
              }
            ]
          }
        }
      };
    });
    const client = new GmailClient({ fetchImpl });
    const msg = await client.getMessage('at', 'msg-1');
    assert.equal(msg.message_id, 'msg-1');
    assert.equal(msg.thread_id, 'thr-1');
    assert.equal(msg.sender_email, 'sarah.j@school.edu');
    assert.equal(msg.sender_name, 'Sarah Johnson');
    assert.equal(msg.subject, 'Interview form due tonight');
    assert.match(msg.body_plain, /Hi Albert/);
    assert.match(msg.body_html ?? '', /<p>Hi Albert<\/p>/);
    assert.equal(msg.attachments?.length, 1);
    assert.equal(msg.attachments?.[0]?.filename, 'form.pdf');
    assert.equal(msg.attachments?.[0]?.size_bytes, 12345);
    assert.equal(msg.headers?.['Authentication-Results'], 'spf=pass');
  });

  it('handles a single-part text/plain message', () => {
    const projected = projectGmailMessage({
      id: 'msg-2',
      internalDate: '1716401400000',
      payload: {
        mimeType: 'text/plain',
        headers: [
          { name: 'From', value: 'noreply@x.com' },
          { name: 'Subject', value: 'simple' }
        ],
        body: { size: 8, data: Buffer.from('hi there', 'utf8').toString('base64url') }
      }
    });
    assert.equal(projected.sender_email, 'noreply@x.com');
    assert.equal(projected.sender_name, undefined);
    assert.equal(projected.body_plain, 'hi there');
    assert.equal(projected.body_html, undefined);
    assert.equal(projected.attachments?.length, 0);
  });

  it('returns frozen RawEmailContext (caller cannot mutate)', () => {
    const projected = projectGmailMessage({
      id: 'msg-3',
      internalDate: '1716401400000',
      payload: { headers: [{ name: 'From', value: 'x@y' }], mimeType: 'text/plain', body: { data: '' } }
    });
    assert.throws(() => {
      (projected as unknown as { subject: string }).subject = 'mutated';
    });
  });

  it('parses bare-email From header without display name', () => {
    const projected = projectGmailMessage({
      id: 'msg-4',
      internalDate: '0',
      payload: { headers: [{ name: 'From', value: 'plain@example.com' }] }
    });
    assert.equal(projected.sender_email, 'plain@example.com');
    assert.equal(projected.sender_name, undefined);
  });

  it('returns empty body_plain when payload missing', () => {
    const projected = projectGmailMessage({
      id: 'msg-5',
      internalDate: '0'
    });
    assert.equal(projected.body_plain, '');
  });
});

describe('GmailClient — no network call in default test path (tripwire)', () => {
  it('global fetch is not invoked when an explicit fetchImpl is provided', async () => {
    const original = globalThis.fetch;
    let tripwireCalls = 0;
    globalThis.fetch = (async () => {
      tripwireCalls++;
      throw new Error('GmailClient leaked through to global fetch');
    }) as typeof fetch;
    try {
      const fetchImpl = mockFetch(async () => ({
        status: 200,
        body: { emailAddress: 'a@b', historyId: '1', messagesTotal: 0, threadsTotal: 0 }
      }));
      const client = new GmailClient({ fetchImpl });
      await client.getProfile('at');
      assert.equal(tripwireCalls, 0);
    } finally {
      globalThis.fetch = original;
    }
  });
});
