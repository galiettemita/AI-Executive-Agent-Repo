// Phase v0.6.0C preflight — Read-only Calendar context substrate.
//
// LOAD-BEARING check: when FOMO_CALENDAR_CONTEXT_ENABLED=true AND
// FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST is empty/unset, preflight ERRORS
// (mirrors [[v05-13-scope]] founder correction #2). Runtime separately
// fails closed by treating every user as not-in-list, but preflight has
// to be loud at the boot gate so the operator notices.
//
// Pure config inspection — no DB, no network.

interface Finding {
  readonly level: 'ERROR' | 'WARN' | 'OK';
  readonly name: string;
  readonly message: string;
}

const findings: Finding[] = [];

function record(level: Finding['level'], name: string, message: string): void {
  findings.push({ level, name, message });
}

const env = process.env;

const enabledRaw = (env.FOMO_CALENDAR_CONTEXT_ENABLED ?? '').trim().toLowerCase();
const enabled = enabledRaw === 'true' || enabledRaw === '1';

const allowlistRaw = env.FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST ?? '';
const allowlist = allowlistRaw
  .split(',')
  .map((s) => s.trim())
  .filter((s) => s.length > 0);

/* ----- LOAD-BEARING: enabled=true AND allowlist empty → ERROR ---------- */
if (enabled && allowlist.length === 0) {
  record(
    'ERROR',
    'FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST',
    'FOMO_CALENDAR_CONTEXT_ENABLED=true but FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST is empty or unset. ' +
      'This combination would cause the runtime to fail closed silently — every user would be ' +
      'treated as not-in-list. Set the allowlist to the comma-separated user_ids that should get ' +
      'Calendar context, OR set FOMO_CALENDAR_CONTEXT_ENABLED=false. (Per v0.5.13 founder correction #2.)'
  );
} else if (enabled) {
  record('OK', 'FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST', `allowlist size = ${allowlist.length}`);
} else {
  record('OK', 'FOMO_CALENDAR_CONTEXT_ENABLED', 'global kill switch OFF — substrate dormant');
}

/* ----- Window hours bounds ---------------------------------------------- */
const windowRaw = (env.FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS ?? '').trim();
if (windowRaw.length > 0) {
  if (!/^\d+$/.test(windowRaw)) {
    record(
      'WARN',
      'FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS',
      `value "${windowRaw}" is not a positive integer — runtime will fall back to default 48h.`
    );
  } else {
    const n = Number(windowRaw);
    if (n < 1 || n > 720) {
      record(
        'WARN',
        'FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS',
        `value ${n} is outside the [1, 720] bound — runtime will fall back to default 48h.`
      );
    } else {
      record('OK', 'FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS', `${n}h`);
    }
  }
} else {
  record('OK', 'FOMO_CALENDAR_CONTEXT_DEFAULT_WINDOW_HOURS', 'unset — using default 48h');
}

/* ----- Cache TTL bounds ------------------------------------------------- */
const ttlRaw = (env.FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS ?? '').trim();
if (ttlRaw.length > 0) {
  if (!/^\d+$/.test(ttlRaw)) {
    record(
      'WARN',
      'FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS',
      `value "${ttlRaw}" is not a non-negative integer — runtime will fall back to default 60000ms.`
    );
  } else {
    const n = Number(ttlRaw);
    if (n > 600_000) {
      record(
        'WARN',
        'FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS',
        `value ${n}ms exceeds the 600000ms (10 minute) bound — runtime will fall back to default 60000ms.`
      );
    } else {
      record('OK', 'FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS', `${n}ms`);
    }
  }
} else {
  record('OK', 'FOMO_CALENDAR_CONTEXT_CACHE_TTL_MS', 'unset — using default 60000ms');
}

/* ----- v0.6.0C invariant: live ranker NOT touched ---------------------- */
// v0.6.0C builds + audits Calendar context but does NOT pass it to the
// production rank call site. This is a substrate-only phase. Surfacing
// this as an OK line keeps the boundary visible.
record(
  'OK',
  'v0.6.0C scope boundary',
  'Calendar context is built + audited but NOT passed to the live ranker. Live ranker stays bit-identical to v0.5.13. v0.6.0E is the phase that wires Calendar into the ranker.'
);

/* ----- Output ----------------------------------------------------------- */
let hadError = false;
let hadWarn = false;
for (const f of findings) {
  if (f.level === 'ERROR') {
    hadError = true;
    process.stderr.write(`[ERROR] ${f.name}: ${f.message}\n`);
  } else if (f.level === 'WARN') {
    hadWarn = true;
    process.stdout.write(`[WARN]  ${f.name}: ${f.message}\n`);
  } else {
    process.stdout.write(`[OK]    ${f.name}: ${f.message}\n`);
  }
}
process.stdout.write(
  `\npreflight v0.6.0C: ${hadError ? 'FAIL' : hadWarn ? 'OK (with warnings)' : 'OK'}\n`
);
process.exit(hadError ? 1 : 0);
