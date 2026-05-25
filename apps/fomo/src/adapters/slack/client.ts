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
  readonly fetchImpl?: FetchLike;
}

export class SlackClient {
  private readonly botToken: string;
  private readonly channelId: string;
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
    this.botToken = config.botToken;
    this.channelId = config.channelId;
    this.fetchImpl = config.fetchImpl ?? fetch;
  }

  channel(): string {
    return this.channelId;
  }

  async postFounderReviewCard(input: SlackPostInput): Promise<SlackPostResult> {
    const body = buildFounderReviewBlocks(input, this.channelId);

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
  channelId: string
): SlackPostBody {
  const { alert_id, user_id, view, rank } = input;
  // Fallback text for notifications (Slack mobile, screen readers).
  // Bounded; never includes body content.
  const fallback = `[FOMO] Candidate alert ${alert_id} — ${rank.label} (score ${rank.score})`;

  const senderLine = view.sender_name
    ? `${view.sender_name} <${view.sender_email_masked}>`
    : view.sender_email_masked;

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
    },
    {
      type: 'section',
      text: { type: 'mrkdwn', text: `*Snippet*\n${view.body_snippet}` }
    },
    {
      type: 'section',
      fields: [
        { type: 'mrkdwn', text: `*Ranker label*\n${rank.label}` },
        { type: 'mrkdwn', text: `*Score*\n${rank.score}` },
        { type: 'mrkdwn', text: `*Model*\n${rank.model_name}` },
        { type: 'mrkdwn', text: `*Prompt*\n${rank.prompt_version}` }
      ]
    },
    {
      type: 'section',
      text: { type: 'mrkdwn', text: `*Why*\n${rank.reason}` }
    },
    {
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: `alert_id: \`${alert_id}\` • user: \`${user_id}\` • received: ${view.received_at} • message_id: \`${view.message_id}\``
        }
      ]
    },
    {
      type: 'context',
      elements: [
        {
          type: 'mrkdwn',
          text: '_Phase 3D.1 — posting only. Approve / reject capture lands in 3D.2. Alert remains in `queued_for_review` until then._'
        }
      ]
    }
  ];

  return Object.freeze({
    channel: channelId,
    text: fallback,
    blocks: Object.freeze(blocks)
  });
}
