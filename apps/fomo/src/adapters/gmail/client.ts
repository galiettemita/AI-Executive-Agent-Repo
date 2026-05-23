// Gmail HTTP client — read-only access to the Gmail REST API.
//
// FOMO_PLAN §9.1 + §9.2: v0.1 uses `gmail.readonly` scope only. No
// sending, no draft, no label changes. Phase 3B.1 ships the client +
// OAuth go-live + cursor store. Phase 3B.2 wires it to a polling worker
// and the gmail.read tool executor.
//
// Design choices:
//   * Direct fetch, no `googleapis` SDK. Mirrors oauth-exchange.ts.
//     Keeps the lockfile small and the call surface auditable.
//   * Injectable FetchLike — tests inject a mock; CI never hits the
//     real Gmail API.
//   * Caller-supplied access_token; the client does NOT manage tokens
//     (that lives in the OAuth substrate). On 401 the client throws
//     GmailUnauthorizedError so the caller can refresh and retry.
//   * Returns a stable shape matching RawEmailContext (egress-policy.ts)
//     so the polling worker in Phase 3B.2 can hand messages straight
//     to the egress layer.

import { type RawEmailContext } from '../../core/egress-policy.js';

export type FetchLike = typeof fetch;

// Read-only scope. Hardcoded — v0.1 has no other Gmail scope.
export const GMAIL_READONLY_SCOPE = 'https://www.googleapis.com/auth/gmail.readonly';

const GMAIL_API_BASE = 'https://gmail.googleapis.com/gmail/v1';

export class GmailUnauthorizedError extends Error {
  readonly httpStatus: 401;
  constructor(reason: string) {
    super(`Gmail returned 401: ${reason}`);
    this.name = 'GmailUnauthorizedError';
    this.httpStatus = 401;
  }
}

export class GmailApiError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  readonly retryable: boolean;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(`Gmail API error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'GmailApiError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
    this.retryable = httpStatus >= 500 || httpStatus === 429;
  }
}

/* ---------------------------------------------------------------------- */
/* Response shapes (only fields v0.1 needs)                               */
/* ---------------------------------------------------------------------- */

export interface GmailProfile {
  readonly emailAddress: string;
  readonly historyId: string;
  readonly messagesTotal: number;
  readonly threadsTotal: number;
}

export interface GmailHistoryItem {
  readonly id: string; // history_id
  readonly messagesAdded?: ReadonlyArray<{ readonly message: { readonly id: string; readonly threadId: string } }>;
}

export interface GmailHistoryListResult {
  // Tuple of (new history_id, message_ids appearing under messagesAdded).
  // Empty list when there are no changes since the cursor.
  readonly latest_history_id: string;
  readonly added_message_ids: readonly string[];
}

/* ---------------------------------------------------------------------- */
/* GmailClient                                                            */
/* ---------------------------------------------------------------------- */

export interface GmailClientConfig {
  // Override fetch for testing. Defaults to global fetch.
  readonly fetchImpl?: FetchLike;
  // Per-request timeout in ms. Defaults to 30s.
  readonly timeoutMs?: number;
}

export class GmailClient {
  private readonly fetchImpl: FetchLike;
  private readonly timeoutMs: number;

  constructor(config: GmailClientConfig = {}) {
    this.fetchImpl = config.fetchImpl ?? fetch;
    this.timeoutMs = config.timeoutMs ?? 30_000;
  }

  async getProfile(accessToken: string): Promise<GmailProfile> {
    const url = `${GMAIL_API_BASE}/users/me/profile`;
    const json = await this.get(url, accessToken);
    return Object.freeze({
      emailAddress: String((json as Record<string, unknown>).emailAddress ?? ''),
      historyId: String((json as Record<string, unknown>).historyId ?? '0'),
      messagesTotal: Number((json as Record<string, unknown>).messagesTotal ?? 0),
      threadsTotal: Number((json as Record<string, unknown>).threadsTotal ?? 0)
    });
  }

  // Incremental polling: list history items since the start_history_id.
  // Returns the latest history_id (use as next cursor) and message ids
  // that appeared under messagesAdded. Subsequent calls should use the
  // returned latest_history_id as the next start_history_id.
  async listHistorySince(
    accessToken: string,
    startHistoryId: string,
    opts: { readonly maxResults?: number } = {}
  ): Promise<GmailHistoryListResult> {
    const params = new URLSearchParams({
      startHistoryId,
      historyTypes: 'messageAdded',
      maxResults: String(opts.maxResults ?? 100)
    });
    const url = `${GMAIL_API_BASE}/users/me/history?${params.toString()}`;
    const json = await this.get(url, accessToken);
    const root = json as Record<string, unknown>;
    const items = (root.history as GmailHistoryItem[] | undefined) ?? [];
    const addedIds: string[] = [];
    for (const item of items) {
      for (const m of item.messagesAdded ?? []) {
        addedIds.push(m.message.id);
      }
    }
    const latestHistoryId = String(root.historyId ?? startHistoryId);
    return Object.freeze({
      latest_history_id: latestHistoryId,
      added_message_ids: Object.freeze(addedIds)
    });
  }

  // Fetch a single message and project it into RawEmailContext. The
  // shape is what the Egress Policy expects.
  async getMessage(accessToken: string, messageId: string): Promise<RawEmailContext> {
    const url = `${GMAIL_API_BASE}/users/me/messages/${encodeURIComponent(messageId)}?format=full`;
    const json = await this.get(url, accessToken);
    return projectGmailMessage(json);
  }

  /* ----- internal ----- */

  private async get(url: string, accessToken: string): Promise<unknown> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);
    let res: Response;
    try {
      res = await this.fetchImpl(url, {
        method: 'GET',
        headers: {
          authorization: `Bearer ${accessToken}`,
          accept: 'application/json'
        },
        signal: controller.signal
      });
    } catch (err) {
      clearTimeout(timer);
      throw new GmailApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }
    clearTimeout(timer);
    if (res.status === 401) {
      throw new GmailUnauthorizedError('access token rejected');
    }
    let body: unknown;
    try {
      body = await res.json();
    } catch {
      if (!res.ok) {
        throw new GmailApiError(res.status, undefined, `non-JSON response`);
      }
      return {};
    }
    if (!res.ok) {
      const errObj = (body as { error?: { code?: number; message?: string; status?: string } }).error;
      throw new GmailApiError(
        res.status,
        errObj?.status,
        errObj?.message ?? `HTTP ${res.status}`
      );
    }
    return body;
  }
}

/* ---------------------------------------------------------------------- */
/* projectGmailMessage — Gmail REST shape → RawEmailContext               */
/* ---------------------------------------------------------------------- */

interface GmailMessagePart {
  readonly mimeType?: string;
  readonly filename?: string;
  readonly body?: { readonly size?: number; readonly data?: string };
  readonly parts?: readonly GmailMessagePart[];
}

interface GmailMessage {
  readonly id: string;
  readonly threadId?: string;
  readonly internalDate?: string;
  readonly payload?: {
    readonly headers?: ReadonlyArray<{ readonly name: string; readonly value: string }>;
    readonly mimeType?: string;
    readonly body?: { readonly data?: string };
    readonly parts?: readonly GmailMessagePart[];
  };
}

function findHeader(headers: ReadonlyArray<{ name: string; value: string }> | undefined, name: string): string {
  if (!headers) return '';
  const lower = name.toLowerCase();
  for (const h of headers) {
    if (h.name.toLowerCase() === lower) return h.value;
  }
  return '';
}

function decodeBase64Url(data: string | undefined): string {
  if (!data) return '';
  const normalized = data.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4);
  try {
    return Buffer.from(padded, 'base64').toString('utf8');
  } catch {
    return '';
  }
}

// Walks parts depth-first; returns first text/plain decoded body found.
function extractPlainBody(payload: GmailMessage['payload']): string {
  if (!payload) return '';
  const visit = (part: GmailMessagePart | NonNullable<GmailMessage['payload']>): string | null => {
    if (part.mimeType === 'text/plain' && part.body?.data) {
      return decodeBase64Url(part.body.data);
    }
    const parts = (part as GmailMessagePart).parts ?? (part as NonNullable<GmailMessage['payload']>).parts;
    for (const sub of parts ?? []) {
      const found = visit(sub);
      if (found !== null) return found;
    }
    return null;
  };
  return visit(payload) ?? '';
}

function extractHtmlBody(payload: GmailMessage['payload']): string | undefined {
  if (!payload) return undefined;
  const visit = (part: GmailMessagePart | NonNullable<GmailMessage['payload']>): string | null => {
    if (part.mimeType === 'text/html' && part.body?.data) {
      return decodeBase64Url(part.body.data);
    }
    const parts = (part as GmailMessagePart).parts ?? (part as NonNullable<GmailMessage['payload']>).parts;
    for (const sub of parts ?? []) {
      const found = visit(sub);
      if (found !== null) return found;
    }
    return null;
  };
  const result = visit(payload);
  return result === null ? undefined : result;
}

function extractAttachments(payload: GmailMessage['payload']): { filename: string; size_bytes: number }[] {
  if (!payload) return [];
  const result: { filename: string; size_bytes: number }[] = [];
  const visit = (part: GmailMessagePart): void => {
    if (part.filename && part.filename.length > 0) {
      result.push({ filename: part.filename, size_bytes: part.body?.size ?? 0 });
    }
    for (const sub of part.parts ?? []) visit(sub);
  };
  for (const sub of payload.parts ?? []) visit(sub);
  return result;
}

function parseFromHeader(rawFrom: string): { email: string; name: string | undefined } {
  const trimmed = rawFrom.trim();
  // Form 1: 'Name <email@x>' or '"Name" <email@x>'
  const withName = /^"?([^"<]+?)"?\s*<([^>]+)>$/.exec(trimmed);
  if (withName) {
    const name = withName[1]?.trim();
    const email = withName[2]?.trim() ?? '';
    return { email, name: name && name.length > 0 ? name : undefined };
  }
  // Form 2: bare 'email@x' with no display name.
  return { email: trimmed, name: undefined };
}

export function projectGmailMessage(raw: unknown): RawEmailContext {
  const msg = raw as GmailMessage;
  const headers = msg.payload?.headers;
  const from = findHeader(headers, 'From');
  const subject = findHeader(headers, 'Subject');
  const { email, name } = parseFromHeader(from);
  const body_plain = extractPlainBody(msg.payload);
  const body_html = extractHtmlBody(msg.payload);
  const attachments = extractAttachments(msg.payload);
  const internalDateMs = msg.internalDate ? Number(msg.internalDate) : Date.now();
  const headerMap: Record<string, string> = {};
  for (const h of headers ?? []) headerMap[h.name] = h.value;

  return Object.freeze({
    message_id: msg.id,
    thread_id: msg.threadId,
    sender_email: email,
    sender_name: name,
    subject,
    body_plain,
    body_html,
    headers: headerMap,
    attachments: Object.freeze(attachments.map((a) => Object.freeze({ ...a }))),
    received_at: new Date(internalDateMs)
  });
}
