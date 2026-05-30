// Slack HTTP client — outbound-only chat.postMessage for the Phase 3D.1
// founder candidate-review channel.
//
// 3D.1 scope: posts a single text-blocks message per candidate alert.
// NO interactive buttons (those require an inbound HTTP endpoint that
// 3D.2 will add). NO message editing or threading. NO inbound webhook
// signature verification (3D.2 territory). NO reactions.
//
// Design choices:
//   * Direct fetch, no @slack/web-api SDK. Mirrors the GmailClient
//     pattern (apps/fomo/src/adapters/gmail/client.ts) — keeps the
//     lockfile small and the call surface auditable.
//   * Injectable FetchLike — tests inject a mock; CI never hits real
//     Slack.
//   * Caller-supplied bot token + channel id at construction time.
//   * Fail-closed at construction: missing/empty token or channel
//     throws synchronously so a misconfigured boot crashes early.
//   * Error classes mirror Gmail: 401/403 → SlackAuthError,
//     ok=false → SlackApiError with providerCode, 429/5xx retryable.
//   * Slack's HTTP layer returns 200 for application-level errors;
//     the `ok: false` body is the real signal. We surface that as
//     SlackApiError with the providerCode (channel_not_found,
//     not_in_channel, rate_limited, etc.) so callers can decide
//     whether to retry.

import { type SlackEgressView } from '../../core/egress-policy.js';
import { type RankLabel } from '../../memory/rank-results.js';

export type FetchLike = typeof fetch;

const SLACK_POSTMESSAGE_URL = 'https://slack.com/api/chat.postMessage';
const SLACK_UPDATE_URL = 'https://slack.com/api/chat.update';

export class SlackAuthError extends Error {
  readonly httpStatus: 401 | 403;
  readonly providerCode: string | undefined;
  constructor(httpStatus: 401 | 403, providerCode: string | undefined, reason: string) {
    super(`Slack auth error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'SlackAuthError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
  }
}

export class SlackApiError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  readonly retryable: boolean;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(`Slack API error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'SlackApiError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
    // 429 (rate_limited) and 5xx are retryable. Slack-side application
    // errors like channel_not_found / invalid_arguments are NOT.
    this.retryable =
      httpStatus >= 500 ||
      httpStatus === 429 ||
      providerCode === 'rate_limited' ||
      providerCode === 'service_unavailable';
  }
}

export interface SlackPostInput {
  readonly alert_id: string;
  readonly user_id: string;
  readonly view: SlackEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
    readonly reason: string;
    readonly model_name: string;
    readonly prompt_version: string;
  };
}

export interface SlackPostResult {
  // Slack message timestamp (e.g. "1748054400.000100"). Stable handle
  // the 3D.2 approval flow can use to thread/edit later.
  readonly ts: string;
  readonly channel: string;
}

export interface SlackClientConfig {
  // Bot token, must start with "xoxb-".
  readonly botToken: string;
  // Channel id (e.g. "C0123456789"). NOT a channel name.
  readonly channelId: string;
  // Phase v0.5.1 Step 5 — the founder's user_id. Used by the card
  // builder to branch between the founder-owned card (full snippet)
  // and the friend-safe card (no body content). UNCONDITIONAL —
  // does NOT depend on FOMO_FRIEND_BETA_ENABLED. Privacy is an
  // invariant of the data shape, not a behavior of the kill switch.
  readonly founderUserId: string;
  readonly fetchImpl?: FetchLike;
}

export class SlackClient {
  private readonly botToken: string;
  private readonly channelId: string;
  private readonly founderUserId: string;
  private readonly fetchImpl: FetchLike;

  constructor(config: SlackClientConfig) {
    if (!config.botToken || config.botToken.length === 0) {
      throw new Error('SlackClient: botToken is required (Slack bot token starting with "xoxb-")');
    }
    if (!config.channelId || config.channelId.length === 0) {
      throw new Error('SlackClient: channelId is required (Slack channel id like "C0123...")');
    }
    if (!config.botToken.startsWith('xoxb-')) {
      // Soft guard — accepts any token shape Slack might issue, but
      // warns at construction since 99% of misconfigs land here.
      throw new Error(
        `SlackClient: botToken must start with "xoxb-" (got "${config.botToken.slice(0, 5)}..."). Use a bot token, not a user token or webhook URL.`
      );
    }
    if (!config.founderUserId || config.founderUserId.length === 0) {
      throw new Error(
        'SlackClient: founderUserId is required (Phase v0.5.1 Step 5 — friend-safe card branch needs to know who the founder is)'
      );
    }
    this.botToken = config.botToken;
    this.channelId = config.channelId;
    this.founderUserId = config.founderUserId;
    this.fetchImpl = config.fetchImpl ?? fetch;
  }

  channel(): string {
    return this.channelId;
  }

  async postFounderReviewCard(input: SlackPostInput): Promise<SlackPostResult> {
    const body = buildFounderReviewBlocks(input, this.channelId, this.founderUserId);

    let response: Response;
    try {
      response = await this.fetchImpl(SLACK_POSTMESSAGE_URL, {
        method: 'POST',
        headers: {
          authorization: `Bearer ${this.botToken}`,
          'content-type': 'application/json; charset=utf-8'
        },
        body: JSON.stringify(body)
      });
    } catch (err) {
      // Network-layer failure (DNS, connect, abort). Treat as retryable.
      throw new SlackApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }

    const httpStatus = response.status;
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      throw new SlackApiError(httpStatus, undefined, 'response was not valid JSON');
    }

    // 401/403 explicitly → auth error
    if (httpStatus === 401 || httpStatus === 403) {
      const code = providerCode(parsed);
      throw new SlackAuthError(httpStatus, code, code ?? `HTTP ${httpStatus}`);
    }

    // Non-200 HTTP → API error
    if (httpStatus < 200 || httpStatus >= 300) {
      throw new SlackApiError(httpStatus, providerCode(parsed), `HTTP ${httpStatus}`);
    }

    // HTTP 200 but Slack ok=false → API error (application-level)
    if (!isOkResponse(parsed)) {
      const code = providerCode(parsed);
      // invalid_auth / token_revoked / not_authed at the app layer
      // should also be treated as auth errors.
      if (code === 'invalid_auth' || code === 'token_revoked' || code === 'not_authed') {
        throw new SlackAuthError(401, code, code);
      }
      throw new SlackApiError(200, code, code ?? 'Slack returned ok=false with no error code');
    }

    const ts = stringField(parsed, 'ts');
    const channel = stringField(parsed, 'channel') ?? this.channelId;
    if (!ts) {
      throw new SlackApiError(200, undefined, 'Slack ok=true but response.ts was missing');
    }
    return Object.freeze({ ts, channel });
  }

  // Phase 3D.2: chat.update the original candidate-review card after
  // the founder clicks Approve or Reject. Same error semantics as
  // postFounderReviewCard (auth + api). Failures here are non-fatal
  // for the approval-capture caller — the alert state has already
  // transitioned by the time we call this; the update is purely
  // visual feedback in the channel.
  async updateFounderReviewCard(input: SlackUpdateInput): Promise<SlackPostResult> {
    const body = buildFounderReviewResolutionBlocks(input, this.founderUserId);
    let response: Response;
    try {
      response = await this.fetchImpl(SLACK_UPDATE_URL, {
        method: 'POST',
        headers: {
          authorization: `Bearer ${this.botToken}`,
          'content-type': 'application/json; charset=utf-8'
        },
        body: JSON.stringify(body)
      });
    } catch (err) {
      throw new SlackApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }
    const httpStatus = response.status;
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      throw new SlackApiError(httpStatus, undefined, 'response was not valid JSON');
    }
    if (httpStatus === 401 || httpStatus === 403) {
      const code = providerCode(parsed);
      throw new SlackAuthError(httpStatus, code, code ?? `HTTP ${httpStatus}`);
    }
    if (httpStatus < 200 || httpStatus >= 300) {
      throw new SlackApiError(httpStatus, providerCode(parsed), `HTTP ${httpStatus}`);
    }
    if (!isOkResponse(parsed)) {
      const code = providerCode(parsed);
      if (code === 'invalid_auth' || code === 'token_revoked' || code === 'not_authed') {
        throw new SlackAuthError(401, code, code);
      }
      throw new SlackApiError(200, code, code ?? 'Slack returned ok=false with no error code');
    }
    const ts = stringField(parsed, 'ts') ?? input.ts;
    const channel = stringField(parsed, 'channel') ?? input.channel;
    return Object.freeze({ ts, channel });
  }
}

function providerCode(body: unknown): string | undefined {
  if (body && typeof body === 'object' && 'error' in body) {
    const e = (body as { error: unknown }).error;
    return typeof e === 'string' ? e : undefined;
  }
  return undefined;
}

function isOkResponse(body: unknown): boolean {
  return !!body && typeof body === 'object' && 'ok' in body && (body as { ok: unknown }).ok === true;
}

function stringField(body: unknown, key: string): string | undefined {
  if (body && typeof body === 'object' && key in body) {
    const v = (body as Record<string, unknown>)[key];
    return typeof v === 'string' ? v : undefined;
  }
  return undefined;
}

/* ====================================================================== */
/* Block builder — single source of truth for the card shape              */
/* ====================================================================== */

interface SlackPostBody {
  readonly channel: string;
  readonly text: string;
  readonly blocks: readonly unknown[];
}

export function buildFounderReviewBlocks(
  input: SlackPostInput,
  channelId: string,
  founderUserId: string
): SlackPostBody {
  const { alert_id, user_id, view, rank } = input;
  // Fallback text for notifications (Slack mobile, screen readers).
  // Bounded; never includes body content.
  const fallback = `[FOMO] Candidate alert ${alert_id} — ${rank.label} (score ${rank.score})`;

  const senderLine = view.sender_name
    ? `${view.sender_name} <${view.sender_email_masked}>`
    : view.sender_email_masked;

  // Phase v0.5.1 Step 5 — friend-safe card branch.
  //
  // The rule (founder-locked 2026-05-29): if alert.user_id !==
  // founderUserId, render the friend-safe card UNCONDITIONALLY.
  // Privacy must NOT depend on FOMO_FRIEND_BETA_ENABLED — the kill
  // switch protects route surfaces (the /onboard route doesn't
  // mount), but if a friend-owned alert exists in the system for
  // any reason (manual SQL insert, half-rolled-back feature,
  // migration carryover, anything), the card must STILL be safe.
  //
  // Friend-safe card may include: sender, subject, ranker reason,
  // label, score.
  // Friend-safe card must NOT include: body excerpt, snippet,
  // attachment names, raw headers, raw email body, message_id (the
  // Gmail message id is a privacy identifier that would let the
  // founder cross-correlate friend emails — drop it).
  const is_friend_owned = user_id !== founderUserId;

  const blocks: unknown[] = [
    {
      type: 'header',
      text: { type: 'plain_text', text: 'FOMO — Candidate alert for founder review' }
    },
    {
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Sender*\n${senderLine}` },
        { type: 'mrkdwn', text: `*Subject*\n${view.subject}` }
      ]
    }
  ];

  if (!is_friend_owned) {
    // Founder-owned alert — full v0.1 card including the snippet
    // (the founder's own email; no third-party privacy concern).
    blocks.push({
      type: 'section',
      text: { type: 'mrkdwn', text: `*Snippet*\n${view.body_snippet}` }
    });
    blocks.push({
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Ranker label*\n${rank.label}` },
        { type: 'mrkdwn', text: `*Score*\n${rank.score}` },
        { type: 'mrkdwn', text: `*Model*\n${rank.model_name}` },
        { type: 'mrkdwn', text: `*Prompt*\n${rank.prompt_version}` }
      ]
    });
  } else {
    // Friend-owned alert — only label + score from the rank stats.
    // model / prompt_version are operational metadata; the founder
    // doesn't need them for the approve/reject decision.
    blocks.push({
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Ranker label*\n${rank.label}` },
        { type: 'mrkdwn', text: `*Score*\n${rank.score}` }
      ]
    });
  }

  blocks.push({
    type: 'section',
    text: { type: 'mrkdwn', text: `*Why*\n${rank.reason}` }
  });

  blocks.push({
    // Phase 3D.2: Approve / Reject interactive buttons. block_id carries
    // alert_id for routing — block_id is preserved verbatim in Slack's
    // interactivity payload, which lets the receiving route handler
    // recover alert_id without trusting the action.value field (which
    // could be tampered with by a misbuilt re-post). action_id encodes
    // the decision; the route handler validates both.
    type: 'actions',
    block_id: `fomo_alert:${alert_id}`,
    elements: [
      {
        type: 'button',
        action_id: 'fomo.approve',
        text: { type: 'plain_text', text: '✅ Approve' },
        style: 'primary',
        value: alert_id
      },
      {
        type: 'button',
        action_id: 'fomo.reject',
        text: { type: 'plain_text', text: '❌ Reject' },
        style: 'danger',
        value: alert_id
      }
    ]
  });

  // Footer context — alert_id always; user_id + received_at + message_id
  // only on founder-owned cards. The friend-safe card shows a
  // friend-tag so the operator can still tell at a glance who this
  // alert belongs to without revealing which Gmail message.
  if (!is_friend_owned) {
    blocks.push({
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: `alert_id: \`${alert_id}\` • user: \`${user_id}\` • received: ${view.received_at} • message_id: \`${view.message_id}\``
        }
      ]
    });
  } else {
    blocks.push({
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: `alert_id: \`${alert_id}\` • friend-owned (user redacted) • received: ${view.received_at}`
        }
      ]
    });
  }

  return Object.freeze({
    channel: channelId,
    text: fallback,
    blocks: Object.freeze(blocks)
  });
}

/* ====================================================================== */
/* updateFounderReviewCard — chat.update after approve/reject capture     */
/* ====================================================================== */

// Phase 3D.2: after the /slack/interactivity route captures a decision
// and transitions the alert, we edit the original card via chat.update
// so anyone in the channel sees the resolution. The new blocks REPLACE
// the actions block with a resolution context, but keep the original
// alert/rank/snippet content. Failures during update are non-fatal —
// the state transition has already landed, the chat.update is purely
// visual feedback.

export interface SlackUpdateInput {
  // Slack message ts returned from the original chat.postMessage call.
  // Stored in fomo.slack.posted audit detail; recovered by the route
  // handler before calling this method.
  readonly ts: string;
  // Channel id the message lives in.
  readonly channel: string;
  readonly alert_id: string;
  readonly user_id: string;
  readonly view: SlackEgressView;
  readonly rank: {
    readonly label: RankLabel;
    readonly score: number;
    readonly reason: string;
    readonly model_name: string;
    readonly prompt_version: string;
  };
  readonly decision: {
    // 'approved' | 'rejected'
    readonly kind: 'approved' | 'rejected';
    // ISO-8601 timestamp the decision was captured.
    readonly at: string;
    // Slack user-id of the founder/admin who clicked. Surfaced in the
    // card so anyone in the channel can see who decided.
    readonly actor: string;
  };
}

export function buildFounderReviewResolutionBlocks(
  input: SlackUpdateInput,
  founderUserId: string
): { readonly channel: string; readonly ts: string; readonly text: string; readonly blocks: readonly unknown[] } {
  const { ts, channel, alert_id, user_id, view, rank, decision } = input;
  const verb = decision.kind === 'approved' ? '✅ Approved' : '❌ Rejected';
  const fallback = `[FOMO] Alert ${alert_id} — ${verb} by ${decision.actor}`;

  const senderLine = view.sender_name
    ? `${view.sender_name} <${view.sender_email_masked}>`
    : view.sender_email_masked;

  // Phase v0.5.1 Step 5 — same UNCONDITIONAL friend-safe branch as
  // buildFounderReviewBlocks. The resolution card is what shows in
  // the Slack channel AFTER the founder clicks Approve/Reject; if
  // the original alert was friend-owned, the resolution card must
  // also be friend-safe.
  const is_friend_owned = user_id !== founderUserId;

  const blocks: unknown[] = [
    {
      type: 'header',
      text: { type: 'plain_text', text: `FOMO — Alert ${verb.toLowerCase()}` }
    },
    {
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Sender*\n${senderLine}` },
        { type: 'mrkdwn', text: `*Subject*\n${view.subject}` }
      ]
    }
  ];

  if (!is_friend_owned) {
    blocks.push({
      type: 'section',
      text: { type: 'mrkdwn', text: `*Snippet*\n${view.body_snippet}` }
    });
    blocks.push({
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Ranker label*\n${rank.label}` },
        { type: 'mrkdwn', text: `*Score*\n${rank.score}` },
        { type: 'mrkdwn', text: `*Model*\n${rank.model_name}` },
        { type: 'mrkdwn', text: `*Prompt*\n${rank.prompt_version}` }
      ]
    });
  } else {
    blocks.push({
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Ranker label*\n${rank.label}` },
        { type: 'mrkdwn', text: `*Score*\n${rank.score}` }
      ]
    });
  }

  blocks.push({
    type: 'section',
    text: { type: 'mrkdwn', text: `*Why (ranker)*\n${rank.reason}` }
  });
  blocks.push({
    type: 'section',
    text: {
      type: 'mrkdwn',
      text: `*${verb}* by <@${decision.actor}> at \`${decision.at}\`.`
    }
  });

  if (!is_friend_owned) {
    blocks.push({
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: `alert_id: \`${alert_id}\` • user: \`${user_id}\` • received: ${view.received_at} • message_id: \`${view.message_id}\``
        }
      ]
    });
  } else {
    blocks.push({
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: `alert_id: \`${alert_id}\` • friend-owned (user redacted) • received: ${view.received_at}`
        }
      ]
    });
  }

  return Object.freeze({ channel, ts, text: fallback, blocks: Object.freeze(blocks) });
}

/* ====================================================================== */
/* Slack signing-secret verification (Phase 3D.2)                         */
/* ====================================================================== */

import { createHmac, timingSafeEqual } from 'node:crypto';

export interface VerifySignatureInput {
  // The signing secret from your Slack app's Basic Information panel.
  // NOT the bot token. Different secret, different rotation lifecycle.
  readonly signingSecret: string;
  // X-Slack-Request-Timestamp header (unix seconds, as a string).
  readonly timestamp: string;
  // X-Slack-Signature header. Format: 'v0=<hex>'.
  readonly signature: string;
  // Raw request body (NOT JSON-parsed; the original bytes).
  readonly body: string;
  // Optional override for the "now" clock — tests inject a fixed time.
  readonly now?: () => number;
  // Optional override for the freshness window in seconds. Slack
  // recommends 300 (5 min); we honor that as default. Tests can shorten.
  readonly maxAgeSeconds?: number;
}

export type SignatureVerificationResult =
  | { readonly ok: true }
  | { readonly ok: false; readonly reason: 'malformed_timestamp' | 'malformed_signature' | 'stale_timestamp' | 'signature_mismatch' };

// Verify an inbound Slack request per
// https://api.slack.com/authentication/verifying-requests-from-slack
//
// Format: v0=hex(hmac_sha256(signingSecret, `v0:${timestamp}:${body}`)).
// Timing-safe compare. Reject if timestamp is older than maxAgeSeconds
// (default 300s) to thwart replay attacks. Caller is responsible for
// reading the RAW body before JSON parsing; Node's req.body parsing
// would lose the original bytes the HMAC was computed over.
export function verifySlackSignature(input: VerifySignatureInput): SignatureVerificationResult {
  const now = input.now ?? (() => Math.floor(Date.now() / 1000));
  const maxAge = input.maxAgeSeconds ?? 300;

  if (!/^\d{8,12}$/.test(input.timestamp)) {
    return Object.freeze({ ok: false as const, reason: 'malformed_timestamp' as const });
  }
  if (!/^v0=[a-f0-9]{64}$/.test(input.signature)) {
    return Object.freeze({ ok: false as const, reason: 'malformed_signature' as const });
  }
  const ts = Number(input.timestamp);
  if (Math.abs(now() - ts) > maxAge) {
    return Object.freeze({ ok: false as const, reason: 'stale_timestamp' as const });
  }

  const basestring = `v0:${input.timestamp}:${input.body}`;
  const expectedHex = createHmac('sha256', input.signingSecret).update(basestring).digest('hex');
  const expected = Buffer.from(`v0=${expectedHex}`, 'utf8');
  const provided = Buffer.from(input.signature, 'utf8');
  if (expected.length !== provided.length) {
    return Object.freeze({ ok: false as const, reason: 'signature_mismatch' as const });
  }
  if (!timingSafeEqual(expected, provided)) {
    return Object.freeze({ ok: false as const, reason: 'signature_mismatch' as const });
  }
  return Object.freeze({ ok: true as const });
}
