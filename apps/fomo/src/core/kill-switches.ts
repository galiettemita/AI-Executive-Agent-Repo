// Kill Switches — env-driven boolean/numeric flags that gate dangerous
// behavior. Per FOMO_PLAN §16.5: "Defaults must be safe."
//
// All boolean defaults bias toward "no effect on the world":
//   send_enabled        false  → no outbound iMessage even after approval
//   auto_send_enabled   false  → no auto-send; founder review required
//   friend_beta_enabled false  → friend onboarding blocked
//   polling_enabled     false  → Gmail polling worker is dormant
//   max_users           1      → only the founder may exist
//   polling_interval_ms 60_000 → 60s; only relevant when polling_enabled
//   polling_max_cycles  null   → unbounded; Phase 3B.3 smoke test sets
//                                this to 1 or 3 so the worker auto-stops
//                                after a controlled window
//   ranker_enabled      false  → polling worker reads Gmail but does not
//                                call the ranker. Default-off so merging
//                                Phase 3C.3 cannot accidentally start
//                                spending OpenAI credits on a founder
//                                inbox until the 3C.4 smoke run flips it.
//   slack_review_enabled false → polling worker may rank but does not
//                                post candidate cards to Slack. Default-off
//                                so merging Phase 3D.1 cannot start
//                                pinging the founder Slack channel until
//                                the 3D.2 smoke gate flips it.
//
// The Permission Gate consults the send/auto-send switches before
// allowing send-tier tools. The polling switch is consulted only by the
// Gmail polling worker bootstrap in index.ts — the gate does NOT block
// gmail.read when polling is off; ad-hoc gmail.read invocations
// (e.g. from a future admin endpoint) remain possible.
//
// env is injectable so tests don't have to mutate process.env.

export interface KillSwitches {
  readonly send_enabled: boolean;
  readonly auto_send_enabled: boolean;
  readonly friend_beta_enabled: boolean;
  readonly polling_enabled: boolean;
  readonly max_users: number;
  readonly polling_interval_ms: number;
  // null means unbounded. A positive integer caps the number of cycles
  // the polling loop runs before auto-stopping. Phase 3B.3 founder smoke
  // test sets this to a small N so the worker cannot accidentally keep
  // polling. Normal production runs leave it unset.
  readonly polling_max_cycles: number | null;
  // Phase 3C.3. When false (default), the polling worker reads Gmail
  // but does not invoke the ranker; rank_results stays empty. Flipping
  // to true requires (a) a model backend wired in bootstrap and (b) the
  // 3C.4 founder smoke gate to PASS before any friend onboarding.
  readonly ranker_enabled: boolean;
  // Phase 3D.1. When false (default), even if the ranker labels a
  // message 'important' the polling worker does NOT create an alert or
  // POST a Slack card. Flipping to true requires (a) the Slack adapter
  // wired in bootstrap (bot token + channel id) and (b) the 3D.2 smoke
  // gate to PASS. 3D.1 alerts created here sit in `queued_for_review`
  // indefinitely until 3D.2 adds approval capture; never reaches 3E
  // SendBlue paths.
  readonly slack_review_enabled: boolean;
  // Phase 3E.2. Mirrors `polling_max_cycles` but for the outbound
  // sender worker. null = unbounded (production default). A positive
  // integer caps the number of `runOutboundOnce` cycles the worker
  // runs before auto-stopping and emitting
  // `fomo.outbound.cycle_cap_reached`. The 3E.2 founder smoke test
  // sets this to a small N (1-3) so the worker cannot accidentally
  // keep firing real iMessages against SendBlue during the smoke
  // window. The Permission Gate does NOT consult this — it only
  // gates which-tools-may-run; the cap is a bootstrap-level safety
  // belt on the worker loop itself.
  readonly outbound_max_cycles: number | null;
}

const DEFAULTS = {
  send_enabled: false,
  auto_send_enabled: false,
  friend_beta_enabled: false,
  polling_enabled: false,
  max_users: 1,
  polling_interval_ms: 60_000,
  polling_max_cycles: null,
  ranker_enabled: false,
  slack_review_enabled: false,
  outbound_max_cycles: null
} as const satisfies KillSwitches;

// Strict opt-in parse: only the literal strings 'true' or '1' (case-insensitive,
// trimmed) enable a switch. Anything else — including 'yes', 'on', '2', 'TRUE\n',
// or unset — is treated as false. This is intentional: we want explicit
// confirmation, not loose truthiness, before any kill switch flips on.
function parseBool(raw: string | undefined): boolean {
  if (raw === undefined) return false;
  const v = raw.trim().toLowerCase();
  return v === 'true' || v === '1';
}

// Positive decimal integer or fallback. Strict /^\d+$/ — values like '1e3',
// '0x10', '3.7', '-5', or 'abc' all fall through to the safe default. A
// misconfigured FOMO_MAX_USERS should not crash boot, and ambiguous numeric
// formats should not silently expand the user cap (1e3 → 1000 would be
// surprising for a user who typed '1e3' as a typo).
function parsePositiveIntSafe(raw: string | undefined, fallback: number): number {
  if (raw === undefined) return fallback;
  const trimmed = raw.trim();
  if (!/^\d+$/.test(trimmed)) return fallback;
  const n = Number(trimmed);
  if (!Number.isInteger(n) || n <= 0) return fallback;
  return n;
}

// Optional positive integer; unset / invalid → null (unbounded).
// Distinct from parsePositiveIntSafe which returns a numeric fallback.
function parsePositiveIntOrNull(raw: string | undefined): number | null {
  if (raw === undefined) return null;
  const trimmed = raw.trim();
  if (trimmed.length === 0) return null;
  if (!/^\d+$/.test(trimmed)) return null;
  const n = Number(trimmed);
  if (!Number.isInteger(n) || n <= 0) return null;
  return n;
}

export function loadKillSwitches(env: NodeJS.ProcessEnv = process.env): KillSwitches {
  return Object.freeze({
    send_enabled: parseBool(env.FOMO_SEND_ENABLED),
    auto_send_enabled: parseBool(env.FOMO_AUTO_SEND_ENABLED),
    friend_beta_enabled: parseBool(env.FOMO_FRIEND_BETA_ENABLED),
    polling_enabled: parseBool(env.FOMO_GMAIL_POLLING_ENABLED),
    max_users: parsePositiveIntSafe(env.FOMO_MAX_USERS, DEFAULTS.max_users),
    polling_interval_ms: parsePositiveIntSafe(
      env.FOMO_GMAIL_POLLING_INTERVAL_MS,
      DEFAULTS.polling_interval_ms
    ),
    polling_max_cycles: parsePositiveIntOrNull(env.FOMO_GMAIL_POLLING_MAX_CYCLES),
    ranker_enabled: parseBool(env.FOMO_RANKER_ENABLED),
    slack_review_enabled: parseBool(env.FOMO_SLACK_REVIEW_ENABLED),
    outbound_max_cycles: parsePositiveIntOrNull(env.FOMO_OUTBOUND_MAX_CYCLES)
  });
}

export const SAFE_DEFAULT_KILL_SWITCHES: KillSwitches = Object.freeze({ ...DEFAULTS });
