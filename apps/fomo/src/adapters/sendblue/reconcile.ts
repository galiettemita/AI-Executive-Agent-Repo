// Phase v0.5.3 item #4 — SendBlue webhook-delivery reconciliation.
//
// Pure logic. The ops:reconcile-sendblue script wires this against
// the live SendBlue API + Postgres audit_log; tests inject fakes.
//
// v0.5.2 surfaced the visibility gap: a SendBlue-confirmed inbound
// iMessage that never reached our /sendblue/inbound webhook (server
// was down + SendBlue retries exhausted) was undetectable without
// manually querying SendBlue's /api/v2/messages 11h later. This
// helper closes that gap on-demand. Per founder correction #4: NOT
// wired as a periodic worker in v0.5.3 (that's a future phase).

export interface SendBlueMessage {
  readonly message_handle: string;
  readonly is_outbound: boolean;
  readonly status: string;
  readonly service: string;
  readonly from_number: string;
  readonly to_number: string;
  readonly date_sent: string;
  // We intentionally do NOT declare `content` here — the reconciler
  // never reads the message body. The audit row for gaps surfaces
  // only safe identifiers (message_handle + from_slug + date + status).
}

export interface ReconcileDeps {
  readonly fetchImpl?: typeof fetch;
  readonly apiKeyId: string;
  readonly apiSecretKey: string;
  readonly windowHours: number;
  readonly now?: () => number;
  // Returns the set of message_handle values we've already audited
  // (any fomo.sendblue.* action whose detail carries provider_message_id)
  // for messages within the window.
  readonly fetchAuditedHandles: (sinceMs: number) => Promise<Set<string>>;
  // Records a fomo.sendblue.delivery_gap_detected audit row.
  readonly recordGap: (msg: SendBlueMessage) => Promise<void>;
  // Optional override for the SendBlue messages endpoint. Defaults
  // to api.sendblue.co.
  readonly messagesUrlBase?: string;
}

export interface ReconcileResult {
  readonly sendblue_inbound_count: number;
  readonly audit_handles_in_window: number;
  readonly gaps_found: number;
  readonly gap_handles: readonly string[];
}

interface SendBlueListResponse {
  readonly status: string;
  readonly data: SendBlueMessage[];
  readonly pagination?: { total: number; limit: number; offset: number; hasMore: boolean };
}

export async function reconcileSendBlue(deps: ReconcileDeps): Promise<ReconcileResult> {
  const fetchImpl = deps.fetchImpl ?? fetch;
  const now = deps.now ?? Date.now;
  const sinceMs = now() - deps.windowHours * 3600 * 1000;
  const urlBase = deps.messagesUrlBase ?? 'https://api.sendblue.co/api/v2/messages';

  // 1. Fetch inbound messages, paginated. Stop when we cross the
  //    window boundary (date_sent < since) OR SendBlue says
  //    hasMore=false.
  const inbounds: SendBlueMessage[] = [];
  let offset = 0;
  const limit = 50;
  // Safety bound to prevent runaway pagination if SendBlue's
  // pagination field is missing/wrong.
  const maxPages = 50;
  let pagesFetched = 0;
  while (pagesFetched < maxPages) {
    const url = `${urlBase}?limit=${limit}&offset=${offset}`;
    const response = await fetchImpl(url, {
      method: 'GET',
      headers: {
        'sb-api-key-id': deps.apiKeyId,
        'sb-api-secret-key': deps.apiSecretKey
      }
    });
    if (!response.ok) {
      throw new Error(`SendBlue ${urlBase} returned HTTP ${response.status}`);
    }
    const body = (await response.json()) as SendBlueListResponse;
    pagesFetched++;
    if (!Array.isArray(body.data) || body.data.length === 0) break;
    let crossedBoundary = false;
    for (const msg of body.data) {
      const sentMs = Date.parse(msg.date_sent);
      if (!Number.isFinite(sentMs) || sentMs < sinceMs) {
        crossedBoundary = true;
        continue;
      }
      if (msg.is_outbound === false) {
        inbounds.push(msg);
      }
    }
    if (crossedBoundary || !body.pagination?.hasMore) break;
    offset += limit;
  }

  // 2. Fetch the set of handles we've already audited in the window.
  const auditHandles = await deps.fetchAuditedHandles(sinceMs);

  // 3. Diff. Inbound on SendBlue side but no audit row = delivery gap.
  const gaps: SendBlueMessage[] = inbounds.filter(
    (m) => !auditHandles.has(m.message_handle)
  );

  // 4. Audit each gap. recordGap is best-effort per-row; we don't
  //    short-circuit on per-row failures because the caller may want
  //    to see all gap handles in the result even if some audit writes
  //    fail.
  for (const gap of gaps) {
    try {
      await deps.recordGap(gap);
    } catch {
      // intentional: ops-side audit write failure must NOT abort
      // the reconciliation. The handle is still in result.gap_handles.
    }
  }

  return Object.freeze({
    sendblue_inbound_count: inbounds.length,
    audit_handles_in_window: auditHandles.size,
    gaps_found: gaps.length,
    gap_handles: Object.freeze(gaps.map((g) => g.message_handle))
  });
}

// Helper for the audit detail: last-4 of the from-number, NEVER full E.164.
export function phoneSlugFromMessage(msg: SendBlueMessage): string {
  const digits = (msg.from_number ?? '').replace(/\D/g, '');
  return digits.slice(-4);
}
