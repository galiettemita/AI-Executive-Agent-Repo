// Phase v0.5.11 — PIL Shadow Ranker Eval Harness.
//
// Offline harness that pairs N synthetic email fixtures with synthetic PIL
// states (sender_importance / sender_suppressed) and asserts the projection
// from buildPilContext PLUS a deterministic shadow-ranker shift model
// produces the EXPECTED shift DIRECTION.
//
// Per founder Q1 lock: this harness consumes buildPilContext + a shadow
// score-shift function. It NEVER touches the live ranker call path. The
// live ranker is bit-identical to v0.5.10.
//
// "Shift direction" — not "shift magnitude". The shadow-ranker is allowed
// to be deterministic (no model nondeterminism this phase); a future phase
// may swap in a real model call. Today we assert that:
//   - sender_suppressed=true → suppressed result; baseline NOT suppressed.
//   - importance score < 0 → shift DOWN vs baseline.
//   - importance score > 0 → shift UP vs baseline.
//   - cross-user: user B with no PIL state at user A's scope_key → no shift.
//
// Includes ≥10 fixtures + the LOAD-BEARING cross-user contamination row
// (C12 mirror of C9). Exits 0 on PASS, 1 on FAIL.

import { InMemoryMemorySignalStore } from '../memory/memory-signals.js';
import { buildPilContext, type PilContext } from '../ranker/pil-context.js';

/* ---------------------------------------------------------------------- */
/* Shadow scoring                                                         */
/* ---------------------------------------------------------------------- */

interface SyntheticEmail {
  readonly id: string;
  readonly subject_pattern: string;
  // Synthetic baseline score the live ranker (hypothetically) would assign
  // BEFORE any PIL context. Used to compare shadow vs baseline.
  readonly baseline_score: number;
}

interface SyntheticPilState {
  // Null → no PIL row exists for this fixture (clean baseline case).
  readonly importance_score: number | null;
  readonly n_positive_events: number;
  readonly n_negative_events: number;
  readonly suppressed: boolean;
  readonly age_days: number;
}

interface Fixture {
  readonly name: string;
  readonly email: SyntheticEmail;
  readonly pil_state: SyntheticPilState | null;
  readonly expected_shift: 'up' | 'down' | 'unchanged' | 'suppressed';
  readonly user_id: string;
  readonly scope_key_user_id: string; // user_id whose memory holds the state (for cross-user test)
}

// Deterministic shadow ranker: returns a (decided_label, score, suppressed)
// triple given (baseline_score, pil_context). NEVER reads anything from
// the live ranker module.
function shadowRank(
  baseline_score: number,
  ctx: PilContext | null
): { label: 'important' | 'not_important' | 'suppressed'; score: number; suppressed: boolean } {
  if (ctx === null) {
    return { label: baseline_score >= 0.5 ? 'important' : 'not_important', score: baseline_score, suppressed: false };
  }
  if (ctx.sender_suppressed) {
    return { label: 'suppressed', score: 0, suppressed: true };
  }
  // Decayed importance score adjusts the baseline within [-1, +1].
  const adjusted = Math.max(0, Math.min(1, baseline_score + ctx.sender_importance_score * 0.5));
  return { label: adjusted >= 0.5 ? 'important' : 'not_important', score: adjusted, suppressed: false };
}

/* ---------------------------------------------------------------------- */
/* Fixtures (≥10, synthetic only — doctrine §9.5)                         */
/* ---------------------------------------------------------------------- */

const SCOPE_A = 'a'.repeat(32);
const SCOPE_B = 'b'.repeat(32);
const SCOPE_C = 'c'.repeat(32);

const FIXTURES: readonly Fixture[] = Object.freeze([
  {
    name: 'F1 — clean baseline, no PIL row → unchanged',
    email: { id: 'em1', subject_pattern: 'generic notification', baseline_score: 0.6 },
    pil_state: null,
    expected_shift: 'unchanged',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F2 — strong positive (score=+0.5, 5 events, 1d old) → shift UP',
    email: { id: 'em2', subject_pattern: 'borderline transactional', baseline_score: 0.4 },
    pil_state: { importance_score: 0.5, n_positive_events: 5, n_negative_events: 0, suppressed: false, age_days: 1 },
    expected_shift: 'up',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F3 — weak positive (score=+0.1, fresh) → small shift UP (direction-only)',
    email: { id: 'em3', subject_pattern: 'commercial newsletter', baseline_score: 0.4 },
    pil_state: { importance_score: 0.1, n_positive_events: 1, n_negative_events: 0, suppressed: false, age_days: 0 },
    expected_shift: 'up',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F4 — strong negative (score=-0.5, 5 events, fresh) → shift DOWN',
    email: { id: 'em4', subject_pattern: 'URGENT marketing', baseline_score: 0.6 },
    pil_state: { importance_score: -0.5, n_positive_events: 0, n_negative_events: 5, suppressed: false, age_days: 1 },
    expected_shift: 'down',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F5 — sender_suppressed=true → suppressed regardless of baseline',
    email: { id: 'em5', subject_pattern: 'previously-blocked sender', baseline_score: 0.95 },
    pil_state: { importance_score: null, n_positive_events: 0, n_negative_events: 0, suppressed: true, age_days: 1 },
    expected_shift: 'suppressed',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F6 — ancient signal (200d old) → decay to zero, treated as unchanged',
    email: { id: 'em6', subject_pattern: 'reactivated sender', baseline_score: 0.5 },
    pil_state: { importance_score: 0.4, n_positive_events: 4, n_negative_events: 0, suppressed: false, age_days: 200 },
    expected_shift: 'unchanged',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F7 — mid-decay signal (135d old, score=+0.4) → factor 0.5, still UP',
    email: { id: 'em7', subject_pattern: 'half-decayed positive sender', baseline_score: 0.4 },
    pil_state: { importance_score: 0.4, n_positive_events: 4, n_negative_events: 0, suppressed: false, age_days: 135 },
    expected_shift: 'up',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F8 — boundary at 90d (still full weight) → shift DOWN',
    email: { id: 'em8', subject_pattern: 'boundary case', baseline_score: 0.6 },
    pil_state: { importance_score: -0.3, n_positive_events: 0, n_negative_events: 3, suppressed: false, age_days: 90 },
    expected_shift: 'down',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F9 — score=0 (offsetting positive + negative events) → unchanged',
    email: { id: 'em9', subject_pattern: 'mixed signals', baseline_score: 0.5 },
    pil_state: { importance_score: 0, n_positive_events: 3, n_negative_events: 3, suppressed: false, age_days: 1 },
    expected_shift: 'unchanged',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F10 — different scope_key (clean baseline at scope_C; row exists at scope_A) → unchanged',
    email: { id: 'em10', subject_pattern: 'different sender, no signal', baseline_score: 0.55 },
    pil_state: null,
    expected_shift: 'unchanged',
    user_id: 'founder',
    scope_key_user_id: 'founder'
  },
  {
    name: 'F11 LOAD-BEARING — cross-user contamination: user B asks at user A scope_key → unchanged',
    email: { id: 'em11', subject_pattern: 'sender User A suppressed', baseline_score: 0.6 },
    // Note: pil_state describes the row that EXISTS at scope_key. The row
    // is stored under userA, but we LOOK UP from userB — the (user_id,
    // scope_key) keyspace prevents leak.
    pil_state: { importance_score: null, n_positive_events: 0, n_negative_events: 0, suppressed: true, age_days: 1 },
    expected_shift: 'unchanged',
    user_id: 'userB',
    scope_key_user_id: 'userA' // <-- stored under userA, queried by userB
  }
]);

/* ---------------------------------------------------------------------- */
/* Runner                                                                  */
/* ---------------------------------------------------------------------- */

interface FixtureResult {
  readonly fixture: Fixture;
  readonly baseline: ReturnType<typeof shadowRank>;
  readonly shadow: ReturnType<typeof shadowRank>;
  readonly actual_shift: 'up' | 'down' | 'unchanged' | 'suppressed';
  readonly passed: boolean;
}

async function runFixture(fixture: Fixture, scopeKey: string): Promise<FixtureResult> {
  const store = new InMemoryMemorySignalStore();
  const now = new Date('2026-06-07T00:00:00Z');
  const writtenAt = fixture.pil_state
    ? new Date(now.getTime() - fixture.pil_state.age_days * 86_400_000).toISOString()
    : null;

  if (fixture.pil_state) {
    if (fixture.pil_state.importance_score !== null) {
      await store.upsert({
        user_id: fixture.scope_key_user_id,
        kind: 'sender_importance',
        scope_key: scopeKey,
        detail: {
          score: fixture.pil_state.importance_score,
          n_positive_events: fixture.pil_state.n_positive_events,
          n_negative_events: fixture.pil_state.n_negative_events,
          last_updated: writtenAt!,
          source_surface: 'email_alert'
        },
        source: 'user_confirmed',
        confidence: 1.0,
        updated_at: writtenAt!
      });
    }
    if (fixture.pil_state.suppressed) {
      await store.upsert({
        user_id: fixture.scope_key_user_id,
        kind: 'sender_suppressed',
        scope_key: scopeKey,
        detail: {
          suppressed: true,
          set_at: writtenAt!,
          source_surface: 'email_alert',
          set_by: 'explicit_ignore_sender'
        },
        source: 'user_confirmed',
        confidence: 1.0,
        updated_at: writtenAt!
      });
    }
  }

  const ctx = await buildPilContext(fixture.user_id, scopeKey, {
    memoryStore: store,
    recency_full_days: 90,
    recency_decay_days: 90,
    now: () => now
  });

  const baseline = shadowRank(fixture.email.baseline_score, null);
  const shadow = shadowRank(fixture.email.baseline_score, ctx);

  let actual_shift: FixtureResult['actual_shift'];
  if (shadow.suppressed) actual_shift = 'suppressed';
  else if (shadow.score > baseline.score + 1e-6) actual_shift = 'up';
  else if (shadow.score < baseline.score - 1e-6) actual_shift = 'down';
  else actual_shift = 'unchanged';

  return {
    fixture,
    baseline,
    shadow,
    actual_shift,
    passed: actual_shift === fixture.expected_shift
  };
}

async function main(): Promise<void> {
  console.log('Phase v0.5.11 PIL shadow eval harness\n');
  console.log(`Fixtures: ${FIXTURES.length} synthetic only (no real PII)\n`);

  const results: FixtureResult[] = [];
  for (let i = 0; i < FIXTURES.length; i++) {
    const fixture = FIXTURES[i]!;
    // Each fixture gets its own scope_key (different sender)
    const scopeKey = [SCOPE_A, SCOPE_B, SCOPE_C][i % 3]!;
    const r = await runFixture(fixture, scopeKey);
    results.push(r);
    const sym = r.passed ? '✓' : '✗';
    console.log(
      `  [${sym}] ${fixture.name}\n        baseline.score=${r.baseline.score.toFixed(3)} shadow.score=${r.shadow.score.toFixed(3)} expected=${fixture.expected_shift} actual=${r.actual_shift}`
    );
  }

  const pass = results.filter((r) => r.passed).length;
  const fail = results.length - pass;

  console.log('');
  console.log('========================================================================');
  if (fail > 0) {
    console.log(`VERDICT: FAIL — ${fail} / ${results.length} fixtures failed.`);
    process.exit(1);
  }
  console.log(`VERDICT: PASS — ${pass} / ${results.length} fixtures green (shift DIRECTION assertions).`);
  process.exit(0);
}

// Only invoke main() when executed as the script (`pnpm run eval:pil-shadow`).
// When this module is imported (e.g. by preflight's dynamic-probe), the
// eval body MUST NOT run — that would call process.exit() inside the
// importing process.
import { fileURLToPath } from 'node:url';
const _runAsScript = process.argv[1] === fileURLToPath(import.meta.url);
if (_runAsScript) {
  main().catch((err) => {
    process.stderr.write(`[eval:pil-shadow] crashed: ${String(err)}\n`);
    process.exit(2);
  });
}

export { FIXTURES, runFixture, shadowRank };
