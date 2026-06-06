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

  /* ------------------------------------------------------------------ */
  /* Phase v0.5.8 — Gmail INBOX Event Reliability Hardening              */
  /* ------------------------------------------------------------------ */

  // C4 — external messageAdded path produces dispatch (no regression
  // from v0.5.7). The Q1.A filter swap MUST still include the legacy
  // messageAdded path; the only widening is adding labelAdded.
  it('v0.5.8 C4: external messageAdded path still produces dispatch (no regression)', async () => {
    let requestedHistoryTypes: string[] = [];
    let rawUrl = '';
    const fetchImpl = mockFetch(async (url) => {
      rawUrl = url;
      const parsed = new URL(url);
      requestedHistoryTypes = parsed.searchParams.getAll('historyTypes');
      return {
        status: 200,
        body: {
          historyId: '12999',
          history: [
            { id: '12500', messagesAdded: [{ message: { id: 'm-ext-1', threadId: 't-ext-1' } }] }
          ]
        }
      };
    });
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '12345');

    // Q1.A — Gmail's contract is REPEATED historyTypes params, not a
    // comma-joined single value. The query string must look like
    // `?historyTypes=messageAdded&historyTypes=labelAdded`. Asserting on
    // .getAll() catches the comma-joined regression that v0.5.8 smoke
    // 2026-06-06 surfaced against real Gmail.
    assert.deepEqual(
      requestedHistoryTypes,
      ['messageAdded', 'labelAdded'],
      `expected exactly ['messageAdded','labelAdded'] (got ${JSON.stringify(requestedHistoryTypes)}; raw=${rawUrl})`
    );

    assert.deepEqual([...result.added_message_ids], ['m-ext-1']);
    const prov = result.event_provenance.get('m-ext-1');
    assert.ok(prov, 'provenance entry must exist for added id');
    assert.equal(prov.via_messageAdded, true);
    assert.equal(prov.via_labelAdded_inbox, false);
    assert.equal(result.malformed_labelAdded_skipped, 0);
  });

  // C5 — Gmail-to-self labelAdded:INBOX-only path produces dispatch.
  // The v0.5.7 baseline NEVER surfaced this message. v0.5.8 surfaces it
  // via the labelAdded path with INBOX literal post-filter.
  it('v0.5.8 C5: Gmail-to-self labelAdded:INBOX-only path produces dispatch', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        historyId: '13100',
        history: [
          {
            id: '13050',
            // NOTE: NO messagesAdded — only labelsAdded. This is the exact
            // Gmail-to-self self-send shape that v0.5.7 missed.
            labelsAdded: [
              { message: { id: 'm-self-1', threadId: 't-self-1' }, labelIds: ['INBOX', 'UNREAD'] }
            ]
          }
        ]
      }
    }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '13000');

    assert.deepEqual([...result.added_message_ids], ['m-self-1']);
    const prov = result.event_provenance.get('m-self-1');
    assert.ok(prov);
    assert.equal(prov.via_messageAdded, false);
    assert.equal(prov.via_labelAdded_inbox, true);
    assert.equal(result.malformed_labelAdded_skipped, 0);
  });

  // C6 — routed/forwarded labelAdded:INBOX path produces dispatch. From
  // the parser's view, this is shape-identical to C5 — a labelAdded
  // event with INBOX literal — regardless of whether the message was
  // freshly delivered, forwarded, or filter-routed. The test asserts
  // a *second*, distinct fixture (different ids/timestamps) to make
  // the regression specifically catch a "first-fixture-only" parser bug.
  it('v0.5.8 C6: routed / forwarded labelAdded:INBOX path produces dispatch', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        historyId: '14500',
        history: [
          {
            id: '14400',
            labelsAdded: [
              { message: { id: 'm-routed-1', threadId: 't-routed-1' }, labelIds: ['INBOX'] }
            ]
          },
          {
            id: '14450',
            labelsAdded: [
              {
                message: { id: 'm-routed-2', threadId: 't-routed-2' },
                labelIds: ['INBOX', 'Label_routed_by_filter_xyz']
              }
            ]
          }
        ]
      }
    }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '14000');

    assert.deepEqual([...result.added_message_ids], ['m-routed-1', 'm-routed-2']);
    for (const id of ['m-routed-1', 'm-routed-2']) {
      const prov = result.event_provenance.get(id);
      assert.ok(prov, `provenance for ${id} must exist`);
      assert.equal(prov.via_labelAdded_inbox, true);
      assert.equal(prov.via_messageAdded, false);
    }
  });

  // C8 — labelAdded with NON-INBOX label is ignored (no dispatch).
  // Q2.A — accept ONLY where addedLabels includes the literal 'INBOX'.
  // STARRED / IMPORTANT / custom labels are silently filtered (no audit
  // — would be noise per Q5 lock).
  it('v0.5.8 C8: labelAdded with NON-INBOX label is ignored (no dispatch)', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        historyId: '15500',
        history: [
          {
            id: '15400',
            labelsAdded: [
              // STARRED-only — must be skipped.
              { message: { id: 'm-starred-1', threadId: 't-1' }, labelIds: ['STARRED'] },
              // IMPORTANT-only — must be skipped.
              { message: { id: 'm-important-1', threadId: 't-2' }, labelIds: ['IMPORTANT'] },
              // Custom user label — must be skipped.
              { message: { id: 'm-custom-1', threadId: 't-3' }, labelIds: ['Label_my_project'] },
              // INBOX present alongside other labels — must be ACCEPTED.
              { message: { id: 'm-inbox-1', threadId: 't-4' }, labelIds: ['STARRED', 'INBOX'] }
            ]
          }
        ]
      }
    }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '15000');

    // Only the INBOX-bearing event surfaces; the others are silently
    // ignored. Q5 says no audit for non-INBOX labels (avoid noise) —
    // verified at the worker layer; the client just returns the post-
    // filter ids.
    assert.deepEqual([...result.added_message_ids], ['m-inbox-1']);
    assert.equal(result.event_provenance.has('m-starred-1'), false);
    assert.equal(result.event_provenance.has('m-important-1'), false);
    assert.equal(result.event_provenance.has('m-custom-1'), false);
    assert.equal(result.malformed_labelAdded_skipped, 0);
  });

  // C9 — malformed labelAdded (missing or non-array labelIds) → skipped
  // silently; malformed_labelAdded_skipped count surfaces so the worker
  // can emit one fomo.gmail.poll.event_skipped audit per skipped event.
  // (The audit emission itself is verified by gmail-poll.test.ts.)
  it('v0.5.8 C9: malformed labelAdded (missing addedLabels) is skipped + counted', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        historyId: '16500',
        history: [
          {
            id: '16400',
            labelsAdded: [
              // Malformed — labelIds missing entirely.
              { message: { id: 'm-malformed-1', threadId: 't-1' } },
              // Malformed — labelIds is null.
              { message: { id: 'm-malformed-2', threadId: 't-2' }, labelIds: null },
              // Malformed — labelIds is a string (not an array).
              {
                message: { id: 'm-malformed-3', threadId: 't-3' },
                labelIds: 'INBOX' as unknown as readonly string[]
              },
              // Well-formed control — should still surface.
              { message: { id: 'm-good-1', threadId: 't-4' }, labelIds: ['INBOX'] }
            ]
          }
        ]
      }
    }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '16000');

    assert.deepEqual([...result.added_message_ids], ['m-good-1']);
    assert.equal(
      result.malformed_labelAdded_skipped,
      3,
      'three malformed labelAdded events expected to be counted'
    );
    assert.equal(result.event_provenance.has('m-malformed-1'), false);
    assert.equal(result.event_provenance.has('m-malformed-2'), false);
    assert.equal(result.event_provenance.has('m-malformed-3'), false);
  });

  // Cross-event-type dedupe at the client layer (Q3.A first-seen wins).
  // Companion to C7 (which is in gmail-poll.test.ts — proves the worker
  // dispatches exactly once). This client-level test proves the parser
  // dedupes too: same id in BOTH event types yields ONE added id with
  // BOTH provenance flags set.
  it('v0.5.8: same message_id in BOTH messageAdded AND labelAdded:INBOX yields ONE added id with both provenance flags', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        historyId: '17500',
        history: [
          {
            id: '17400',
            messagesAdded: [{ message: { id: 'm-both-1', threadId: 't-1' } }],
            labelsAdded: [{ message: { id: 'm-both-1', threadId: 't-1' }, labelIds: ['INBOX'] }]
          }
        ]
      }
    }));
    const client = new GmailClient({ fetchImpl });
    const result = await client.listHistorySince('at', '17000');

    // Q3.A first-seen wins: m-both-1 appears EXACTLY once.
    assert.deepEqual([...result.added_message_ids], ['m-both-1']);
    const prov = result.event_provenance.get('m-both-1');
    assert.ok(prov);
    assert.equal(prov.via_messageAdded, true);
    assert.equal(prov.via_labelAdded_inbox, true);
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
