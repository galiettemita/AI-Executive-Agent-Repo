// SendBlue HTTP client — outbound-only POST /api/send-message for the
// Phase 3E.1 founder iMessage send path.
//
// 3E.1 scope: send a deterministic, template-rendered text to ONE
// allowlisted phone number (the founder's). No reply parsing, no
// webhook handling, no per-conversation context, no Android/SMS
// fallback logic in our code. The provider response decides the
// outcome.
//
// Three-outcome semantics (founder directive 2026-05-25):
//
//   * `sent`              — clear provider success. Use only when the
//                           response is HTTP 2xx AND the parsed body's
//                           top-level status is one of QUEUED / SENT /
//                           DELIVERED. Caller transitions alert
//                           approved → sent.
//   * `send_status_unknown` — ambiguous outcome. Caller transitions
//                           approved → send_status_unknown and DOES
//                           NOT auto-retry. Sources:
//                             - network/abort failure
//                             - HTTP 5xx
//                             - HTTP 200 with unrecognized provider
//                               status string (FAILED / ERROR / etc.
//                               are 'failed'; anything ELSE that isn't
//                               a known success is 'unknown')
//                             - parsable but missing required fields
//                             - HTTP 429
//                           Auto-retrying ambiguous sends risks
//                           delivering real iMessages twice.
//   * `failed`            — clear provider failure. Use when the
//                           response is HTTP 2xx with status FAILED /
//                           ERROR, or any HTTP 4xx that is NOT a
//                           transient/auth error. Caller transitions
//                           approved → failed.
//
// Auth errors (401/403, invalid_api_key) surface as `failed` because
// the operator must intervene; retrying is meaningless until the API
// key is rotated.
//
// Design choices mirror SlackClient:
//   * Direct fetch, no SDK. Same auditable shape.
//   * Injectable FetchLike — tests inject a mock; CI never hits real
//     SendBlue.
//   * Fail-closed at construction: apiKeyId / apiSecretKey / fromNumber
//     (allowlisted founder number) all required.
//   * Body never includes our internal alert_id — only what SendBlue
//     needs (number + content). Our identifiers stay in audit/state.
//
// SendBlue's exact response schema is finalized when the founder
// provisions an account during 3E.2; the parser below treats every
// field defensively. Fields we don't recognize fall to send_status_unknown.

export type FetchLike = typeof fetch;

const SENDBLUE_SEND_URL = 'https://api.sendblue.co/api/send-message';

// Clear-success indicators per the SendBlue docs. These are the only
// status strings we treat as terminal `sent`. Everything else returned
// with HTTP 2xx maps to either `failed` (explicit FAILED/ERROR) or
// `send_status_unknown` (anything we don't recognize).
const PROVIDER_SUCCESS_STATUSES: ReadonlySet<string> = new Set([
  'QUEUED',
  'SENT',
  'DELIVERED'
]);

const PROVIDER_FAILURE_STATUSES: ReadonlySet<string> = new Set([
  'FAILED',
  'ERROR'
]);

export class SendBlueAuthError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(`SendBlue auth error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'SendBlueAuthError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
  }
}

export interface SendInput {
  // E.164-formatted destination (e.g. +14155551234). Caller is
  // responsible for the founder-phone allowlist check BEFORE handing
  // a value here; this client trusts that gate.
  readonly to: string;
  // The deterministic, template-rendered text. Bounded by the caller
  // (founder-text-template renders ≤280 chars).
  readonly content: string;
}

export type SendOutcomeKind = 'sent' | 'failed' | 'send_status_unknown';

export interface SendOutcome {
  readonly kind: SendOutcomeKind;
  // The provider's top-level status field (e.g. 'QUEUED', 'FAILED',
  // 'ERROR'), if present. Used by the caller for audit detail; never
  // user-facing text.
  readonly providerStatus: string | undefined;
  // Provider's opaque message handle on success (so the caller can
  // audit and, in a future phase, correlate inbound replies). Empty
  // string when not present.
  readonly providerMessageHandle: string;
  // Provider's HTTP status code, or 0 when the request never reached
  // an HTTP boundary (network/abort).
  readonly httpStatus: number;
  // Operator-facing diagnostic — short string describing why this
  // outcome was chosen. NEVER includes the rendered message content.
  readonly reason: string;
}

export interface SendBlueClientConfig {
  // API key id from the SendBlue dashboard.
  readonly apiKeyId: string;
  // API secret key from the SendBlue dashboard.
  readonly apiSecretKey: string;
  // SendBlue-assigned sender phone number (E.164, e.g. +12143547196).
  // REQUIRED by SendBlue's /api/send-message endpoint — the API
  // returns HTTP 400 `missing required parameter: "from_number"`
  // without it. Surfaced as a 3E.2 smoke-test finding (mock tests
  // didn't catch it because the synthetic responses ignored body
  // shape).
  readonly fromNumber: string;
  readonly fetchImpl?: FetchLike;
  // Optional override for the request timeout in ms. Default 30s.
  // SendBlue's free-tier endpoint can take ~13s to respond (3E.2
  // smoke-test diagnostic); the prior 10s default timed out before
  // the response arrived. On timeout the outcome is
  // 'send_status_unknown' (per founder directive — never auto-retry
  // ambiguous).
  readonly timeoutMs?: number;
}

export class SendBlueClient {
  private readonly apiKeyId: string;
  private readonly apiSecretKey: string;
  private readonly fromNumber: string;
  private readonly fetchImpl: FetchLike;
  private readonly timeoutMs: number;

  constructor(config: SendBlueClientConfig) {
    if (!config.apiKeyId || config.apiKeyId.length === 0) {
      throw new Error('SendBlueClient: apiKeyId is required');
    }
    if (!config.apiSecretKey || config.apiSecretKey.length === 0) {
      throw new Error('SendBlueClient: apiSecretKey is required');
    }
    if (!config.fromNumber || config.fromNumber.length === 0) {
      throw new Error(
        'SendBlueClient: fromNumber is required (E.164, e.g. +12143547196 — your SendBlue-assigned sender number)'
      );
    }
    if (!/^\+\d{7,15}$/.test(config.fromNumber)) {
      throw new Error(
        `SendBlueClient: fromNumber must be E.164 format (got '${config.fromNumber.slice(0, 4)}...'). Expected '+' followed by 7-15 digits.`
      );
    }
    this.apiKeyId = config.apiKeyId;
    this.apiSecretKey = config.apiSecretKey;
    this.fromNumber = config.fromNumber;
    this.fetchImpl = config.fetchImpl ?? fetch;
    const t = config.timeoutMs ?? 30_000;
    if (!Number.isInteger(t) || t <= 0) {
      throw new Error(`SendBlueClient: timeoutMs must be a positive integer (got ${t})`);
    }
    this.timeoutMs = t;
  }

  async send(input: SendInput): Promise<SendOutcome> {
    if (!input.to || typeof input.to !== 'string' || input.to.length === 0) {
      // Caller-side argument errors are surfaced as `failed` — not
      // unknown. There is no provider call to be ambiguous about.
      return Object.freeze({
        kind: 'failed' as const,
        providerStatus: undefined,
        providerMessageHandle: '',
        httpStatus: 0,
        reason: 'argument_error: missing destination'
      });
    }
    if (!input.content || typeof input.content !== 'string' || input.content.length === 0) {
      return Object.freeze({
        kind: 'failed' as const,
        providerStatus: undefined,
        providerMessageHandle: '',
        httpStatus: 0,
        reason: 'argument_error: missing content'
      });
    }

    const ac = new AbortController();
    const timer = setTimeout(() => ac.abort(), this.timeoutMs);

    let response: Response;
    try {
      response = await this.fetchImpl(SENDBLUE_SEND_URL, {
        method: 'POST',
        headers: {
          'sb-api-key-id': this.apiKeyId,
          'sb-api-secret-key': this.apiSecretKey,
          'content-type': 'application/json'
        },
        body: JSON.stringify({
          number: input.to,
          content: input.content,
          from_number: this.fromNumber
        }),
        signal: ac.signal
      });
    } catch (err) {
      // Network failure / DNS / abort / timeout. Treat as ambiguous —
      // we DON'T know whether the request reached SendBlue. Per founder
      // directive, the caller must NOT auto-retry.
      const reason = ac.signal.aborted
        ? `timeout after ${this.timeoutMs}ms`
        : err instanceof Error
          ? `network_error: ${err.message}`
          : `network_error: ${String(err)}`;
      return Object.freeze({
        kind: 'send_status_unknown' as const,
        providerStatus: undefined,
        providerMessageHandle: '',
        httpStatus: 0,
        reason
      });
    } finally {
      clearTimeout(timer);
    }

    const httpStatus = response.status;
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      // 2xx with unparseable body → unknown; we don't know if it sent.
      // 4xx/5xx with unparseable body → keep the original signal.
      if (httpStatus >= 200 && httpStatus < 300) {
        return Object.freeze({
          kind: 'send_status_unknown' as const,
          providerStatus: undefined,
          providerMessageHandle: '',
          httpStatus,
          reason: 'response_unparseable: 2xx but body not valid JSON'
        });
      }
      parsed = null;
    }

    // 401/403/Forbidden → clear auth failure. NOT retryable; operator
    // must rotate keys. We surface as `failed` (not unknown) because
    // the message did NOT go out.
    if (httpStatus === 401 || httpStatus === 403) {
      return Object.freeze({
        kind: 'failed' as const,
        providerStatus: providerStatus(parsed),
        providerMessageHandle: '',
        httpStatus,
        reason: `auth_error: HTTP ${httpStatus} ${providerCode(parsed) ?? ''}`.trim()
      });
    }

    // 429 → ambiguous. SendBlue may have accepted-and-queued or rejected
    // before processing. Caller MUST NOT auto-retry.
    if (httpStatus === 429) {
      return Object.freeze({
        kind: 'send_status_unknown' as const,
        providerStatus: providerStatus(parsed),
        providerMessageHandle: '',
        httpStatus,
        reason: 'rate_limited: HTTP 429'
      });
    }

    // 5xx → ambiguous. Same reasoning as 429.
    if (httpStatus >= 500) {
      return Object.freeze({
        kind: 'send_status_unknown' as const,
        providerStatus: providerStatus(parsed),
        providerMessageHandle: '',
        httpStatus,
        reason: `provider_server_error: HTTP ${httpStatus}`
      });
    }

    // Other 4xx → clear failure.
    if (httpStatus >= 400) {
      return Object.freeze({
        kind: 'failed' as const,
        providerStatus: providerStatus(parsed),
        providerMessageHandle: '',
        httpStatus,
        reason: `client_error: HTTP ${httpStatus} ${providerCode(parsed) ?? ''}`.trim()
      });
    }

    // 2xx — inspect the parsed body for the provider's status field.
    const status = providerStatus(parsed);
    const handle = providerMessageHandle(parsed);

    if (status && PROVIDER_SUCCESS_STATUSES.has(status)) {
      return Object.freeze({
        kind: 'sent' as const,
        providerStatus: status,
        providerMessageHandle: handle,
        httpStatus,
        reason: `provider_status=${status}`
      });
    }
    if (status && PROVIDER_FAILURE_STATUSES.has(status)) {
      return Object.freeze({
        kind: 'failed' as const,
        providerStatus: status,
        providerMessageHandle: handle,
        httpStatus,
        reason: `provider_status=${status}`
      });
    }

    // 2xx with an unrecognized or missing status — DO NOT assume sent.
    // Per founder directive, ambiguous outcomes never auto-retry.
    return Object.freeze({
      kind: 'send_status_unknown' as const,
      providerStatus: status,
      providerMessageHandle: handle,
      httpStatus,
      reason: status
        ? `unknown_provider_status: ${status}`
        : 'missing_provider_status_field'
    });
  }
}

function providerStatus(body: unknown): string | undefined {
  if (!body || typeof body !== 'object') return undefined;
  // SendBlue's documented top-level field is `status` per their API
  // docs. We tolerate `message_status` as an alias because earlier
  // versions of their docs used that name.
  if ('status' in body) {
    const v = (body as { status: unknown }).status;
    return typeof v === 'string' ? v : undefined;
  }
  if ('message_status' in body) {
    const v = (body as { message_status: unknown }).message_status;
    return typeof v === 'string' ? v : undefined;
  }
  return undefined;
}

function providerMessageHandle(body: unknown): string {
  if (!body || typeof body !== 'object') return '';
  if ('message_handle' in body) {
    const v = (body as { message_handle: unknown }).message_handle;
    if (typeof v === 'string') return v;
  }
  if ('messageHandle' in body) {
    const v = (body as { messageHandle: unknown }).messageHandle;
    if (typeof v === 'string') return v;
  }
  return '';
}

function providerCode(body: unknown): string | undefined {
  if (!body || typeof body !== 'object') return undefined;
  if ('error' in body) {
    const v = (body as { error: unknown }).error;
    return typeof v === 'string' ? v : undefined;
  }
  if ('error_code' in body) {
    const v = (body as { error_code: unknown }).error_code;
    return typeof v === 'string' ? v : undefined;
  }
  return undefined;
}
