// Phase 3G.1 item #10 — memory_signals snapshot at boot.
//
// Real incident captured: 2026-05-29 01:00 UTC during 3G smoke setup.
// `memory_signals.stop_active=true` had been set during the 3F.2
// smoke run 18 hours earlier (2026-05-28 01:12 UTC via the founder's
// STOP text) and silently survived into the next day. The next demo
// alert was correctly blocked by stop_enforced — but the founder had
// no boot-time visibility that stop_active was still set. Discovery
// took ~10 min of manual psql querying.
//
// Founder corrections (2026-05-29):
//   * Surface every ACTIVE signal at boot — especially stop_active=true
//   * Log kind, age, source, confidence
//   * NEVER log sensitive raw `detail` content; the detail jsonb can
//     contain arbitrary recipient/sender data on other signal kinds
//
// Factored into a standalone helper so the test suite can exercise it
// directly. The caller (index.ts boot wiring) decides how to surface
// (one INFO line per signal in the founder-scope snapshot).

import {
  type MemorySignal,
  type MemorySignalKind,
  type MemorySignalSource,
  type MemorySignalStore
} from '../memory/memory-signals.js';

export interface MemorySignalSnapshotEntry {
  readonly user_id: string;
  readonly kind: MemorySignalKind;
  readonly scope_key: string | null;
  readonly age_seconds: number;
  readonly source: MemorySignalSource;
  readonly confidence: number;
  // Boolean projection of `detail.active` ONLY for kinds that carry
  // active/inactive semantics (stop_active is the v0.1 case). This is
  // the single named-safe boolean we surface from the detail jsonb so
  // the operator can grep "active=true" without parsing the full
  // detail body. Null when the kind has no `active` semantics.
  readonly active_flag: boolean | null;
}

export interface MemorySignalBootSnapshotDeps {
  readonly memoryStore: Pick<MemorySignalStore, 'list'>;
}

export interface MemorySignalBootSnapshotOptions {
  // Defaults to 0.5. Per the founder's discipline ("important active
  // memory signals"), low-confidence inferred signals are pruned from
  // the boot surface to keep the log tight.
  readonly minConfidence?: number;
  // Returns ms-since-epoch. Injected so tests can pin a deterministic
  // "now" without touching global Date. Defaults to Date.now.
  readonly now?: () => number;
}

// Kinds for which `detail.active` is meaningful. Extend as the
// memory-signal vocabulary grows. Kept narrow so we never grab
// arbitrary jsonb keys called "active" from unrelated detail shapes.
const KINDS_WITH_ACTIVE_FLAG: readonly MemorySignalKind[] = Object.freeze(['stop_active']);

function extractActiveFlag(signal: MemorySignal): boolean | null {
  if (!(KINDS_WITH_ACTIVE_FLAG as readonly string[]).includes(signal.kind)) return null;
  const v = (signal.detail as { active?: unknown }).active;
  return typeof v === 'boolean' ? v : null;
}

/**
 * Returns a structured snapshot of every memory signal for the given
 * user that meets the confidence threshold. Read-only. Caller decides
 * how to surface (boot log line, monitoring metric, etc.).
 *
 * The snapshot intentionally does NOT include the `detail` jsonb body
 * — that field can contain arbitrary recipient context on signal
 * kinds we don't yet anticipate. We surface a single named-safe
 * `active_flag` boolean for the `stop_active` case which is the
 * v0.1 trigger for the 3G smoke incident.
 */
export async function snapshotMemorySignalsForBoot(
  user_id: string,
  deps: MemorySignalBootSnapshotDeps,
  options: MemorySignalBootSnapshotOptions = {}
): Promise<readonly MemorySignalSnapshotEntry[]> {
  const minConfidence = options.minConfidence ?? 0.5;
  const now = options.now ?? Date.now;
  const signals = await deps.memoryStore.list(user_id);

  const entries: MemorySignalSnapshotEntry[] = [];
  for (const s of signals) {
    if (s.confidence < minConfidence) continue;
    const updatedMs = new Date(s.updated_at).getTime();
    const ageMs = Math.max(0, now() - updatedMs);
    entries.push({
      user_id: s.user_id,
      kind: s.kind,
      scope_key: s.scope_key,
      age_seconds: Math.floor(ageMs / 1000),
      source: s.source,
      confidence: s.confidence,
      active_flag: extractActiveFlag(s)
    });
  }
  // Newest-first so the operator sees the most recently changed
  // signals (the ones most likely to surprise them) at the top.
  entries.sort((a, b) => a.age_seconds - b.age_seconds);
  return Object.freeze(entries.map((e) => Object.freeze(e)));
}
