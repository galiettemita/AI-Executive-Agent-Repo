import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { type SlackEgressView } from '../../core/egress-policy.ts';

import {
  SlackApiError,
  SlackAuthError,
  SlackClient,
  buildFounderReviewBlocks,
  type SlackPostInput
} from './client.ts';

const VIEW: SlackEgressView = Object.freeze({
  purpose: 'slack_founder_card',
  sender_email_masked: 'co****@school.edu',
  sender_name: 'Counselor',
  subject: 'Reminder: form due tonight',
  body_snippet: 'Please confirm the form by midnight tonight.',
  received_at: '2026-05-25T03:45:00.000Z',
  message_id: 'msg-abc'
});

const INPUT: SlackPostInput = Object.freeze({
  alert_id: 'alert-test-1',
  user_id: 'founder',
  view: VIEW,
  rank: Object.freeze({
    label: 'important',
    score: 0.85,
    reason: 'Time-sensitive deadline today.',
    model_name: 'gpt-5-mini',
    prompt_version: 'ranker-v0.1.0'
  })
});

function mockFetch(impl: (input: RequestInfo | URL, init?: RequestInit) => Response | Promise<Response>): typeof fetch {
  return (async (input, init) => impl(input as RequestInfo | URL, init)) as typeof fetch;
}

function okResponse(body: object, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { 'content-type': 'application/json' }
  });
}

/* ====================================================================== */
/* Construction guards                                                    */
/* ====================================================================== */

describe('SlackClient — construction', () => {
  it('throws when botToken is missing', () => {
    assert.throws(
      () => new SlackClient({ botToken: '', channelId: 'C123' }),
      /botToken is required/
    );
  });

  it('throws when channelId is missing', () => {
    assert.throws(
      () => new SlackClient({ botToken: 'xoxb-test', channelId: '' }),
      /channelId is required/
    );
  });

  it('throws when botToken does not start with xoxb-', () => {
    assert.throws(
      () => new SlackClient({ botToken: 'xoxa-not-a-bot', channelId: 'C123' }),
      /must start with "xoxb-"/
    );
  });

  it('accepts valid token + channel', () => {
    const c = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123' });
    assert.equal(c.channel(), 'C123');
  });
});

/* ====================================================================== */
/* postFounderReviewCard — happy path                                     */
/* ====================================================================== */

describe('SlackClient.postFounderReviewCard — happy path', () => {
  it('POSTs to chat.postMessage with bearer auth + JSON body, returns ts', async () => {
    let captured: { url: string; init?: RequestInit } | null = null;
    const fetchImpl = mockFetch((input, init) => {
      captured = { url: input.toString(), init };
      return okResponse({ ok: true, ts: '1748054400.000100', channel: 'C123' });
    });
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    const result = await client.postFounderReviewCard(INPUT);

    assert.equal(result.ts, '1748054400.000100');
    assert.equal(result.channel, 'C123');
    assert.ok(captured);
    const cap = captured as unknown as { url: string; init: RequestInit };
    assert.equal(cap.url, 'https://slack.com/api/chat.postMessage');
    assert.equal(cap.init.method, 'POST');
    const headers = cap.init.headers as Record<string, string>;
    assert.equal(headers.authorization, 'Bearer xoxb-test');
    assert.match(headers['content-type'] ?? '', /application\/json/);
    const sent = JSON.parse(cap.init.body as string);
    assert.equal(sent.channel, 'C123');
    assert.match(sent.text, /alert-test-1/);
    assert.ok(Array.isArray(sent.blocks));
    assert.ok(sent.blocks.length > 0);
  });

  it('falls back to channelId in result.channel when Slack omits channel field', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: true, ts: '1748054400.000200' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C999', fetchImpl });
    const result = await client.postFounderReviewCard(INPUT);
    assert.equal(result.channel, 'C999');
  });
});

/* ====================================================================== */
/* Auth failures                                                          */
/* ====================================================================== */

describe('SlackClient.postFounderReviewCard — auth failures', () => {
  it('throws SlackAuthError on HTTP 401', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'invalid_auth' }, { status: 401 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('throws SlackAuthError on HTTP 403', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'access_denied' }, { status: 403 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('promotes app-layer invalid_auth (HTTP 200, ok=false) to SlackAuthError', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'invalid_auth' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('promotes app-layer token_revoked (HTTP 200, ok=false) to SlackAuthError', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'token_revoked' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });
});

/* ====================================================================== */
/* API failures                                                           */
/* ====================================================================== */

describe('SlackClient.postFounderReviewCard — API failures', () => {
  it('throws SlackApiError on channel_not_found (HTTP 200, ok=false), non-retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'channel_not_found' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) =>
        err instanceof SlackApiError &&
        err.providerCode === 'channel_not_found' &&
        err.retryable === false
    );
  });

  it('throws SlackApiError on not_in_channel (HTTP 200, ok=false), non-retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'not_in_channel' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) =>
        err instanceof SlackApiError &&
        err.providerCode === 'not_in_channel' &&
        err.retryable === false
    );
  });

  it('throws SlackApiError on HTTP 429 rate_limited, retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'ratelimited' }, { status: 429 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.httpStatus === 429 && err.retryable === true
    );
  });

  it('throws SlackApiError on HTTP 500, retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({}, { status: 500 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.retryable === true
    );
  });

  it('throws SlackApiError on network failure (fetch throws), retryable', async () => {
    const fetchImpl = (async () => {
      throw new Error('ECONNREFUSED');
    }) as typeof fetch;
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.httpStatus === 0
    );
  });

  it('throws SlackApiError on ok=true but missing ts', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: true, channel: 'C123' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      /response\.ts was missing/
    );
  });
});

/* ====================================================================== */
/* Block builder — privacy + structure                                    */
/* ====================================================================== */

describe('buildFounderReviewBlocks — privacy + structure', () => {
  it('includes alert_id, model, label, score in blocks', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123');
    const serialized = JSON.stringify(body);
    assert.match(serialized, /alert-test-1/);
    assert.match(serialized, /important/);
    assert.match(serialized, /gpt-5-mini/);
    assert.match(serialized, /0\.85/);
  });

  it('uses the egress-redacted view: sender_email_masked, no raw email', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123');
    const serialized = JSON.stringify(body);
    // Masked sender appears verbatim (the redaction the egress layer produced)
    assert.match(serialized, /co\*\*\*\*@school\.edu/);
    // The Slack view does NOT carry body_plain / body_html / headers / attachments
    for (const forbidden of ['body_plain', 'body_html', 'headers', 'attachments', 'attachment_count']) {
      assert.ok(
        !serialized.includes(forbidden),
        `block payload must not contain key "${forbidden}"`
      );
    }
  });

  it('fallback text contains alert_id + label + score but no body content', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123');
    assert.match(body.text, /alert-test-1/);
    assert.match(body.text, /important/);
    assert.match(body.text, /0\.85/);
    assert.ok(!body.text.includes('Please confirm'));
  });

  it('channel field matches the channelId argument', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C-SPECIFIC');
    assert.equal(body.channel, 'C-SPECIFIC');
  });

  it('includes the 3D.1 status disclaimer (queued_for_review until 3D.2)', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123');
    const serialized = JSON.stringify(body);
    assert.match(serialized, /Phase 3D\.1/);
    assert.match(serialized, /queued_for_review/);
  });
});
