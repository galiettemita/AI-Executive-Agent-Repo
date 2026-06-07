// Phase v0.5.13 — Synthetic Path C seed (runbook §16 sanctioned substitute).
//
// In-vivo Path C requires a real Gmail rank where: (a) the worker's
// allowlist gate lets the user through, (b) the alert sender's HMAC
// matches a canonical-HMAC PIL row in the founder substrate, and (c)
// the two-call hybrid fires, producing a brevio.rank.pil_applied audit.
//
// The founder cannot predict which sender_email maps to which substrate
// scope_key (HMAC is opaque), and natural Gmail traffic during a short
// canary window may not hit a substrate-matching sender. Per surface-
// scope rule, the synthetic-seed substitute is sanctioned to produce the
// audit row + verify the audit-write code path end-to-end against the
// live DB.
//
// What this script DOES that the v0.5.12 seed did not:
//   1. Loads the SAME KillSwitches the production worker loads.
//   2. Performs the EXACT allowlist gate check the worker performs in
//      apps/fomo/src/workers/gmail-poll.ts:677:
//        user_id ∈ killSwitches.pil_live_user_allowlist
//      with strict === (founder correction #1 — no lowercase).
//   3. ONLY if the gate allows the user, invokes buildLivePilContext +
//      rankEmailWithLivePil. The gate decision is the load-bearing v0.5.13
//      proof; the hybrid + audit chain is the v0.5.12 carry-forward.
//
// This script DOES NOT bypass the gate. If FOMO_PIL_LIVE_USER_ALLOWLIST
// does not contain the user_id, the script exits with a clear message
// and does NOT call the hybrid (matching the worker's fail-through
// behavior).

import { loadDbClient } from '../src/db/client.js';
import { PostgresAuditStore } from '../src/db/stores/audit-postgres.js';
import { PostgresMemorySignalStore } from '../src/db/stores/memory-postgres.js';
import { PostgresRankResultStore } from '../src/db/stores/rank-results-postgres.js';
import { OpenAIBackend } from '../src/core/model-backends/openai.js';
import { createModelRouter } from '../src/core/model-router.js';
import { InMemoryCostStore } from '../src/core/cost-tracking.js';
import { RANKER_OPENAI_RESPONSE_FORMAT } from '../src/ranker/openai-response-format.js';
import { buildLivePilContext } from '../src/ranker/pil-context.js';
import {
  rankEmailWithLivePil,
  writeBrevioRankPilAppliedAudit
} from '../src/ranker/index.js';
import { loadPilTunables } from '../src/memory/pil-aggregation.js';
import { loadKillSwitches } from '../src/core/kill-switches.js';
import { type RawEmailContext } from '../src/core/egress-policy.js';

// Same BB1-equivalent scope the v0.5.12 seed used: founder's
// sender_importance.score=-0.30 (n=3) + sender_suppressed=true.
const SCOPE_KEY = 'd1a6c6c65e9c5d7a363198a0af91c1b7';

async function main(): Promise<void> {
  const dbUrl = process.env.DATABASE_URL;
  if (!dbUrl) throw new Error('DATABASE_URL must be set');
  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) throw new Error('OPENAI_API_KEY must be set');
  const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
  if (!founderUserId) throw new Error('FOMO_FOUNDER_USER_ID must be set');

  // === LOAD-BEARING v0.5.13 gate check ===
  // Load the SAME KillSwitches the production wiring loads, then mirror
  // the exact worker gate logic from gmail-poll.ts:677.
  const killSwitches = loadKillSwitches(process.env);
  const pilLiveEnabled = killSwitches.pil_live_enabled;
  const allowlist = killSwitches.pil_live_user_allowlist;
  const userInAllowlist = pilLiveEnabled && allowlist.includes(founderUserId);

  process.stdout.write(
    `Gate evaluation:\n` +
      `  FOMO_PIL_LIVE_ENABLED  = ${pilLiveEnabled}\n` +
      `  allowlist              = ${JSON.stringify([...allowlist])}\n` +
      `  founder user_id        = ${founderUserId}\n` +
      `  userInAllowlist        = ${userInAllowlist}\n\n`
  );

  if (!pilLiveEnabled) {
    process.stdout.write(
      'Gate DECLINED: FOMO_PIL_LIVE_ENABLED is false. Worker would fall through to baseline-only path. No audit row would be written. (This matches case (a) of the 4-case truth table.)\n'
    );
    process.exit(0);
  }
  if (!userInAllowlist) {
    process.stdout.write(
      `Gate DECLINED: founder user_id '${founderUserId}' is NOT in pil_live_user_allowlist. Worker would fall through to baseline-only path AND tick messages_pil_skipped_not_in_allowlist. No audit row would be written. (This matches case (b) or case (c) non-match.)\n`
    );
    process.exit(0);
  }

  process.stdout.write(
    `Gate ALLOWED: founder is in allowlist + global=true. Proceeding with v0.5.12 two-call hybrid + audit-write chain.\n\n`
  );

  // === v0.5.12 carry-forward hybrid + audit chain ===
  const tunables = loadPilTunables(process.env);
  const pilScoreCap = killSwitches.pil_score_cap;
  const model = process.env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini';

  const dbHandle = loadDbClient({ env: process.env });
  if (!dbHandle.ok) {
    throw new Error(`loadDbClient returned not-ok: ${dbHandle.reason}`);
  }
  const db = dbHandle.client;
  const memoryStore = new PostgresMemorySignalStore(db);
  const auditStore = new PostgresAuditStore(db);
  const rankStore = new PostgresRankResultStore(db);

  const cost = new InMemoryCostStore();
  const backend = new OpenAIBackend({ apiKey, model, responseFormat: RANKER_OPENAI_RESPONSE_FORMAT });
  const router = createModelRouter({ costStore: cost });
  router.registerBackend('classification', backend);

  const pilContext = await buildLivePilContext(founderUserId, SCOPE_KEY, {
    memoryStore,
    recency_full_days: tunables.recency_full_days,
    recency_decay_days: tunables.recency_decay_days,
    k_threshold: tunables.k_threshold
  });
  if (!pilContext) {
    throw new Error(
      `buildLivePilContext returned null for founder scope ${SCOPE_KEY} — substrate may have changed since v0.5.12 PASS or k_threshold floor blocked it.`
    );
  }
  process.stdout.write(`PIL context loaded: ${JSON.stringify(pilContext)}\n\n`);

  const ts = Date.now();
  const messageId = `smoke-v0.5.13-path-c-${ts}`;
  const raw: RawEmailContext = Object.freeze({
    message_id: messageId,
    thread_id: `thr-${ts}`,
    sender_email: 'ceo-counsel@example.com',
    sender_name: 'CEO Counsel',
    subject: 'URGENT: term sheet signature needed by 9pm tonight',
    body_plain:
      'The board called. The Series A term sheet expires at 9pm if we do not counter-sign. Please confirm receipt and sign now.',
    body_html:
      '<p>The board called. The Series A term sheet expires at 9pm if we do not counter-sign. Please confirm receipt and sign now.</p>',
    headers: {},
    attachments: [],
    received_at: new Date()
  } as RawEmailContext);

  const r = await rankEmailWithLivePil(
    {
      raw,
      user_id: founderUserId,
      pil_context: pilContext,
      sender_email_hash: SCOPE_KEY
    },
    {
      router,
      auditStore,
      pil_live_enabled: true,
      pil_score_cap: pilScoreCap
    }
  );

  if (!r.result.ok) {
    throw new Error(`rankEmailWithLivePil failed: ${r.result.reason}`);
  }
  if (!r.audit_payload) {
    throw new Error('audit_payload is null — two-call hybrid did NOT run.');
  }

  const writeOutcome = await rankStore.write({
    user_id: founderUserId,
    message_id: raw.message_id,
    invocation_id: `smoke-v0.5.13-path-c-${ts}`,
    model_name: r.result.model_name,
    prompt_version: r.result.prompt_version,
    label: r.result.decision.label,
    score: r.result.decision.score,
    reason: r.result.decision.reason,
    latency_ms: r.result.latency_ms,
    input_tokens: r.result.input_tokens,
    output_tokens: r.result.output_tokens,
    estimated_cost_usd: r.result.estimated_cost_usd
  });

  await writeBrevioRankPilAppliedAudit(
    auditStore,
    founderUserId,
    writeOutcome.rank_result_id,
    r.audit_payload
  );

  process.stdout.write(
    `Path C synthetic-seed COMPLETE.\n` +
      `  rank_result_id:      ${writeOutcome.rank_result_id}\n` +
      `  prompt_version:      ${r.result.prompt_version}\n` +
      `  scope_key:           ${SCOPE_KEY}\n` +
      `  baseline_score:      ${r.baseline_result?.ok ? r.baseline_result.decision.score.toFixed(3) : 'n/a'}\n` +
      `  pil_score (pre-cap): ${r.pil_result?.ok ? r.pil_result.decision.score.toFixed(3) : 'n/a'}\n` +
      `  final_score:         ${r.result.decision.score.toFixed(3)}\n` +
      `  pil_score_delta:     ${r.audit_payload.pil_score_delta.toFixed(3)}\n` +
      `  was_capped:          ${r.audit_payload.pil_score_delta_was_capped}\n` +
      `  audit written:       brevio.rank.pil_applied for founder/${writeOutcome.rank_result_id}\n`
  );

  await dbHandle.pool.end();
}

main().catch((err) => {
  process.stderr.write(`[seed-path-c v0.5.13] crashed: ${String(err)}\n`);
  process.exit(1);
});
