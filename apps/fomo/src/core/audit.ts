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
  | 'fomo.onboard.kill_switch_off';

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
  'fomo.onboard.kill_switch_off'
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
