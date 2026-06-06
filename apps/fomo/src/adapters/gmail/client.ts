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
  // Phase v0.5.8 — Q1.A: Gmail history.list now includes labelAdded events
  // when historyTypes='messageAdded,labelAdded'. The shape mirrors Gmail's
  // REST contract: each labelsAdded entry carries the affected message ref
  // plus the labels that were added. Q2.A accepts the event ONLY when
  // labelIds includes the literal 'INBOX' system label. labelIds is
  // optional+nullable per Gmail's API drift tolerance; Q5 malformed-skip
  // catches the null/undefined/non-array case.
  readonly labelsAdded?: ReadonlyArray<{
    readonly message: { readonly id: string; readonly threadId: string };
    readonly labelIds?: readonly string[] | null;
  }>;
}

// Phase v0.5.8 — per-message provenance carried back to the worker so it
// can emit `fomo.gmail.poll.event_observed` with the structural Q6.A fields
// AND compute the four cycle-level counters. The provenance map is keyed
// by message_id; each entry records WHICH event type(s) surfaced the
// message in this cursor span. After the Q3.A first-seen-wins dedupe,
// added_message_ids contains each id exactly once, in first-seen order;
// the provenance map can mark BOTH via flags true when a second event-
// type sighting occurred for the same id (that's the "dedupe drop" — the
// id was NOT re-added to added_message_ids, but the provenance reflects
// the multi-event observation).
export interface GmailEventProvenance {
  readonly via_messageAdded: boolean;
  readonly via_labelAdded_inbox: boolean;
}

export interface GmailHistoryListResult {
  // Tuple of (new history_id, message_ids appearing under EITHER
  // messagesAdded OR labelAdded:INBOX events). Empty list when there are
  // no changes since the cursor.
  //
  // Phase v0.5.8 Q3.A — added_message_ids is POST-DEDUPE UNIQUE. Each
  // message_id appears at most once in this list, in the order it was
  // first observed (first-seen wins). The worker treats this list exactly
  // as it did in v0.5.7 (iterate, dispatch each).
  readonly latest_history_id: string;
  readonly added_message_ids: readonly string[];
  // Phase v0.5.8 Q6.A — per-message event-type provenance. Keyed by
  // message_id; covers every id in added_message_ids (no extra keys; no
  // missing keys). The worker reads this to populate the new
  // fomo.gmail.poll.event_observed audit detail and to compute the four
  // cycle-level counters (*_messageAdded_only, *_labelAdded_only, *_both,
  // dedupe_drops).
  readonly event_provenance: ReadonlyMap<string, GmailEventProvenance>;
  // Phase v0.5.8 Q5 — count of labelAdded events that arrived without a
  // valid `labelIds` array (Gmail malformed). Best-effort: the parser
  // silently skips these and reports the count so the worker can emit
  // `fomo.gmail.poll.event_skipped` audits (one per malformed event) for
  // operator visibility. NO retry per Q5.
  readonly malformed_labelAdded_skipped: number;
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
  // that appeared under EITHER messagesAdded OR labelAdded:INBOX events.
  // Subsequent calls should use the returned latest_history_id as the
  // next start_history_id.
  //
  // Phase v0.5.8 Q1.A — historyTypes='messageAdded,labelAdded' (single
  // comma-list call). Gmail records BOTH event types for new INBOX mail:
  //   * messageAdded only — typical external delivery
  //   * labelAdded:INBOX only — Gmail-to-self self-sends (the message
  //     exists in Sent the moment Send is clicked; only labelAdded:INBOX
  //     fires when it lands in INBOX). v0.5.7 baseline NEVER surfaced
  //     these.
  //   * both — Gmail batched both events into one cursor span
  // The single call preserves cursor advancement semantics exactly; Gmail
  // returns both event types in the same history items array.
  //
  // Phase v0.5.8 Q2.A — labelAdded events are accepted ONLY when the
  // added labelIds include the literal 'INBOX' (a reserved Gmail system
  // label; no per-user lookup). Other label-additions (STARRED, IMPORTANT,
  // custom user labels) are silently ignored — they are not Brevio's
  // concern and surfacing them as observations would be noise.
  //
  // Phase v0.5.8 Q3.A — per-cycle Set<message_id> dedupe; first-seen wins.
  // The same message_id arriving via BOTH messagesAdded AND labelAdded in
  // the same cursor span produces exactly ONE entry in added_message_ids
  // (the first event-type observed) and ONE entry in event_provenance
  // with both via flags true. The DB-side rank_results.UNIQUE(user_id,
  // message_id) constraint is the load-bearing fallback for cross-cycle
  // dedupe (Q4.A — no new persistence).
  //
  // Phase v0.5.8 Q5 — malformed labelAdded events (missing/invalid
  // labelIds field) are silently skipped; the count is returned to the
  // worker so it can emit one `fomo.gmail.poll.event_skipped` audit per
  // skipped event. NO retry. INBOX-removal-after-addition in the same
  // cursor span is processed as a normal observation (let the user STOP
  // rather than silently drop).
  async listHistorySince(
    accessToken: string,
    startHistoryId: string,
    opts: { readonly maxResults?: number } = {}
  ): Promise<GmailHistoryListResult> {
    // Gmail expects historyTypes as REPEATED params, not a comma-joined
    // string. Comma-joined produces a 400 INVALID_ARGUMENT — Gmail decodes
    // the whole value as one enum literal. v0.5.8 smoke 2026-06-06 caught
    // this against real Gmail; unit test C4 had the same wrong assumption.
    const params = new URLSearchParams({
      startHistoryId,
      maxResults: String(opts.maxResults ?? 100)
    });
    params.append('historyTypes', 'messageAdded');
    params.append('historyTypes', 'labelAdded');
    const url = `${GMAIL_API_BASE}/users/me/history?${params.toString()}`;
    const json = await this.get(url, accessToken);
    const root = json as Record<string, unknown>;
    const items = (root.history as GmailHistoryItem[] | undefined) ?? [];

    const orderedIds: string[] = [];
    const seen = new Set<string>();
    const provenance = new Map<string, { via_messageAdded: boolean; via_labelAdded_inbox: boolean }>();
    let malformed_labelAdded_skipped = 0;

    const observe = (message_id: string, via: 'messageAdded' | 'labelAdded_inbox'): void => {
      if (!seen.has(message_id)) {
        seen.add(message_id);
        orderedIds.push(message_id);
        provenance.set(message_id, {
          via_messageAdded: via === 'messageAdded',
          via_labelAdded_inbox: via === 'labelAdded_inbox'
        });
        return;
      }
      // Second sighting in the same cursor span — Q3.A first-seen wins:
      // do NOT re-add to orderedIds. Update provenance so the worker can
      // mark is_dedupe_drop and count messages_observed_via_both.
      const prev = provenance.get(message_id);
      if (prev) {
        provenance.set(message_id, {
          via_messageAdded: prev.via_messageAdded || via === 'messageAdded',
          via_labelAdded_inbox: prev.via_labelAdded_inbox || via === 'labelAdded_inbox'
        });
      }
    };

    for (const item of items) {
      for (const m of item.messagesAdded ?? []) {
        observe(m.message.id, 'messageAdded');
      }
      for (const la of item.labelsAdded ?? []) {
        // Q5 — malformed labelAdded: missing OR non-array labelIds.
        // Defense-in-depth: also catch the case where Gmail returns a
        // labelIds field that's present but null.
        const labels = la.labelIds;
        if (!Array.isArray(labels)) {
          malformed_labelAdded_skipped++;
          continue;
        }
        // Q2.A — INBOX literal post-filter. Non-INBOX label-additions
        // (STARRED, IMPORTANT, custom labels) are silently ignored.
        if (!labels.includes('INBOX')) {
          continue;
        }
        observe(la.message.id, 'labelAdded_inbox');
      }
    }

    const latestHistoryId = String(root.historyId ?? startHistoryId);
    return Object.freeze({
      latest_history_id: latestHistoryId,
      added_message_ids: Object.freeze(orderedIds),
      event_provenance: provenance,
      malformed_labelAdded_skipped
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
