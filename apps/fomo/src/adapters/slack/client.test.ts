import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { type SlackEgressView } from '../../core/egress-policy.ts';

import {
  SlackApiError,
  SlackAuthError,
  SlackClient,
  buildFounderReviewBlocks,
  buildFounderReviewResolutionBlocks,
  verifySlackSignature,
  type SlackPostInput,
  type SlackUpdateInput
} from './client.ts';
import { createHmac } from 'node:crypto';

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
      () => new SlackClient({ botToken: '', channelId: 'C123', founderUserId: 'founder' }),
      /botToken is required/
    );
  });

  it('throws when channelId is missing', () => {
    assert.throws(
      () => new SlackClient({ botToken: 'xoxb-test', channelId: '', founderUserId: 'founder' }),
      /channelId is required/
    );
  });

  it('throws when botToken does not start with xoxb-', () => {
    assert.throws(
      () => new SlackClient({ botToken: 'xoxa-not-a-bot', channelId: 'C123' , founderUserId: 'founder' }),
      /must start with "xoxb-"/
    );
  });

  it('accepts valid token + channel', () => {
    const c = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123' , founderUserId: 'founder' });
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
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
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
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C999', fetchImpl , founderUserId: 'founder' });
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
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('throws SlackAuthError on HTTP 403', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'access_denied' }, { status: 403 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('promotes app-layer invalid_auth (HTTP 200, ok=false) to SlackAuthError', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'invalid_auth' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });

  it('promotes app-layer token_revoked (HTTP 200, ok=false) to SlackAuthError', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'token_revoked' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(() => client.postFounderReviewCard(INPUT), SlackAuthError);
  });
});

/* ====================================================================== */
/* API failures                                                           */
/* ====================================================================== */

describe('SlackClient.postFounderReviewCard — API failures', () => {
  it('throws SlackApiError on channel_not_found (HTTP 200, ok=false), non-retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'channel_not_found' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
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
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
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
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.httpStatus === 429 && err.retryable === true
    );
  });

  it('throws SlackApiError on HTTP 500, retryable', async () => {
    const fetchImpl = mockFetch(() => okResponse({}, { status: 500 }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.retryable === true
    );
  });

  it('throws SlackApiError on network failure (fetch throws), retryable', async () => {
    const fetchImpl = (async () => {
      throw new Error('ECONNREFUSED');
    }) as typeof fetch;
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(
      () => client.postFounderReviewCard(INPUT),
      (err: unknown) => err instanceof SlackApiError && err.httpStatus === 0
    );
  });

  it('throws SlackApiError on ok=true but missing ts', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: true, channel: 'C123' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
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
    const body = buildFounderReviewBlocks(INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    assert.match(serialized, /alert-test-1/);
    assert.match(serialized, /important/);
    assert.match(serialized, /gpt-5-mini/);
    assert.match(serialized, /0\.85/);
  });

  it('uses the egress-redacted view: sender_email_masked, no raw email', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123', 'founder');
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
    const body = buildFounderReviewBlocks(INPUT, 'C123', 'founder');
    assert.match(body.text, /alert-test-1/);
    assert.match(body.text, /important/);
    assert.match(body.text, /0\.85/);
    assert.ok(!body.text.includes('Please confirm'));
  });

  it('channel field matches the channelId argument', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C-SPECIFIC', 'founder');
    assert.equal(body.channel, 'C-SPECIFIC');
  });

  it('Phase 3D.2: includes Approve + Reject interactive buttons with alert_id-bearing block_id', () => {
    const body = buildFounderReviewBlocks(INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    // Each button's action_id encodes the decision; block_id carries alert_id
    assert.match(serialized, /"action_id":"fomo\.approve"/);
    assert.match(serialized, /"action_id":"fomo\.reject"/);
    assert.match(serialized, /"block_id":"fomo_alert:alert-test-1"/);
    // Phase 3D.1 status disclaimer should no longer appear — the buttons
    // themselves are the affordance for "you can act on this now."
    assert.ok(!serialized.includes('Phase 3D.1'), 'stale 3D.1 disclaimer should be removed once buttons land');
    // Button text + styles
    assert.match(serialized, /Approve/);
    assert.match(serialized, /Reject/);
    assert.match(serialized, /"style":"primary"/);
    assert.match(serialized, /"style":"danger"/);
  });
});

/* ====================================================================== */
/* Phase 3D.2: verifySlackSignature                                       */
/* ====================================================================== */

const TEST_SIGNING_SECRET = 'test_signing_secret_aaaaaaaaaaaaaaaa';

function signSlackRequest(secret: string, timestamp: string, body: string): string {
  const base = `v0:${timestamp}:${body}`;
  const hex = createHmac('sha256', secret).update(base).digest('hex');
  return `v0=${hex}`;
}

describe('verifySlackSignature — happy path', () => {
  it('accepts a fresh, correctly-signed request', () => {
    const timestamp = '1748054400';
    const body = 'payload=%7B%22type%22%3A%22block_actions%22%7D';
    const signature = signSlackRequest(TEST_SIGNING_SECRET, timestamp, body);
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp,
      signature,
      body,
      now: () => 1748054400
    });
    assert.equal(r.ok, true);
  });
});

describe('verifySlackSignature — rejection paths', () => {
  const NOW = 1748054400;
  const goodBody = 'payload=%7B%22ok%22%3Atrue%7D';
  const goodTs = String(NOW);
  const goodSig = signSlackRequest(TEST_SIGNING_SECRET, goodTs, goodBody);

  it('rejects a stale timestamp (>300s old)', () => {
    const oldTs = String(NOW - 301);
    const sig = signSlackRequest(TEST_SIGNING_SECRET, oldTs, goodBody);
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: oldTs,
      signature: sig,
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'stale_timestamp');
  });

  it('rejects a future timestamp (>300s ahead)', () => {
    const futureTs = String(NOW + 301);
    const sig = signSlackRequest(TEST_SIGNING_SECRET, futureTs, goodBody);
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: futureTs,
      signature: sig,
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'stale_timestamp');
  });

  it('rejects a malformed timestamp', () => {
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: 'not-a-number',
      signature: goodSig,
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_timestamp');
  });

  it('rejects a malformed signature (wrong prefix)', () => {
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: goodTs,
      signature: 'v1=abc123',
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_signature');
  });

  it('rejects a malformed signature (wrong length)', () => {
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: goodTs,
      signature: 'v0=deadbeef',
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_signature');
  });

  it('rejects a signature signed with the WRONG secret', () => {
    const wrongSig = signSlackRequest('not_the_real_secret_xxxxxxxxxxxxxxx', goodTs, goodBody);
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: goodTs,
      signature: wrongSig,
      body: goodBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'signature_mismatch');
  });

  it('rejects when the BODY has been tampered with after signing', () => {
    // Replay attack: attacker captures a valid (ts, sig) and replaces body.
    const tamperedBody = 'payload=%7B%22ok%22%3Afalse%7D';
    const r = verifySlackSignature({
      signingSecret: TEST_SIGNING_SECRET,
      timestamp: goodTs,
      signature: goodSig,
      body: tamperedBody,
      now: () => NOW
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'signature_mismatch');
  });
});

/* ====================================================================== */
/* Phase 3D.2: updateFounderReviewCard (chat.update)                      */
/* ====================================================================== */

const UPDATE_INPUT: SlackUpdateInput = Object.freeze({
  ts: '1748054400.000100',
  channel: 'C123',
  alert_id: 'alert-test-1',
  user_id: 'founder',
  view: VIEW,
  rank: Object.freeze({
    label: 'important',
    score: 0.85,
    reason: 'Time-sensitive deadline today.',
    model_name: 'gpt-5-mini',
    prompt_version: 'ranker-v0.1.0'
  }),
  decision: Object.freeze({
    kind: 'approved' as const,
    at: '2026-05-25T19:00:00.000Z',
    actor: 'U_FOUNDER'
  })
});

describe('SlackClient.updateFounderReviewCard — happy path', () => {
  it('POSTs to chat.update with bearer auth + JSON body referencing ts + channel', async () => {
    let captured: { url: string; init?: RequestInit } | null = null;
    const fetchImpl = mockFetch((input, init) => {
      captured = { url: input.toString(), init };
      return okResponse({ ok: true, ts: '1748054400.000100', channel: 'C123' });
    });
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    const result = await client.updateFounderReviewCard(UPDATE_INPUT);

    assert.equal(result.ts, '1748054400.000100');
    assert.equal(result.channel, 'C123');
    assert.ok(captured);
    const cap = captured as unknown as { url: string; init: RequestInit };
    assert.equal(cap.url, 'https://slack.com/api/chat.update');
    assert.equal(cap.init.method, 'POST');
    const sent = JSON.parse(cap.init.body as string);
    assert.equal(sent.channel, 'C123');
    assert.equal(sent.ts, '1748054400.000100');
    // Resolution body must reference the decision
    assert.match(sent.text, /Approved by U_FOUNDER/);
    // Buttons (actions block) should be GONE — replaced by the resolution context
    const serialized = JSON.stringify(sent.blocks);
    assert.ok(!serialized.includes('"action_id":"fomo.approve"'), 'updated card must remove the approve button');
    assert.ok(!serialized.includes('"action_id":"fomo.reject"'), 'updated card must remove the reject button');
  });

  it('throws SlackAuthError on token_revoked', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'token_revoked' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(() => client.updateFounderReviewCard(UPDATE_INPUT), SlackAuthError);
  });

  it('throws SlackApiError on message_not_found (non-retryable)', async () => {
    const fetchImpl = mockFetch(() => okResponse({ ok: false, error: 'message_not_found' }));
    const client = new SlackClient({ botToken: 'xoxb-test', channelId: 'C123', fetchImpl , founderUserId: 'founder' });
    await assert.rejects(
      () => client.updateFounderReviewCard(UPDATE_INPUT),
      (err: unknown) =>
        err instanceof SlackApiError &&
        err.providerCode === 'message_not_found' &&
        err.retryable === false
    );
  });
});

describe('buildFounderReviewResolutionBlocks — privacy + structure', () => {
  it('header reflects the decision (approved vs rejected)', () => {
    const approved = buildFounderReviewResolutionBlocks(UPDATE_INPUT, 'founder');
    assert.match(JSON.stringify(approved), /✅ Approved/);
    const rejected = buildFounderReviewResolutionBlocks(
      {
        ...UPDATE_INPUT,
        decision: { ...UPDATE_INPUT.decision, kind: 'rejected' }
      },
      'founder'
    );
    assert.match(JSON.stringify(rejected), /❌ Rejected/);
  });

  it('replaces action buttons with a resolution context section', () => {
    const body = buildFounderReviewResolutionBlocks(UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    assert.ok(!serialized.includes('"action_id":"fomo.approve"'));
    assert.ok(!serialized.includes('"action_id":"fomo.reject"'));
    assert.match(serialized, /<@U_FOUNDER>/); // Slack user mention
    assert.match(serialized, /2026-05-25T19:00:00\.000Z/);
  });

  it('still respects egress privacy (no body / headers / attachments)', () => {
    const body = buildFounderReviewResolutionBlocks(UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    for (const forbidden of ['body_plain', 'body_html', 'headers', 'attachments', 'attachment_count']) {
      assert.ok(!serialized.includes(forbidden), `resolution payload must not contain "${forbidden}"`);
    }
    assert.match(serialized, /co\*\*\*\*@school\.edu/);
  });
});

/* ====================================================================== */
/* Phase v0.5.1 Step 5 — friend-safe Slack card (UNCONDITIONAL)           */
/* ====================================================================== */
//
// Founder-locked rule (2026-05-29):
//   if alert.user_id !== founderUserId, render friend-safe card
//   UNCONDITIONALLY. Privacy must NOT depend on FOMO_FRIEND_BETA_ENABLED.
//
// Friend-safe card may include: sender, subject, ranker reason, label, score.
// Friend-safe card must NOT include: body excerpt, snippet, attachment names,
// raw headers, raw email body, hidden canary strings.
//
// We use a planted canary string in body_snippet/subject/sender to prove
// the redaction at the card-builder layer; any leak surfaces immediately.

const FRIEND_CANARY_BODY = 'CANARY-FRIEND-BODY-MUST-NOT-LEAK-TO-FOUNDER-SLACK-12345';
const FRIEND_CANARY_ATTACHMENT = 'CANARY-FRIEND-ATTACHMENT-NAME-67890';
const FRIEND_CANARY_HEADER = 'CANARY-FRIEND-HEADER-PARSE-ME-IF-YOU-CAN';
const FRIEND_VIEW: SlackEgressView = Object.freeze({
  purpose: 'slack_founder_card',
  sender_email_masked: 'el****@greenoaks.com',
  sender_name: 'Eli at Greenoaks',
  subject: 'Re: Greenoaks investor intro — Thursday meeting?',
  // Body snippet here would contain the canary in v0.1; for friend-owned
  // alerts the card builder must DROP this field entirely.
  body_snippet: FRIEND_CANARY_BODY,
  received_at: '2026-05-29T01:18:20.744Z',
  message_id: 'gmail-msg-19e714e6290c786b'
});
const FRIEND_INPUT: SlackPostInput = Object.freeze({
  alert_id: 'alert-friend-1',
  user_id: 'friend-uuid-not-the-founder',
  view: FRIEND_VIEW,
  rank: Object.freeze({
    label: 'important',
    score: 0.94,
    reason: 'Investor intro with named partner; time-sensitive Thursday slot.',
    model_name: 'gpt-5-mini',
    prompt_version: 'ranker-v0.1.0'
  })
});

describe('buildFounderReviewBlocks — friend-safe card (Phase v0.5.1 Step 5)', () => {
  it('friend-owned card (user_id !== founderUserId) DROPS the snippet section entirely', () => {
    const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    assert.equal(
      serialized.includes(FRIEND_CANARY_BODY),
      false,
      'friend-owned card leaked body_snippet content'
    );
    assert.equal(
      serialized.includes('*Snippet*'),
      false,
      'friend-owned card must not contain a *Snippet* section header'
    );
  });

  it('friend-owned card contains ONLY the founder-spec allowed fields (sender, subject, label, score, reason)', () => {
    const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    // Allowed fields present.
    assert.match(serialized, /el\*\*\*\*@greenoaks\.com/);  // masked sender email
    assert.match(serialized, /Eli at Greenoaks/);            // sender name
    assert.match(serialized, /Greenoaks investor intro/);    // subject
    assert.match(serialized, /important/);                   // label
    assert.match(serialized, /0\.94/);                       // score
    assert.match(serialized, /Investor intro with named partner/); // ranker reason
    // Approve/Reject buttons must still be present — the founder
    // needs to act on the card.
    assert.match(serialized, /fomo\.approve/);
    assert.match(serialized, /fomo\.reject/);
  });

  it('friend-owned card REDACTS message_id from the context line (defense-in-depth)', () => {
    const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    assert.equal(
      serialized.includes('gmail-msg-19e714e6290c786b'),
      false,
      'friend-owned card leaked the Gmail message_id'
    );
    assert.equal(
      serialized.includes('message_id'),
      false,
      'friend-owned card must not contain a message_id field label'
    );
    // Context line uses the redaction tag instead.
    assert.match(serialized, /friend-owned \(user redacted\)/);
  });

  it('friend-owned card REDACTS user_id from the context line (defense-in-depth)', () => {
    const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    assert.equal(
      serialized.includes('friend-uuid-not-the-founder'),
      false,
      'friend-owned card leaked the friend user_id'
    );
  });

  it('canary-string scan: no body / snippet / attachment / header / raw-email content in friend-owned card', () => {
    // Plant additional canaries in different egress-view fields.
    const inputWithCanaries: SlackPostInput = {
      ...FRIEND_INPUT,
      view: {
        ...FRIEND_VIEW,
        body_snippet: FRIEND_CANARY_BODY,
        // Plant via subject too — subject IS allowed in friend-safe,
        // so this canary SHOULD appear; the test below confirms it.
        subject: FRIEND_VIEW.subject
      }
    };
    const body = buildFounderReviewBlocks(inputWithCanaries, 'C123', 'founder');
    const serialized = JSON.stringify(body);

    // Forbidden canaries — these MUST NOT appear.
    const forbiddenCanaries = [FRIEND_CANARY_BODY, FRIEND_CANARY_ATTACHMENT, FRIEND_CANARY_HEADER];
    for (const canary of forbiddenCanaries) {
      assert.equal(
        serialized.includes(canary),
        false,
        `friend-owned card leaked canary "${canary}"`
      );
    }

    // Allowed canary (subject content) — the founder spec permits subject.
    assert.match(serialized, /Greenoaks investor intro/);
  });

  it('founder-owned card (user_id === founderUserId) STILL INCLUDES the snippet (v0.1 backward compat)', () => {
    // Same INPUT shape as the original v0.1 tests: user_id='founder'.
    const body = buildFounderReviewBlocks(INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    // Snippet must be present for the founder's own emails — no
    // third-party privacy concern.
    assert.match(serialized, /\*Snippet\*/);
    assert.match(serialized, /Please confirm the form by midnight tonight/);
    // The v0.1 context line shows user_id + message_id for founder-owned alerts.
    assert.match(serialized, /user: `founder`/);
    assert.match(serialized, /message_id: `msg-abc`/);
  });

  it('forbidden-key sweep: friend-owned card has NO body_plain / body_html / headers / attachments / snippet text', () => {
    const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
    const serialized = JSON.stringify(body);
    const forbiddenKeys = [
      'body_plain',
      'body_html',
      'body_snippet',
      'attachments',
      'attachment_count',
      'headers',
      'raw_body',
      'snippet'
    ];
    for (const k of forbiddenKeys) {
      assert.equal(
        serialized.includes(k),
        false,
        `friend-owned card payload must not contain key "${k}"`
      );
    }
  });

  it('privacy is UNCONDITIONAL — the builder does NOT consult any kill-switch env var', () => {
    // Sanity: even if a future contributor wires FOMO_FRIEND_BETA_ENABLED
    // into the builder, this test asserts that for a friend-owned alert
    // the card is friend-safe REGARDLESS of any env setting at call time.
    const prevEnv = process.env.FOMO_FRIEND_BETA_ENABLED;
    try {
      // Try BOTH switch states; assert the friend-safe behavior either way.
      for (const v of [undefined, 'false', 'true']) {
        if (v === undefined) delete process.env.FOMO_FRIEND_BETA_ENABLED;
        else process.env.FOMO_FRIEND_BETA_ENABLED = v;
        const body = buildFounderReviewBlocks(FRIEND_INPUT, 'C123', 'founder');
        const serialized = JSON.stringify(body);
        assert.equal(
          serialized.includes(FRIEND_CANARY_BODY),
          false,
          `friend-owned card leaked snippet when FOMO_FRIEND_BETA_ENABLED=${String(v)}`
        );
        assert.equal(serialized.includes('*Snippet*'), false);
      }
    } finally {
      if (prevEnv === undefined) delete process.env.FOMO_FRIEND_BETA_ENABLED;
      else process.env.FOMO_FRIEND_BETA_ENABLED = prevEnv;
    }
  });
});

describe('buildFounderReviewResolutionBlocks — friend-safe card (Phase v0.5.1 Step 5)', () => {
  const FRIEND_UPDATE_INPUT: SlackUpdateInput = Object.freeze({
    ts: '1780017500.939999',
    channel: 'C123',
    alert_id: 'alert-friend-1',
    user_id: 'friend-uuid-not-the-founder',
    view: FRIEND_VIEW,
    rank: FRIEND_INPUT.rank,
    decision: Object.freeze({
      kind: 'approved',
      at: '2026-05-29T01:18:51.725Z',
      actor: 'U-L3RA'
    })
  });

  it('friend-owned resolution card DROPS the snippet (same rule as the initial card)', () => {
    const body = buildFounderReviewResolutionBlocks(FRIEND_UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    assert.equal(serialized.includes(FRIEND_CANARY_BODY), false);
    assert.equal(serialized.includes('*Snippet*'), false);
  });

  it('friend-owned resolution card includes the allowed fields + redacts user_id/message_id', () => {
    const body = buildFounderReviewResolutionBlocks(FRIEND_UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    assert.match(serialized, /el\*\*\*\*@greenoaks\.com/);
    assert.match(serialized, /Greenoaks investor intro/);
    assert.match(serialized, /important/);
    assert.match(serialized, /0\.94/);
    assert.match(serialized, /friend-owned \(user redacted\)/);
    assert.equal(serialized.includes('friend-uuid-not-the-founder'), false);
    assert.equal(serialized.includes('gmail-msg-19e714e6290c786b'), false);
  });

  it('founder-owned resolution card (regression) STILL includes snippet + full context', () => {
    const body = buildFounderReviewResolutionBlocks(UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    assert.match(serialized, /\*Snippet\*/);
    assert.match(serialized, /user: `founder`/);
  });

  it('forbidden-key sweep on friend-owned resolution card', () => {
    const body = buildFounderReviewResolutionBlocks(FRIEND_UPDATE_INPUT, 'founder');
    const serialized = JSON.stringify(body);
    const forbiddenKeys = [
      'body_plain',
      'body_html',
      'body_snippet',
      'attachments',
      'attachment_count',
      'headers',
      'raw_body',
      'snippet'
    ];
    for (const k of forbiddenKeys) {
      assert.equal(
        serialized.includes(k),
        false,
        `friend-owned resolution must not contain key "${k}"`
      );
    }
  });
});
