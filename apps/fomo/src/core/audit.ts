// Audit log helper. Writes to public.audit_log via injected store; in dev mode uses
// in-memory ring buffer so the simulator can read its own audit history without Postgres.

import { redact } from './safe-logger.js';

export type AuditAction =
  // Lifecycle (Phase 2A)
  | 'consent.grant'
  | 'consent.revoke'
  | 'consent.snooze'
  | 'oauth.connect'
  | 'oauth.disconnect'
  | 'oauth.refresh'
  | 'oauth.revoke_failed'
  | 'token.decrypt_failure'
  | 'session.created'
  | 'onboarding.dismissed'
  // Kernel-touch events (Phase 2F.1) — written by the integrated kernel
  // path so the audit log participates in every meaningful substrate
  // operation, not just lifecycle events. Callers MUST pass sanitized
  // detail only: no raw email body, no headers, no attachment filenames,
  // no prompt text, no full reply text. Operational identifiers
  // (tool_id, model_name, prompt_version, alert_id, from/to_state) are
  // OK; user-payload content is not.
  | 'policy.decided'
  | 'tool.invoked'
  | 'state.transitioned'
  | 'feedback.written'
  | 'memory.upserted'
  | 'model.routed'
  // Workflow events (Phase 3B.2) — one entry per polling worker cycle.
  // Per-message reads continue to surface as policy.decided + tool.invoked
  // for tool_id='gmail.read'; this aggregate cycle entry exists so ops
  // can answer "is polling alive?" without correlating dispatch events.
  | 'gmail.poll.cycle'
  // Ranker events (Phase 3C.3) — one entry per dispatched gmail.read
  // result when the polling worker has a ranker wired. Sanitized detail
  // only: model_name + prompt_version + label + score + token counts +
  // latency + cost. NEVER body content; the ranker's input is already
  // egress-redacted but this audit row does not include input either.
  // already_ranked surfaces idempotency hits (rank_results unique
  // constraint matched); failed surfaces ranker timeouts/schema errors.
  | 'fomo.rank.completed'
  | 'fomo.rank.already_ranked'
  | 'fomo.rank.failed'
  // Slack candidate review posting events (Phase 3D.1) — fire when a
  // RankerSuccess with label='important' lands AND FOMO_SLACK_REVIEW_ENABLED
  // is on. Sanitized detail only: alert_id, rank_result_id, message_id,
  // label, score, model_name, slack_ts on success. NEVER body content
  // (the Slack card payload itself is egress-redacted via
  // applyEgressForSlackCard and NOT included in audit detail).
  // `fomo.slack.already_alerted` fires when the alerts UNIQUE constraint
  // on rank_result_id hits — protects the founder channel from re-spam
  // on cursor rewinds / restarts. `fomo.slack.failed` surfaces Slack
  // auth or API errors (channel_not_found, rate_limited, 5xx); cycle
  // continues, alert state transitions to `failed`.
  | 'alert.created'
  | 'fomo.slack.posted'
  | 'fomo.slack.already_alerted'
  | 'fomo.slack.failed'
  // Slack approval-capture inbound events (Phase 3D.2) — fire on the
  // /slack/interactivity HTTP route. Sanitized detail only: route
  // identifiers (action_id, alert_id, decision, target_state) and a
  // user_slug / channel_slug suffix for traceability. NEVER the raw
  // Slack payload, NEVER the full Slack user_id (only the suffix), and
  // NEVER message text. The 'interaction_received' row fires BEFORE
  // signature verification so a flood of unsigned requests is still
  // visible in the audit log.
  | 'fomo.slack.interaction_received'
  | 'fomo.slack.signature_invalid'
  | 'fomo.slack.payload_invalid'
  | 'fomo.slack.approval_unauthorized'
  | 'fomo.slack.approval_captured'
  | 'fomo.slack.approval_duplicate'
  // SendBlue outbound send events (Phase 3E.1) — fire from the outbound
  // sender worker when an alert in state 'approved' is picked up and
  // dispatched through sendblue.send_user_message. Sanitized detail
  // only: alert_id, message_id, rank_result_id, destination_slug
  // (last 4 chars of the phone number), template_version, provider_status,
  // provider_message_handle on success. NEVER the rendered message text,
  // NEVER the full phone number, NEVER the SendBlue API key.
  //
  // Three-outcome semantics per founder directive 2026-05-25:
  //   - `fomo.send.succeeded`    → provider returned a clear success
  //     code; alert transitions approved → sent
  //   - `fomo.send.failed`       → provider returned a clear failure
  //     (4xx other than auth; explicit error code); alert transitions
  //     approved → failed. Auth errors also surface here.
  //   - `fomo.send.status_unknown` → ambiguous outcome (network
  //     timeout, 5xx, malformed response, or non-terminal provider
  //     status). Alert transitions approved → send_status_unknown.
  //     The worker DOES NOT auto-retry this state — duplicate sends
  //     could deliver real texts twice. Operator must inspect.
  //
  // Defense-in-depth at the worker boundary (in addition to gate):
  //   - `fomo.send.unauthorized_destination` fires when the resolved
  //     destination phone number does not match FOMO_FOUNDER_PHONE_NUMBER.
  //     No SendBlue call is made; alert transitions approved → failed.
  //   - `fomo.send.kill_switch_off` fires when the worker is asked to
  //     send while FOMO_SEND_ENABLED=false. No SendBlue call; no state
  //     transition. Visible in audit so operators can see attempted
  //     activity even with the switch off.
  //   - `fomo.send.attempted` fires BEFORE the SendBlue API call so a
  //     flood of failed attempts is still visible in the audit log
  //     even if every subsequent call errors before writing its result.
  | 'fomo.send.attempted'
  | 'fomo.send.succeeded'
  | 'fomo.send.failed'
  | 'fomo.send.status_unknown'
  | 'fomo.send.unauthorized_destination'
  | 'fomo.send.kill_switch_off'
  // SendBlue outbound STOP-enforcement event (Phase 3F.1) — fires
  // when the outbound-sender picks up an approved alert but the
  // user has an active `stop_active` memory signal. The send is
  // refused, the alert transitions `approved → failed` with reason
  // 'stop_enforced', and NO SendBlue API call is made. Per the
  // founder directive 2026-05-26, STOP enforcement is deterministic
  // (no LLM decides whether STOP means stop). Sanitized detail only:
  // alert_id, message_id, rank_result_id, destination_slug. NEVER
  // the rendered message text, NEVER the full destination phone.
  | 'fomo.send.stop_enforced'
  // SendBlue OPTED_OUT drift detection (Phase 3G.1 item #2) — fires
  // when SendBlue's send API returns a 4xx whose decoded
  // providerError.error_message === 'OPTED_OUT'. SendBlue's
  // carrier-level opt-out list and our local stop_active memory
  // signal can drift: a user texts STOP, we record stop_active=true
  // and SendBlue records carrier opt-out, then a future SQL clear
  // (or any memory_signals tampering) flips our local back to
  // active=false while SendBlue's spam-rule firewall still blocks
  // outbound. The runtime catches OPTED_OUT, re-writes
  // stop_active=true (source='opt_out_drift_carrier'), and emits
  // this audit so the operator knows local cache was wrong.
  // Detail surfaces alert_id + message_id + rank_result_id +
  // destination_slug + provider error_message/error_reason/error_code.
  // NEVER the raw response body, NEVER the rendered message text,
  // NEVER the full destination phone. Real incident: 2026-05-29 01:12 UTC.
  | 'fomo.send.opt_out_drift_detected'
  // SendBlue inbound reply events (Phase 3F.1) — fire on the
  // /sendblue/inbound HTTP route. Sanitized detail ONLY: route
  // identifiers (provider_message_id, intent, intent_source,
  // confidence), alert_id when matched, a from_slug suffix for
  // traceability, snooze_until ISO when relevant. NEVER the raw
  // webhook payload, NEVER the founder's reply text, NEVER the full
  // from-phone (only the 4-char slug suffix), NEVER the SendBlue
  // signing secret.
  //
  // The 'inbound_received' row fires BEFORE signature verification
  // so a flood of unsigned requests is still visible in the audit
  // log. The 'reply_duplicate' row fires when the inbound_replies
  // UNIQUE constraint catches a SendBlue retry (idempotency proof).
  // 'stop_recorded' and 'start_recorded' are the deterministic
  // safety/compliance commands (NOT classifier output).
  | 'fomo.sendblue.inbound_received'
  | 'fomo.sendblue.signature_invalid'
  | 'fomo.sendblue.payload_invalid'
  | 'fomo.sendblue.reply_unauthorized'
  | 'fomo.sendblue.reply_duplicate'
  | 'fomo.sendblue.reply_parsed'
  | 'fomo.sendblue.reply_unclear'
  | 'fomo.sendblue.stop_recorded'
  | 'fomo.sendblue.start_recorded'
  | 'fomo.sendblue.kill_switch_off'
  // Friend-beta onboarding events (Phase v0.5.1 step #3+#4). Fire
  // when the founder issues an invite token (issue-friend-token
  // script) and when a friend lands at /onboard.
  // Sanitized detail only: invite_id, token_hash_prefix (8 chars),
  // intended_phone_slug (last 4), expires_at_iso, consumed_user_id.
  // NEVER the plaintext token, NEVER the raw phone, NEVER the full
  // intended_phone_hash. See [[multitenant-design-principles]] §5b/§5d.
  | 'fomo.onboard.invite_issued'
  | 'fomo.onboard.invite_invalid'
  | 'fomo.onboard.phone_mismatch'
  | 'fomo.onboard.user_created'
  | 'fomo.onboard.kill_switch_off'
  // Phase v0.5.3 production-hardening events. Fire when the substrate
  // self-heals from real-world conditions v0.5.2 surfaced. Sanitized
  // detail only (operator identifiers + safe slugs; NEVER raw phones,
  // refresh tokens, SendBlue payloads, or DB connection strings).
  //
  // SendBlue contact lifecycle (v0.5.3 item #1):
  //   - contact_registered     → /onboard/callback POSTed /api/v2/contacts
  //     successfully; memory_signals.sendblue_contact_status set to
  //     {registered: true}. Friend's outbound path is unblocked.
  //   - contact_registration_failed → POST /api/v2/contacts returned 4xx/5xx
  //     OR send not enabled. users row + oauth_tokens are NOT rolled back
  //     (friend's OAuth grant is valuable); memory_signals records
  //     {registered: false, error_reason}. Outbound is gated by the
  //     next event below until founder retries.
  //   - send.contact_not_registered → outbound worker refused to send
  //     because memory_signals.sendblue_contact_status is false. Alert
  //     transitions approved → failed with reason 'contact_not_registered';
  //     NO SendBlue API call is made.
  | 'fomo.sendblue.contact_registered'
  | 'fomo.sendblue.contact_registration_failed'
  | 'fomo.send.contact_not_registered'
  // OAuth auto-refresh (v0.5.3 item #2):
  //   - oauth.refreshed        → polling worker detected near-expired
  //     access_token, called refreshAccessToken, saved the new token,
  //     advanced last_refreshed_at. Detail: provider, expires_at_iso.
  //   - oauth.refresh_failed   → Google returned 4xx (typically
  //     invalid_grant). refresh_token is revoked/invalid; worker sets
  //     needs_reauth=true, skips the user this cycle. Detail: provider,
  //     reason ('invalid_grant'|'network'|'unknown'). NEVER the
  //     refresh_token plaintext, NEVER the access_token plaintext.
  | 'fomo.oauth.refreshed'
  | 'fomo.oauth.refresh_failed'
  // DB pool resilience (v0.5.3 item #3):
  //   - db.connection_error    → pg pool emitted an 'error' event from
  //     a transient connection drop (ECONNRESET, server-side timeout,
  //     etc.). The handler logs (primary, always works) + audits
  //     best-effort (DB may be down; the audit write is wrapped in
  //     try/catch and does NOT re-throw). Process does NOT exit.
  //     Detail: error_code, sanitized message. NEVER the connection
  //     string, NEVER credentials.
  | 'fomo.db.connection_error'
  // SendBlue webhook-delivery reconciliation (v0.5.3 item #4):
  //   - sendblue.delivery_gap_detected → ops:reconcile-sendblue
  //     compared SendBlue's /api/v2/messages against our audit_log
  //     (joined by message_handle = provider_message_id) and found
  //     an inbound SendBlue claims to have received that has NO
  //     fomo.sendblue.inbound_received row on our side. Most common
  //     cause: our server was down during webhook delivery + SendBlue's
  //     retries exhausted. Detail: provider_message_id, from_slug,
  //     date_sent_iso. NEVER the message content, NEVER the raw phone.
  | 'fomo.sendblue.delivery_gap_detected'
  // Phase v0.5.5 — STOP Enforcement + Confirmation. Four new audit
  // kinds fire when (a) the polling worker skips ranker dispatch
  // because the user has stop_active=true (`fomo.poll.skipped_stop_active`),
  // (b) the alert-creation pipeline short-circuits at the
  // postSlackCandidateReview defense-in-depth check
  // (`fomo.alert.suppressed_stop_active`), (c) the SendBlue inbound
  // route's `applyStop` sends the deterministic STOP confirmation
  // courtesy reply (`fomo.sendblue.stop_confirmation_sent`), or
  // (d) that confirmation send fails (`fomo.sendblue.stop_confirmation_failed`).
  //
  // Q5 (founder-locked): 24h idempotency window for confirmations.
  // Duplicate STOP within 24h → no new confirmation send → no
  // `stop_confirmation_sent` row.
  //
  // Q6 (founder-locked): best-effort audit, no retry. Confirmation
  // send failure writes `fomo.sendblue.stop_confirmation_failed` and
  // stops. STOP enforcement itself is load-bearing; the confirmation
  // is a courtesy/trust message. Detail surfaces sanitized
  // error_code + error_message (≤200 chars) + target_user_id. NEVER
  // the SendBlue API key, NEVER the rendered confirmation body
  // beyond the canonical `message_preview` field.
  //
  // The STOP confirmation is the ONLY allowed outbound exception
  // after STOP — the normal FOMO alert pipeline (outbound-sender.ts)
  // continues to be blocked by `fomo.send.stop_enforced`. This
  // exception path uses a separate SendBlue client call with its
  // own 24h idempotency guard, and writes to the same `stop_active`
  // memory signal's detail JSONB (key: `stop_confirmation_sent_at`,
  // ISO 8601). No new memory_signal kind.
  | 'fomo.poll.skipped_stop_active'
  | 'fomo.alert.suppressed_stop_active'
  | 'fomo.sendblue.stop_confirmation_sent'
  | 'fomo.sendblue.stop_confirmation_failed'
  // Phase v0.5.6 — iMessage Tone + Summary Length. Fires when the
  // outbound-sender's body-render step (renderFounderText) detects
  // that the stored rank_result.reason fails the v0.5.6 body schema
  // (empty/whitespace OR length > REASON_HARD_CAP_FOR_RENDER) and
  // substitutes the deterministic fallback string. Defense-in-depth
  // at the body-render layer for any rank_result that bypassed the
  // ranker-level validator (e.g. historical rows from a prior phase,
  // or a strict-mode escape). Q6 founder-locked: best-effort audit,
  // NO retry — the alert continues to send with the fallback body.
  // 3E.1 directive (2026-05-25) PRESERVED: the fallback is
  // deterministic, never LLM-generated. Detail: alert_id, message_id,
  // rank_result_id, reason_violation_kind ('empty' | 'too_long'),
  // original_reason_length. NEVER the original reason text body,
  // NEVER any email content.
  | 'fomo.alert.drafter_schema_failed'
  // Phase v0.5.7 — Human Message Renderer. Fires when the
  // outbound-sender's body-render step (renderHumanMessage) applies
  // ANY of the Q5.A degradation fallback rules:
  //   * sender_resolution_path='generic' (fell through Q2.B chain to "Someone")
  //   * subject_strip_applied='subject_empty' (subject was empty, "about X" clause dropped)
  //   * reason_voice='fallback' (rank.reason failed schema; deterministic fallback substituted)
  //   * reason_voice='legacy_3p' (ranker-v0.1.0 still in use; transitional)
  // Best-effort audit, NO retry. The alert continues to send with the
  // degraded body. 3E.1 PRESERVED: renderHumanMessage remains
  // deterministic; only rank.reason is model-generated. Detail surfaces
  // STRUCTURAL fields only (which fallback fired, which audit-field
  // enum value), NEVER raw subject/body/header/attachment content.
  // Companion to v0.5.6's `fomo.alert.drafter_schema_failed` (which
  // fires only on reason_voice='fallback'); this v0.5.7 audit covers
  // the OTHER three Q5.A degradation paths too.
  | 'fomo.alert.hmr_degradation_applied'
  // Phase v0.5.8 — Gmail INBOX Event Reliability Hardening. Fires once
  // per (cycle, message_id) AFTER the per-cycle Q3.A Set<message_id>
  // dedupe, regardless of which Gmail history event type surfaced the
  // message first. The Q1.A filter swap (historyTypes='messageAdded,
  // labelAdded') means a single new INBOX message can surface via:
  //   - messageAdded only (typical external mail, fresh delivery)
  //   - labelAdded:INBOX only (typical Gmail-to-self self-sends; v0.5.7
  //     baseline = NEVER surfaced; the KEY METRIC the hardening targets)
  //   - both (Gmail history batched both events into the same cursor span)
  // Detail is STRUCTURAL ONLY per Q6.A founder lock (sanitized + minimal):
  //   - event_types_seen: ('messageAdded'|'labelAdded')[]
  //   - inbox_label_present: boolean (true if labelAdded included the
  //     literal 'INBOX' system label; the Q2.A INBOX-literal filter
  //     guarantees this is true whenever 'labelAdded' is in event_types_seen)
  //   - is_dedupe_drop: boolean (true if both event types fired for the
  //     same message_id in the same cycle and the second sighting was
  //     dropped — equals via_both per Q3.A first-seen-wins)
  //   - message_id: the Gmail message id (opaque)
  // NEVER subject, sender, body, raw label names beyond the boolean
  // derivative inbox_label_present, raw event JSON, or attachment names.
  // Companion: cycle-level aggregate counters on gmail.poll.cycle detail
  // (messages_observed_via_messageAdded_only, *_labelAdded_only, *_both,
  // messages_dedupe_drops) carry the rollup view without per-message scan
  // cost. messages_observed continues to count post-dedupe UNIQUE messages.
  | 'fomo.gmail.poll.event_observed'
  // Phase v0.5.8 — Q5 fallback for malformed Gmail history events.
  // Fires once per malformed labelAdded event observed in a poll cycle.
  // Gmail rarely returns a labelAdded event without an addedLabels field
  // (or with a non-array addedLabels), but defense-in-depth: the parser
  // silently skips the event AND emits this audit so the operator can
  // notice if Gmail's API contract drifts. Best-effort, NO retry — the
  // cycle continues; the cursor still advances on the next happy event.
  // Detail surfaces sanitized fields only: reason ('malformed_labelAdded').
  // NEVER the raw event JSON, NEVER a message_id (the malformed event
  // may not even carry one).
  | 'fomo.gmail.poll.event_skipped'
  // Phase v0.5.9 — Feedback + Learn/Grow Loop substrate (Brevio-wide).
  // Fires once per memory_signal upsert performed by the applyFeedback
  // consumer. The v0.5.9 hardcoded match arm is:
  //   (source_surface='email_alert', mapped_verb='ignored', detail.dimension='sender')
  //   → upsert memory_signals(kind='sender_feedback_ignored', scope_key=<HMAC-hashed>)
  // Detail is STRUCTURAL ONLY per Q6.C + founder approval-time privacy
  // guardrail (NO raw sender email):
  //   - feedback_event_id: bigint
  //   - source_surface: BrevioFeedbackSurface
  //   - verb: BrevioFeedbackEventKind (the generic/mapped verb)
  //   - dimension: string | undefined (e.g. 'sender', 'alert')
  //   - memory_signal_kind: MemorySignalKind (e.g. 'sender_feedback_ignored')
  //   - memory_signal_action: 'created' | 'updated'
  //   - memory_signal_scope_key_hash: string (the same HMAC hash that's
  //       the scope_key — exposes the lookup key for cross-row joins
  //       without leaking the raw sender)
  //   - confidence: number
  // NEVER subject, sender_email, body, snippet, raw headers, attachment
  // names, access tokens.
  | 'brevio.feedback.applied';

// Phase 3G.1 — runtime registry of every FOMO-namespaced audit
// action. Used by the 3G.1 evidence script (and any future ops
// tooling) to assert the runtime has registered a new action
// without booting the server. The `satisfies` clause ensures every
// entry is a real member of the AuditAction union at compile time.
export const FOMO_AUDIT_ACTIONS = [
  'gmail.poll.cycle',
  'fomo.rank.completed',
  'fomo.rank.already_ranked',
  'fomo.rank.failed',
  'alert.created',
  'fomo.slack.posted',
  'fomo.slack.already_alerted',
  'fomo.slack.failed',
  'fomo.slack.interaction_received',
  'fomo.slack.signature_invalid',
  'fomo.slack.payload_invalid',
  'fomo.slack.approval_unauthorized',
  'fomo.slack.approval_captured',
  'fomo.slack.approval_duplicate',
  'fomo.send.attempted',
  'fomo.send.succeeded',
  'fomo.send.failed',
  'fomo.send.status_unknown',
  'fomo.send.unauthorized_destination',
  'fomo.send.kill_switch_off',
  'fomo.send.stop_enforced',
  'fomo.send.opt_out_drift_detected',
  'fomo.sendblue.inbound_received',
  'fomo.sendblue.signature_invalid',
  'fomo.sendblue.payload_invalid',
  'fomo.sendblue.reply_unauthorized',
  'fomo.sendblue.reply_duplicate',
  'fomo.sendblue.reply_parsed',
  'fomo.sendblue.reply_unclear',
  'fomo.sendblue.stop_recorded',
  'fomo.sendblue.start_recorded',
  'fomo.sendblue.kill_switch_off',
  'fomo.onboard.invite_issued',
  'fomo.onboard.invite_invalid',
  'fomo.onboard.phone_mismatch',
  'fomo.onboard.user_created',
  'fomo.onboard.kill_switch_off',
  'fomo.sendblue.contact_registered',
  'fomo.sendblue.contact_registration_failed',
  'fomo.send.contact_not_registered',
  'fomo.oauth.refreshed',
  'fomo.oauth.refresh_failed',
  'fomo.db.connection_error',
  'fomo.sendblue.delivery_gap_detected',
  // Phase v0.5.5 — STOP Enforcement + Confirmation.
  'fomo.poll.skipped_stop_active',
  'fomo.alert.suppressed_stop_active',
  'fomo.sendblue.stop_confirmation_sent',
  'fomo.sendblue.stop_confirmation_failed',
  // Phase v0.5.6 — iMessage Tone + Summary Length.
  'fomo.alert.drafter_schema_failed',
  // Phase v0.5.7 — Human Message Renderer.
  'fomo.alert.hmr_degradation_applied',
  // Phase v0.5.8 — Gmail INBOX Event Reliability Hardening.
  'fomo.gmail.poll.event_observed',
  'fomo.gmail.poll.event_skipped',
  // Phase v0.5.9 — Feedback + Learn/Grow Loop substrate (Brevio-wide).
  // Closes the historical gap on 'feedback.written' (was in the AuditAction
  // union but missing from this runtime array; the v0.5.9 evidence script
  // now iterates the registry). 'brevio.feedback.applied' is the new
  // consumer-side audit emitted by applyFeedback on memory_signal upsert.
  'feedback.written',
  'brevio.feedback.applied'
] as const satisfies readonly AuditAction[];

export type AuditResult = 'success' | 'failure';

export interface AuditEntry {
  id?: number;
  occurred_at: string;
  actor_user_id: string | null;
  actor_ip: string | null;
  actor_user_agent: string | null;
  action: AuditAction;
  target: string | null;
  result: AuditResult;
  detail: Record<string, unknown> | null;
}

export interface AuditStore {
  write(entry: Omit<AuditEntry, 'id' | 'occurred_at'> & { occurred_at?: string }): Promise<void>;
  recent(userId: string, limit?: number): Promise<AuditEntry[]>;
}

export class InMemoryAuditStore implements AuditStore {
  private entries: AuditEntry[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 5000) {
    this.capacity = capacity;
  }

  async write(entry: Omit<AuditEntry, 'id' | 'occurred_at'> & { occurred_at?: string }): Promise<void> {
    const detail = entry.detail ? (redact(entry.detail) as Record<string, unknown>) : null;
    this.entries.push({
      id: this.nextId++,
      occurred_at: entry.occurred_at ?? new Date().toISOString(),
      actor_user_id: entry.actor_user_id,
      actor_ip: entry.actor_ip,
      actor_user_agent: entry.actor_user_agent,
      action: entry.action,
      target: entry.target,
      result: entry.result,
      detail
    });
    if (this.entries.length > this.capacity) {
      this.entries.splice(0, this.entries.length - this.capacity);
    }
  }

  async recent(userId: string, limit = 100): Promise<AuditEntry[]> {
    const filtered = this.entries.filter((e) => e.actor_user_id === userId);
    return filtered.slice(-limit).reverse();
  }
}
