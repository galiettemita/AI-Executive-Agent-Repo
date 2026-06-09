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
  // Phase 3F.1. When false (default), the /sendblue/inbound HTTP
  // route is NOT mounted on the server — SendBlue webhook POSTs
  // return 404, no reply parsing happens. Flipping to true requires
  // (a) the SendBlue webhook signing secret wired in bootstrap and
  // (b) the 3F.2 smoke gate to PASS. Defense-in-depth at three
  // layers (mirrors 3D.1 / 3D.2 pattern): bootstrap (route not
  // mounted when false), route handler (re-checks the switch at
  // request time + audits `fomo.sendblue.kill_switch_off`), and
  // signature verification (every accepted request HMAC-verified).
  readonly sendblue_inbound_enabled: boolean;
  // Phase v0.5.12 — Live ranker reads PIL in guarded mode.
  // Q5.A global kill switch (default false). When false: pil_context is null
  // unconditionally at the production rank call site, ranker prompt is the
  // v0.5.11 baseline shape (PROMPT_VERSION='ranker-v0.2.0', no PIL block,
  // single ranker call per rank), and brevio.rank.pil_applied audits do NOT
  // fire. Behavior is bit-identical to v0.5.11. When true AND a canonical-HMAC
  // PIL row exists for the alert sender, the two-call hybrid (Q1.C-modified)
  // runs baseline + PIL ranker calls and clamps the score delta to
  // ±FOMO_PIL_SCORE_CAP. The kill switch is the load-bearing contract
  // documented in C1 + BB7 LOAD-BEARING.
  readonly pil_live_enabled: boolean;
  // Phase v0.5.12 — Q2.A hard cap on absolute PIL score delta after clamp.
  // Default 0.15; bounds [0.05, 0.25] enforced by preflight. Cap is enforced
  // at the rank-write step AFTER both model calls return — prompt-only
  // instructions are not sufficient (founder lock — without baseline call,
  // Q1.C is rejected). Documented in C4 + BB8 LOAD-BEARING.
  readonly pil_score_cap: number;
  // Phase v0.5.13 — Founder-only PIL Live Canary / Controlled Activation.
  // Per-user allowlist for the v0.5.12 live PIL ranker path. Parsed from
  // FOMO_PIL_LIVE_USER_ALLOWLIST as a comma-separated list of user_id strings.
  // Founder correction #1 (2026-06-07): the parser TRIMS whitespace per entry
  // and filters empty entries, but does NOT lowercase — match the existing
  // `FOMO_FOUNDER_USER_ID?.trim()` convention used throughout the codebase.
  // Production user_ids are 4 lowercase UUIDs + the `founder` literal, but
  // mixed-case input is not ruled out by parsing; preserve exact strings so
  // (user_id ∈ allowlist) is a strict === comparison.
  //
  // 4-case truth table (founder-locked):
  //   pil_live_enabled=false, allowlist=any         → bit-identical v0.5.11 for all users
  //   pil_live_enabled=true,  allowlist=[]          → all users baseline-only (fail-closed)
  //   pil_live_enabled=true,  allowlist=[founder]   → only founder hybrid; others baseline-only
  //   pil_live_enabled=true,  allowlist=[a,b]       → both hybrid (generic mechanism)
  //
  // Enforced at the polling worker call site BEFORE buildLivePilContext. When
  // a user_id is NOT in the allowlist, the worker falls through to the
  // single-call rankEmail path (bit-identical to pil_live_enabled=false for
  // that user) and increments the cycle counter
  // messages_pil_skipped_not_in_allowlist.
  //
  // Preflight ERRORS (founder correction #2) when pil_live_enabled=true AND
  // allowlist is empty. Runtime fail-closed if preflight is bypassed: the
  // worker treats every user as not-in-list and bootstrap emits a single
  // fomo.pil_live.allowlist_empty WARN log.
  readonly pil_live_user_allowlist: readonly string[];
  // Phase v0.6.0C — Read-only Calendar context substrate.
  // Q5.A global kill switch (default false). When false: CalendarContextSource
  // returns null unconditionally without making any Calendar API call;
  // brevio.context.calendar_built audits do NOT fire. Behavior is bit-
  // identical to v0.5.13 for the live ranker (which never receives the
  // calendar_context anyway in v0.6.0C — Calendar context is built and
  // audited but NOT passed to the rank call site until v0.6.0E).
  readonly calendar_context_enabled: boolean;
  // Per-user allowlist mirrors PIL's pattern (founder correction #1 to
  // [[v05-13-scope]]): trim-only, no-lowercase, preserve EXACT case for
  // strict === comparison. Same parser as parsePilLiveUserAllowlist;
  // separate field so allowlists stay independent (founder may run
  // Calendar canary on one user before PIL Live).
  //
  // 4-case truth table:
  //   calendar_context_enabled=false, allowlist=any  → CalendarContextSource null for all
  //   calendar_context_enabled=true,  allowlist=[]   → CalendarContextSource null for all (fail-closed)
  //   calendar_context_enabled=true,  allowlist=[f]  → only f gets Calendar API calls
  //   calendar_context_enabled=true,  allowlist=[a,b]→ both get Calendar API calls
  //
  // Preflight ERRORS (mirrors [[v05-13-scope]] correction #2) when
  // calendar_context_enabled=true AND allowlist is empty.
  readonly calendar_context_user_allowlist: readonly string[];
  // Process-local cache TTL for CalendarContextSource. Default 60s
  // ([docs/v0.6.0B-oauth-scope-readiness.md §4.3]). 0 disables the
  // cache (every call goes to the Calendar API). Capped at 10 minutes
  // to bound staleness; values above the cap fall back to default.
  readonly calendar_context_cache_ttl_ms: number;
  // Default windowHours for Calendar lookups when the caller passes a
  // non-positive value. Default 48 ([docs/v0.6.0B-oauth-scope-readiness.md §1]
  // decision row 3). Mandatory configurable from day one so future
  // non-email windows (weekly prep 168, travel 336) don't need a code
  // change to enable.
  readonly calendar_context_default_window_hours: number;
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
  outbound_max_cycles: null,
  sendblue_inbound_enabled: false,
  pil_live_enabled: false,
  pil_score_cap: 0.15,
  pil_live_user_allowlist: [] as readonly string[],
  calendar_context_enabled: false,
  calendar_context_user_allowlist: [] as readonly string[],
  calendar_context_cache_ttl_ms: 60_000,
  calendar_context_default_window_hours: 48
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

// Phase v0.5.13 — Founder correction #1 (2026-06-07): trim-only, no-lowercase.
// Parse FOMO_PIL_LIVE_USER_ALLOWLIST as a comma-separated user_id list.
// Each entry is .trim()ed (matches the existing FOMO_FOUNDER_USER_ID?.trim()
// convention used throughout the codebase). Empty entries (trailing comma /
// double comma) are filtered out. user_id strings are preserved with EXACT
// case — production user_ids are 4 lowercase UUIDs + the `founder` literal,
// but mixed-case input is not ruled out by parsing; the worker-level gate
// compares with strict === so any case-mismatch is a configuration error the
// preflight detects.
//
// Unset / empty input → []. The worker treats [] as fail-closed when global
// kill switch is on (bootstrap logs fomo.pil_live.allowlist_empty WARN).
function parsePilLiveUserAllowlist(raw: string | undefined): readonly string[] {
  if (raw === undefined) return [];
  const entries = raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
  return Object.freeze(entries);
}

// Phase v0.6.0C — bounded TTL for FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS.
// Default 60_000 (60s). Hard upper bound 600_000 (10 minutes) to keep
// the staleness of "what's on your calendar in the next 48h" bounded.
// Out-of-bounds / invalid → default. 0 is permitted (disables cache).
function parseCalendarCacheTtlMs(raw: string | undefined, fallback: number): number {
  if (raw === undefined) return fallback;
  const trimmed = raw.trim();
  if (trimmed.length === 0) return fallback;
  if (!/^\d+$/.test(trimmed)) return fallback;
  const n = Number(trimmed);
  if (!Number.isInteger(n) || n < 0 || n > 600_000) return fallback;
  return n;
}

// Phase v0.6.0C — bounded windowHours for FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS.
// Default 48. Bounds [1, 720] (1 hour to 30 days). Out-of-bounds / invalid → default.
// The 30-day upper bound is a safety belt; v0.6.0C use cases stop at 168h
// (weekly prep) but the env var is left flexible so future Brevio doesn't
// need a code change to support travel/event windows.
function parseCalendarDefaultWindowHours(raw: string | undefined, fallback: number): number {
  if (raw === undefined) return fallback;
  const trimmed = raw.trim();
  if (trimmed.length === 0) return fallback;
  if (!/^\d+$/.test(trimmed)) return fallback;
  const n = Number(trimmed);
  if (!Number.isInteger(n) || n < 1 || n > 720) return fallback;
  return n;
}

// Phase v0.5.12 — Q2.A bounded float for FOMO_PIL_SCORE_CAP.
// Bounds [0.05, 0.25] founder-locked. Out-of-range / invalid → default (safe).
// The preflight script ALSO checks the env value and produces a clearer error,
// so this parser only needs to be safe-defaulting at runtime.
function parsePilScoreCap(raw: string | undefined, fallback: number): number {
  if (raw === undefined) return fallback;
  const trimmed = raw.trim();
  if (trimmed.length === 0) return fallback;
  const n = Number(trimmed);
  if (!Number.isFinite(n)) return fallback;
  if (n < 0.05 || n > 0.25) return fallback;
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
    outbound_max_cycles: parsePositiveIntOrNull(env.FOMO_OUTBOUND_MAX_CYCLES),
    sendblue_inbound_enabled: parseBool(env.FOMO_SENDBLUE_INBOUND_ENABLED),
    pil_live_enabled: parseBool(env.FOMO_PIL_LIVE_ENABLED),
    pil_score_cap: parsePilScoreCap(env.FOMO_PIL_SCORE_CAP, DEFAULTS.pil_score_cap),
    pil_live_user_allowlist: parsePilLiveUserAllowlist(env.FOMO_PIL_LIVE_USER_ALLOWLIST),
    // Phase v0.6.0C — Calendar context substrate. Allowlist parser reused
    // from PIL (trim-only, no-lowercase) per the convention founder locked
    // in [[v05-13-scope]] correction #1. Independent field so the two
    // canaries can roll independently.
    calendar_context_enabled: parseBool(env.FOMO_CALENDAR_CONTEXT_ENABLED),
    calendar_context_user_allowlist: parsePilLiveUserAllowlist(
      env.FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST
    ),
    calendar_context_cache_ttl_ms: parseCalendarCacheTtlMs(
      env.FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS,
      DEFAULTS.calendar_context_cache_ttl_ms
    ),
    calendar_context_default_window_hours: parseCalendarDefaultWindowHours(
      env.FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS,
      DEFAULTS.calendar_context_default_window_hours
    )
  });
}

export const SAFE_DEFAULT_KILL_SWITCHES: KillSwitches = Object.freeze({ ...DEFAULTS });
