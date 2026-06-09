// Phase v0.6.0E.1 — real-model dry-run on the v0.6.0D fixtures.
//
// PURPOSE
//   Produce ACTUAL production ranker/model output for each of the 10
//   v0.6.0D fixtures (baseline + calendar-aware = 20 model calls). Founder
//   reviews the real model output — not the hardcoded expected text the
//   v0.6.0D harness emitted.
//
//   This script's output is the LOAD-BEARING evidence for the v0.6.0E.2
//   gate: only if real-model output passes founder taste check on
//   F1/F2/F4/F5/F7/F8/F10 does v0.6.0E.2 (live wiring) unlock.
//
// HARD INVARIANTS (founder-locked v0.6.0E.1 scope)
//   - Refuses to run unless BREVIO_REAL_MODEL_DRYRUN_AUTHORIZED=true.
//   - Refuses to run unless OPENAI_API_KEY is set.
//   - Calls the OpenAI ranker AT MOST 20 times (10 baseline + 10
//     calendar-aware). Hard count cap; script aborts if a code change
//     makes it exceed.
//   - Uses an InMemoryCostStore — NO writes to the production
//     cost_records DB table from this script.
//   - Uses synthetic fixtures only; no real Gmail, no real Calendar API
//     call, no real user data.
//   - Does NOT change any production runtime path. Does NOT register a
//     backend on the production model router. Does NOT modify any
//     rank call site.
//   - Does NOT flip any env flag.
//   - Does NOT touch the audit log, the rank_results table, or any
//     other persistent store.
//
// WHAT GETS SENT TO OPENAI
//   - The assembled ranker prompt (system preamble + voice rules +
//     output schema + 5 v0.5.7 examples + optional Calendar block +
//     synthetic email view).
//   - Synthetic sender/subject/body snippets only. The sender_email
//     in every fixture is already masked (e.g. m***@acme.com); the
//     subject + body snippet are synthetic; no real PII.
//   - The Calendar block contains only the three projected fields
//     (summary, start, end) from the v0.6.0D synthetic raw events.
//
// WHAT DOES NOT GET SENT
//   - No real Gmail content. No real Calendar event. No real user_id
//     (synthetic 'user-A-founder' / 'user-B-friend' only).
//   - No production rank_results, audit, or memory_signal data.
//
// COST DISCLOSURE
//   - Each model call writes ONE row to the in-memory cost store with
//     {capability, prompt_version, model_name, user_id (synthetic),
//     input_tokens, output_tokens, estimated_cost_usd}. NO prompt text,
//     NO response text in the cost row.
//   - Cost rows are NEVER written to the production cost_records table
//     by this script — it constructs its own InMemoryCostStore.
//   - Script prints a cost summary at the end (total spend +
//     per-call avg).

// Operator: source apps/fomo/.env.3b3.local before running so OPENAI_API_KEY
// + FOMO_OPENAI_MODEL are in process.env. Mirrors the convention every other
// script in apps/fomo/scripts/ uses.
import { setTimeout as sleep } from 'node:timers/promises';

import { InMemoryCostStore } from '../src/core/cost-tracking.js';
import { OpenAIBackend } from '../src/core/model-backends/openai.js';
import { createModelRouter } from '../src/core/model-router.js';
import { projectCalendarEvent } from '../src/adapters/google-calendar/context-source.js';
import { RANKER_OPENAI_RESPONSE_FORMAT } from '../src/ranker/openai-response-format.js';
import {
  PROMPT_VERSION,
  PROMPT_VERSION_WITH_PIL as _PROMPT_VERSION_WITH_PIL,
  buildRankerPrompt
} from '../src/ranker/prompt.js';
import { PROMPT_VERSION_WITH_CALENDAR } from '../src/ranker/calendar-prompt.js';
import { validateRankerOutput } from '../src/ranker/validator.js';
import { FIXTURES } from '../src/eval/calendar-shadow.eval.js';
import type {
  CalendarContext,
  CalendarEvent,
  RawGoogleCalendarEvent
} from '../src/adapters/google-calendar/types.js';

// ---------------------------------------------------------------------------
// Authorization gates
// ---------------------------------------------------------------------------

const AUTH = (process.env.BREVIO_REAL_MODEL_DRYRUN_AUTHORIZED ?? '').trim().toLowerCase();
if (AUTH !== 'true') {
  process.stderr.write(
    '[v0.6.0E.1] REFUSE: BREVIO_REAL_MODEL_DRYRUN_AUTHORIZED is not "true".\n' +
      '  This script issues real OpenAI API calls (20 max) and must be authorized\n' +
      '  explicitly. Re-run with BREVIO_REAL_MODEL_DRYRUN_AUTHORIZED=true.\n'
  );
  process.exit(2);
}

const API_KEY = process.env.OPENAI_API_KEY?.trim() ?? '';
if (API_KEY.length === 0) {
  process.stderr.write(
    '[v0.6.0E.1] REFUSE: OPENAI_API_KEY is not set. Cannot make real model calls.\n'
  );
  process.exit(2);
}

const MODEL = process.env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini';

// Fixed clock so Calendar offsets stay deterministic across re-runs.
const FIXED_NOW_ISO = '2026-06-09T10:00:00.000Z';
const FIXED_NOW_MS = Date.parse(FIXED_NOW_ISO);
const fixedClock = () => FIXED_NOW_MS;

// Hard cap on the total number of OpenAI calls this script issues.
const MAX_CALLS = 20;

// ---------------------------------------------------------------------------
// Helpers — copied small, intentionally, so the script is self-contained
// ---------------------------------------------------------------------------

function buildCtxFromRaw(
  raw: readonly RawGoogleCalendarEvent[],
  window_hours: number
): CalendarContext {
  const events: CalendarEvent[] = [];
  for (const r of raw) {
    const ev = projectCalendarEvent(r);
    if (ev !== null) events.push(ev);
  }
  const nearestMinutes =
    events.length === 0
      ? null
      : Math.round((Date.parse(events[0]!.start) - FIXED_NOW_MS) / 60_000);
  return Object.freeze({
    events: Object.freeze(events),
    event_count_in_window: events.length,
    nearest_event_start_offset_minutes: nearestMinutes,
    window_hours_in_force: window_hours,
    cache_hit: false
  });
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n - 1) + '…';
}

function escapeMd(s: string): string {
  return s.replace(/\|/g, '\\|').replace(/\n/g, ' ');
}

// ---------------------------------------------------------------------------
// Model wiring
// ---------------------------------------------------------------------------

const costStore = new InMemoryCostStore();
const backend = new OpenAIBackend({
  apiKey: API_KEY,
  model: MODEL,
  responseFormat: RANKER_OPENAI_RESPONSE_FORMAT
});
const router = createModelRouter({ costStore });
router.registerBackend('classification', backend);

// ---------------------------------------------------------------------------
// Call shape
// ---------------------------------------------------------------------------

interface DryRunCallResult {
  readonly ok: boolean;
  readonly variant: 'baseline' | 'calendar_aware';
  readonly prompt_version: string;
  readonly label: 'important' | 'not_important' | null;
  readonly score: number | null;
  readonly reason: string | null;
  readonly model_name: string | null;
  readonly latency_ms: number | null;
  readonly input_tokens: number | null;
  readonly output_tokens: number | null;
  readonly estimated_cost_usd: number | null;
  readonly error_code: string | null;
  readonly error_reason: string | null;
}

async function callRanker(
  variant: 'baseline' | 'calendar_aware',
  prompt: string,
  promptVersion: string,
  userId: string
): Promise<DryRunCallResult> {
  const routed = await router.route({
    capability: 'classification',
    prompt,
    prompt_version: promptVersion,
    user_id: userId,
    validate: validateRankerOutput,
    timeout_ms: 60_000
  });
  if (routed.ok) {
    return Object.freeze({
      ok: true,
      variant,
      prompt_version: promptVersion,
      label: routed.output.label,
      score: routed.output.score,
      reason: routed.output.reason,
      model_name: routed.model_name,
      latency_ms: routed.latency_ms,
      input_tokens: routed.input_tokens,
      output_tokens: routed.output_tokens,
      estimated_cost_usd: routed.estimated_cost_usd,
      error_code: null,
      error_reason: null
    });
  }
  return Object.freeze({
    ok: false,
    variant,
    prompt_version: promptVersion,
    label: null,
    score: null,
    reason: null,
    model_name: routed.model_name,
    latency_ms: null,
    input_tokens: null,
    output_tokens: null,
    estimated_cost_usd: null,
    error_code: routed.code,
    error_reason: routed.reason
  });
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

interface FixturePair {
  readonly fixture_name: string;
  readonly fixture_kind: string;
  readonly founder_verdict_on_expected: string;
  readonly baseline: DryRunCallResult;
  readonly calendar_aware: DryRunCallResult;
}

async function main(): Promise<void> {
  process.stdout.write('Phase v0.6.0E.1 — real-model dry-run on the v0.6.0D fixtures\n');
  process.stdout.write(`Model:              ${MODEL}\n`);
  process.stdout.write(`Fixed clock:        ${FIXED_NOW_ISO}\n`);
  process.stdout.write(`Fixtures:           ${FIXTURES.length} synthetic (no real PII)\n`);
  process.stdout.write(`Max model calls:    ${MAX_CALLS}\n`);
  process.stdout.write(`Cost store:         InMemoryCostStore (NO writes to production cost_records)\n`);
  process.stdout.write('\n');

  if (FIXTURES.length * 2 > MAX_CALLS) {
    process.stderr.write(
      `[v0.6.0E.1] REFUSE: fixture count (${FIXTURES.length}) × 2 = ${
        FIXTURES.length * 2
      } > MAX_CALLS (${MAX_CALLS}). Reduce FIXTURES or raise MAX_CALLS deliberately.\n`
    );
    process.exit(2);
  }

  const pairs: FixturePair[] = [];
  let callsMade = 0;

  for (let i = 0; i < FIXTURES.length; i++) {
    const f = FIXTURES[i]!;
    const short = f.name.split(' — ')[0] ?? f.name;
    process.stdout.write(`[${i + 1}/${FIXTURES.length}] ${short} ... `);

    // Build calendar context: empty for cross-user fixture (User B's
    // substrate never sees User A's calendar — same shape as v0.6.0D).
    const calendarOwnedByAlertUser = f.calendar_owner_user_id === f.alert_user_id;
    const ctx = buildCtxFromRaw(
      calendarOwnedByAlertUser ? f.raw_events : [],
      f.window_hours
    );

    const baselinePrompt = buildRankerPrompt(f.view, null, null);
    const calendarAwarePrompt = buildRankerPrompt(f.view, null, ctx, fixedClock);

    if (callsMade + 2 > MAX_CALLS) {
      process.stderr.write(`REFUSE: would exceed MAX_CALLS (${MAX_CALLS})\n`);
      process.exit(2);
    }
    const baseline = await callRanker(
      'baseline',
      baselinePrompt,
      PROMPT_VERSION,
      f.alert_user_id
    );
    callsMade++;
    // Small inter-call delay to avoid stampeding the API.
    await sleep(250);
    const calendar_aware = await callRanker(
      'calendar_aware',
      calendarAwarePrompt,
      PROMPT_VERSION_WITH_CALENDAR,
      f.alert_user_id
    );
    callsMade++;
    process.stdout.write(`baseline=${baseline.ok ? 'ok' : 'FAIL'} cal=${calendar_aware.ok ? 'ok' : 'FAIL'}\n`);

    pairs.push({
      fixture_name: f.name,
      fixture_kind: f.kind,
      founder_verdict_on_expected: f.founder_verdict,
      baseline,
      calendar_aware
    });
    await sleep(250);
  }

  process.stdout.write('\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write('JSON-LINE per fixture pair (machine-readable)\n');
  process.stdout.write('========================================================================\n');
  for (const p of pairs) {
    process.stdout.write(JSON.stringify(p) + '\n');
  }

  process.stdout.write('\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write('REAL MODEL OUTPUT — baseline vs calendar-aware (per fixture)\n');
  process.stdout.write('  These are LIVE model outputs from the production ranker, NOT hardcoded.\n');
  process.stdout.write('========================================================================\n\n');
  process.stdout.write(
    '| # | Fixture | Baseline label / score | Baseline reason (LIVE) | Calendar-aware label / score | Calendar-aware reason (LIVE) |\n'
  );
  process.stdout.write(
    '|---|---|---|---|---|---|\n'
  );
  for (let i = 0; i < pairs.length; i++) {
    const p = pairs[i]!;
    const short = p.fixture_name.split(' — ')[0] ?? p.fixture_name;
    const bl = p.baseline.ok
      ? `${p.baseline.label} / ${p.baseline.score?.toFixed(2)}`
      : `(error: ${p.baseline.error_code})`;
    const ca = p.calendar_aware.ok
      ? `${p.calendar_aware.label} / ${p.calendar_aware.score?.toFixed(2)}`
      : `(error: ${p.calendar_aware.error_code})`;
    const blReason = p.baseline.reason ?? p.baseline.error_reason ?? '';
    const caReason = p.calendar_aware.reason ?? p.calendar_aware.error_reason ?? '';
    process.stdout.write(
      `| ${i + 1} | ${escapeMd(short)} | ${bl} | ${escapeMd(truncate(blReason, 220))} | ${ca} | ${escapeMd(truncate(caReason, 220))} |\n`
    );
  }

  // -----------------------------------------------------------------------
  // Cost summary
  // -----------------------------------------------------------------------
  const costRows = await costStore.recent('user-A-founder', 1000);
  const costRowsB = await costStore.recent('user-B-friend', 1000);
  const allCostRows = [...costRows, ...costRowsB];
  const totalUsd = allCostRows.reduce((acc, r) => acc + (r.estimated_cost_usd ?? 0), 0);
  const totalInputTokens = allCostRows.reduce((acc, r) => acc + (r.input_tokens ?? 0), 0);
  const totalOutputTokens = allCostRows.reduce((acc, r) => acc + (r.output_tokens ?? 0), 0);
  const numCalls = allCostRows.length;

  process.stdout.write('\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write('COST SUMMARY (InMemoryCostStore — never written to production cost_records)\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write(`Total model calls:    ${numCalls}\n`);
  process.stdout.write(`Total input tokens:   ${totalInputTokens}\n`);
  process.stdout.write(`Total output tokens:  ${totalOutputTokens}\n`);
  process.stdout.write(`Total estimated USD:  $${totalUsd.toFixed(6)}\n`);
  if (numCalls > 0) {
    process.stdout.write(`Avg USD per call:     $${(totalUsd / numCalls).toFixed(6)}\n`);
  }
  process.stdout.write(
    'Cost rows carry: {capability, prompt_version, model_name, user_id, input_tokens,\n' +
      'output_tokens, estimated_cost_usd}. NO prompt text. NO response text.\n'
  );

  // -----------------------------------------------------------------------
  // Privacy + structural assertions
  // -----------------------------------------------------------------------
  process.stdout.write('\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write('STRUCTURAL ASSERTIONS\n');
  process.stdout.write('========================================================================\n');

  const PRIVACY_NEEDLES: readonly string[] = [
    'attendees', 'description', 'location', 'attachments', 'conferenceData',
    'organizer', 'creator', 'htmlLink', 'recurringEventId', 'hangoutLink',
    'meet.google',
    'Therapy', 'appointment',
    'A-private-strategy-meeting',
    'responseStatus', 'displayName', 'fileUrl', 'entryPoints', 'conferenceId'
  ];

  let hardFail = false;
  const fail = (msg: string): void => {
    hardFail = true;
    process.stdout.write(`[FAIL] ${msg}\n`);
  };

  // Privacy canary on every model reason text
  let canaryHits: string[] = [];
  for (const p of pairs) {
    for (const text of [p.baseline.reason ?? '', p.calendar_aware.reason ?? '']) {
      for (const needle of PRIVACY_NEEDLES) {
        if (text.includes(needle)) canaryHits.push(`${p.fixture_name} :: "${needle}"`);
      }
    }
  }
  if (canaryHits.length > 0) {
    fail(`privacy canary HITS in model output: ${canaryHits.join('; ')}`);
  } else {
    process.stdout.write(
      `[OK] privacy canary clean — 0 hits across ${PRIVACY_NEEDLES.length} excluded substrings on real model output.\n`
    );
  }

  // F10 cross-user: User A summary "A-private-strategy-meeting" must not
  // appear in the calendar-aware reason for the F10 fixture (User B alert).
  const f10 = pairs.find((p) => p.fixture_kind === 'cross_user_must_not_bleed');
  if (!f10) {
    fail('F10 cross-user fixture missing from results');
  } else {
    const f10Text = `${f10.baseline.reason ?? ''} ${f10.calendar_aware.reason ?? ''}`;
    if (f10Text.includes('A-private-strategy-meeting')) {
      fail('F10 cross-user isolation: User A calendar leaked into User B output');
    } else {
      process.stdout.write('[OK] F10 cross-user isolation clean in real model output.\n');
    }
  }

  // Count successful pairs
  const successfulPairs = pairs.filter(
    (p) => p.baseline.ok && p.calendar_aware.ok
  ).length;
  process.stdout.write(`[OK] ${successfulPairs}/${pairs.length} fixture pairs produced successful model output.\n`);
  if (successfulPairs < pairs.length) {
    process.stdout.write(
      `[INFO] ${pairs.length - successfulPairs} fixture pair(s) had model errors — see JSON-line section.\n`
    );
  }

  // Bounded call count
  if (callsMade > MAX_CALLS) {
    fail(`call count ${callsMade} > MAX_CALLS ${MAX_CALLS}`);
  } else {
    process.stdout.write(`[OK] call count = ${callsMade} (≤ MAX_CALLS ${MAX_CALLS}).\n`);
  }

  process.stdout.write('\n');
  process.stdout.write('========================================================================\n');
  process.stdout.write(
    hardFail
      ? 'VERDICT: FAIL (structural assertion broke)\n'
      : 'VERDICT: DRY-RUN PASS — real model output captured; founder taste check on table above is the load-bearing E.1 PASS gate.\n'
  );
  process.stdout.write('========================================================================\n');

  process.exit(hardFail ? 1 : 0);
}

// Only invoke when executed as the script.
import { fileURLToPath } from 'node:url';
const _runAsScript = process.argv[1] === fileURLToPath(import.meta.url);
if (_runAsScript) {
  main().catch((err) => {
    process.stderr.write(`[v0.6.0E.1] crashed: ${String(err)}\n`);
    process.exit(2);
  });
}
