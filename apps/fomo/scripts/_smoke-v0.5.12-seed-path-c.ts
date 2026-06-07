// Phase v0.5.12 — synthetic Path C seed (runbook §7.2 sanctioned substitute).
//
// In-vivo Path C requires an email from a sender whose HMAC matches a
// canonical PIL row in the founder's substrate. The founder does not know
// which sender_email produces each scope_key (the HMAC includes BREVIO_SENDER_HASH_KEY
// and is opaque). Per runbook §7.2, the substitute is to seed an alert
// with sender_email_hash = chosen canonical-HMAC scope_key and run the
// rank/audit pipeline directly.
//
// This script invokes the SAME rankEmailWithLivePil + writeBrevioRankPilAppliedAudit
// code path that the production polling worker calls. It writes through
// PostgresRankResultStore + PostgresAuditStore so the resulting row(s) land
// in the live Neon DB and are observable by smoke-evidence:v0.5.12.
//
// Scope choice: `d1a6c6c65e9c5d7a363198a0af91c1b7`
//   - sender_importance: score=-0.30, n_negative_events=3 (>= k_threshold=3 → passes new floor)
//   - sender_suppressed: true (BB1-equivalent state)
//
// Synthetic email: subject signals a high-importance override scenario (term
// sheet signature deadline). With the suppressed + negative prior, BB1's
// contract says the model should label `important` and acknowledge both
// the prior and the override in `reason` — proving the suppression-bypass
// works at the live ranker level on a real OpenAI call.

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
import { type RawEmailContext } from '../src/core/egress-policy.js';

const SCOPE_KEY = 'd1a6c6c65e9c5d7a363198a0af91c1b7';
const USER_ID = 'founder';

async function main(): Promise<void> {
  const dbUrl = process.env.DATABASE_URL;
  if (!dbUrl) throw new Error('DATABASE_URL must be set');
  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) throw new Error('OPENAI_API_KEY must be set');

  const tunables = loadPilTunables(process.env);
  const pilScoreCap = Number(process.env.FOMO_PIL_SCORE_CAP ?? '0.15');
  if (!Number.isFinite(pilScoreCap) || pilScoreCap <= 0) {
    throw new Error(`Invalid FOMO_PIL_SCORE_CAP=${process.env.FOMO_PIL_SCORE_CAP}`);
  }
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

  // Build PIL context via the SAME path as production (k_threshold floor applied).
  const pilContext = await buildLivePilContext(USER_ID, SCOPE_KEY, {
    memoryStore,
    recency_full_days: tunables.recency_full_days,
    recency_decay_days: tunables.recency_decay_days,
    k_threshold: tunables.k_threshold
  });
  if (!pilContext) {
    throw new Error(
      `buildLivePilContext returned null for scope ${SCOPE_KEY} — substrate may be missing or k_threshold floor blocked it`
    );
  }
  process.stdout.write(`PIL context loaded: ${JSON.stringify(pilContext)}\n`);

  // Synthetic raw email — BB1-equivalent high-importance override scenario.
  const ts = Date.now();
  const messageId = `smoke-v0.5.12-path-c-${ts}`;
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

  // Call the SAME two-call hybrid the production worker calls.
  const r = await rankEmailWithLivePil(
    {
      raw,
      user_id: USER_ID,
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
  process.stdout.write(`rank result: ok label=${r.result.decision.label} score=${r.result.decision.score.toFixed(3)} prompt_version=${r.result.prompt_version}\n`);
  if (!r.audit_payload) {
    throw new Error(
      'audit_payload is null — two-call hybrid did NOT run. Either pil_context was null after filter, or one of the two ranker calls failed.'
    );
  }
  process.stdout.write(`audit payload: ${JSON.stringify(r.audit_payload)}\n`);

  // Persist rank_results.
  const writeOutcome = await rankStore.write({
    user_id: USER_ID,
    message_id: raw.message_id,
    invocation_id: `smoke-v0.5.12-path-c-${ts}`,
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
  process.stdout.write(
    `rank_results write: inserted=${writeOutcome.inserted} rank_result_id=${writeOutcome.rank_result_id}\n`
  );

  // Write the brevio.rank.pil_applied audit AFTER rank_result_id is known.
  await writeBrevioRankPilAppliedAudit(
    auditStore,
    USER_ID,
    writeOutcome.rank_result_id,
    r.audit_payload
  );
  process.stdout.write(
    `brevio.rank.pil_applied audit written for rank_result_id=${writeOutcome.rank_result_id}\n`
  );

  process.stdout.write('\nPath C synthetic-seed COMPLETE.\n');
  process.stdout.write(`  rank_result_id: ${writeOutcome.rank_result_id}\n`);
  process.stdout.write(`  scope_key: ${SCOPE_KEY}\n`);
  process.stdout.write(`  baseline_score: ${r.baseline_result?.ok ? r.baseline_result.decision.score.toFixed(3) : 'n/a'}\n`);
  process.stdout.write(`  pil_score (pre-cap): ${r.pil_result?.ok ? r.pil_result.decision.score.toFixed(3) : 'n/a'}\n`);
  process.stdout.write(`  final_score (post-cap): ${r.result.decision.score.toFixed(3)}\n`);
  process.stdout.write(`  pil_score_delta: ${r.audit_payload.pil_score_delta.toFixed(3)}\n`);
  process.stdout.write(`  was_capped: ${r.audit_payload.pil_score_delta_was_capped}\n`);
  process.stdout.write(`  model_mentioned_pil_in_reason: ${r.audit_payload.model_mentioned_pil_in_reason}\n`);
  process.stdout.write(`  rank.reason: "${r.result.decision.reason}"\n`);

  await dbHandle.pool.end();
}

main().catch((err) => {
  process.stderr.write(`[seed-path-c] crashed: ${String(err)}\n`);
  process.exit(1);
});
