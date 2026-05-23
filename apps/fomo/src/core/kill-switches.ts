// Kill Switches — env-driven boolean/numeric flags that gate dangerous
// behavior. Per FOMO_PLAN §16.5: "Defaults must be safe."
//
// All four defaults bias toward "no effect on the world":
//   send_enabled        false  → no outbound iMessage even after approval
//   auto_send_enabled   false  → no auto-send; founder review required
//   friend_beta_enabled false  → friend onboarding blocked
//   max_users           1      → only the founder may exist
//
// The Permission Gate consults these before allowing send-tier tools.
// env is injectable so tests don't have to mutate process.env.

export interface KillSwitches {
  readonly send_enabled: boolean;
  readonly auto_send_enabled: boolean;
  readonly friend_beta_enabled: boolean;
  readonly max_users: number;
}

const DEFAULTS = {
  send_enabled: false,
  auto_send_enabled: false,
  friend_beta_enabled: false,
  max_users: 1
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

export function loadKillSwitches(env: NodeJS.ProcessEnv = process.env): KillSwitches {
  return Object.freeze({
    send_enabled: parseBool(env.FOMO_SEND_ENABLED),
    auto_send_enabled: parseBool(env.FOMO_AUTO_SEND_ENABLED),
    friend_beta_enabled: parseBool(env.FOMO_FRIEND_BETA_ENABLED),
    max_users: parsePositiveIntSafe(env.FOMO_MAX_USERS, DEFAULTS.max_users)
  });
}

export const SAFE_DEFAULT_KILL_SWITCHES: KillSwitches = Object.freeze({ ...DEFAULTS });
