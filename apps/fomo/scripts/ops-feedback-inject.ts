// Phase v0.5.9 — Founder-only ops CLI for the Brevio-wide feedback substrate.
//
// LOCAL/DEV-ONLY per founder lock 2026-06-06. This script:
//   * Is a CLI tool, NOT a route. There is no HTTP endpoint.
//   * REFUSES to run when NODE_ENV=production unless the explicit override
//     env var FOMO_OPS_DEV_OVERRIDE=true is set.
//   * Writes a single feedback_event through the regular FeedbackStore.write
//     path (active-surface gate applies) and invokes applyFeedback for the
//     consumer side.
//   * Surfaces the gate rejection (BrevioFeedbackError) cleanly so smoke
//     §6 Test 5 (LOAD-BEARING) can prove the substrate rejects
//     declared-but-inactive surfaces like 'calendar_reminder'.
//
// USAGE:
//   pnpm --filter @brevio/fomo run ops:feedback-inject -- \
//     --user-id founder \
//     --kind ignored \
//     --source-surface email_alert \
//     --dimension sender \
//     --sender 'noisy-newsletter@example.com'
//
//   To prove the active-surface gate (smoke Test 5):
//   pnpm --filter @brevio/fomo run ops:feedback-inject -- \
//     --user-id founder \
//     --kind ignored \
//     --source-surface calendar_reminder \
//     --dimension event \
//     --sender 'unused-for-calendar'
//   → Exits non-zero with the BrevioFeedbackError('inactive_surface') message.
//
// Privacy: --sender is plain text input (founder types it). The value is
// stored in the legacy feedback_events.sender_email column (v0.5.x state,
// unchanged), AND used to derive the per-user HMAC scope_key for the
// memory_signal upsert. The raw email NEVER lands in the new
// brevio.feedback.applied audit detail or sender_feedback_ignored signal
// detail per the founder approval-time privacy guardrail.
//
// This script is the bridge: until reply-parser feedback intents and
// Slack-button-for-ignored-sender ship in future phases, ops-inject is
// the founder's way to exercise the feedback → memory_signal pipe.

import { loadDbClient } from '../src/db/client.js';
import { PostgresAuditStore } from '../src/db/stores/audit-postgres.js';
import { PostgresFeedbackStore } from '../src/db/stores/feedback-postgres.js';
import { PostgresMemorySignalStore } from '../src/db/stores/memory-postgres.js';
import { applyFeedback, loadSenderHashKey } from '../src/memory/feedback-apply.js';
import {
  BrevioFeedbackError,
  mapLegacyFeedbackKind,
  type FeedbackEventInput
} from '../src/memory/feedback-events.js';

interface ParsedArgs {
  user_id: string;
  kind: string;
  source_surface: string;
  dimension: string | null;
  sender: string | null;
  alert_id: string | null;
  role: string | null;
  detail_overrides: Record<string, unknown>;
}

function parseArgs(argv: readonly string[]): ParsedArgs {
  const out: Partial<ParsedArgs> = {
    detail_overrides: {}
  };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = argv[i + 1];
    const takeStr = (target: keyof ParsedArgs): void => {
      if (typeof next === 'string') {
        (out as Record<string, unknown>)[target] = next;
        i++;
      } else {
        process.stderr.write(`[ops:feedback-inject] ${a} requires a value\n`);
        process.exit(2);
      }
    };
    if (a === '--user-id') takeStr('user_id');
    else if (a === '--kind') takeStr('kind');
    else if (a === '--source-surface') takeStr('source_surface');
    else if (a === '--dimension') takeStr('dimension');
    else if (a === '--sender') takeStr('sender');
    else if (a === '--alert-id') takeStr('alert_id');
    else if (a === '--role') takeStr('role');
  }
  if (!out.user_id) {
    process.stderr.write('[ops:feedback-inject] --user-id <id> is required\n');
    process.exit(2);
  }
  if (!out.kind) {
    process.stderr.write('[ops:feedback-inject] --kind <generic-or-legacy> is required\n');
    process.exit(2);
  }
  return {
    user_id: out.user_id,
    kind: out.kind,
    source_surface: out.source_surface ?? 'email_alert',
    dimension: out.dimension ?? null,
    sender: out.sender ?? null,
    alert_id: out.alert_id ?? null,
    role: out.role ?? null,
    detail_overrides: out.detail_overrides ?? {}
  };
}

async function main(): Promise<void> {
  // LOCAL/DEV-ONLY production guard (founder lock).
  const nodeEnv = (process.env.NODE_ENV ?? '').trim().toLowerCase();
  const devOverride = (process.env.FOMO_OPS_DEV_OVERRIDE ?? '').trim() === 'true';
  if (nodeEnv === 'production' && !devOverride) {
    process.stderr.write(
      '[ops:feedback-inject] REFUSING to run when NODE_ENV=production.\n' +
        '                     This script is LOCAL/DEV-ONLY per founder lock 2026-06-06.\n' +
        '                     To force-run in production (smoke-only): set FOMO_OPS_DEV_OVERRIDE=true.\n'
    );
    process.exit(2);
  }

  const args = parseArgs(process.argv.slice(2));

  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[ops:feedback-inject] DATABASE_URL is not set. Source .env first.\n');
    process.exit(2);
  }

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[ops:feedback-inject] DB client load failed: ${dbResult.reason}\n`);
    process.exit(2);
  }
  const db = dbResult.client;

  // Stores. We construct directly (no store-factory) to keep this script
  // self-contained and avoid pulling kill-switches / OAuth / SendBlue wiring.
  const auditStore = new PostgresAuditStore(db);
  const feedbackStore = new PostgresFeedbackStore(db);
  const memoryStore = new PostgresMemorySignalStore(db);

  // Sender hash key — required for the consumer (applyFeedback) to derive
  // the privacy-preserving scope_key. Surfaces a clear message if missing.
  let senderHashKey: Buffer;
  try {
    senderHashKey = loadSenderHashKey(process.env);
  } catch (err) {
    process.stderr.write(
      `[ops:feedback-inject] ${err instanceof Error ? err.message : String(err)}\n`
    );
    process.exit(2);
  }

  // Compose the detail overlay (legacy mapping + caller-supplied dimension/role).
  const mapped = mapLegacyFeedbackKind(args.kind);
  const detail: Record<string, unknown> = {
    // dimension: caller > mapping overlay > undefined
    ...(args.dimension ? { dimension: args.dimension } : mapped?.overlay.dimension ? { dimension: mapped.overlay.dimension } : {}),
    // role: caller > mapping overlay > undefined
    ...(args.role ? { role: args.role } : mapped?.overlay.role ? { role: mapped.overlay.role } : {}),
    ...args.detail_overrides
  };

  const input: FeedbackEventInput = {
    user_id: args.user_id,
    alert_id: args.alert_id,
    sender_email: args.sender,
    kind: args.kind as FeedbackEventInput['kind'],
    source_surface: args.source_surface,
    detail
  };

  let writtenEvent;
  try {
    writtenEvent = await feedbackStore.write(input);
  } catch (err) {
    if (err instanceof BrevioFeedbackError) {
      // Active-surface or unknown-surface rejection. Emit the failure audit
      // so smoke C6 / C7 can observe the rejection in audit_log.
      await auditStore.write({
        actor_user_id: args.user_id,
        actor_ip: null,
        actor_user_agent: null,
        action: 'feedback.written',
        target: 'ops:feedback-inject',
        result: 'failure',
        detail: {
          source_surface: 'email_alert', // the script's own surface — NOT the attempted value
          rejection_reason: err.code,
          attempted_source_surface: err.attempted_source_surface,
          verb: mapped?.verb ?? args.kind,
          legacy_kind: mapped ? args.kind : undefined
        }
      });
      process.stderr.write(
        `[ops:feedback-inject] REJECTED — ${err.message}\n` +
          `                    audit row written (action=feedback.written, result=failure, rejection_reason=${err.code}).\n` +
          `                    Per v0.5.9 scope, only source_surface='email_alert' is active.\n`
      );
      process.exit(3);
    }
    throw err;
  }

  // Emit the success-side feedback.written audit with the v0.5.9 extended
  // detail (Q6.A). The ops CLI is also the canonical pattern future
  // surfaces will mirror, so the detail shape here is the contract.
  await auditStore.write({
    actor_user_id: args.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'feedback.written',
    target: writtenEvent.alert_id ? `alert:${writtenEvent.alert_id}` : 'ops:feedback-inject',
    result: 'success',
    detail: {
      feedback_event_id: writtenEvent.id ?? null,
      source_surface: writtenEvent.source_surface,
      verb: mapped?.verb ?? args.kind,
      dimension: (detail.dimension as string | undefined) ?? undefined,
      role: (detail.role as string | undefined) ?? undefined,
      legacy_kind: mapped ? args.kind : undefined,
      sender_present: writtenEvent.sender_email !== null
    }
  });

  // Apply: invoke the consumer. For v0.5.9 this fires ONLY for
  // (email_alert, ignored, sender). Other tuples return no_match.
  const applied = await applyFeedback(writtenEvent, {
    memoryStore,
    auditStore,
    senderHashKey
  });

  // Human-readable output. Privacy: print scope_key hash + length, NOT the
  // raw sender email (which the caller already knows from the --sender arg
  // they typed).
  process.stdout.write(
    `[ops:feedback-inject] WROTE feedback_events.id=${writtenEvent.id ?? '?'} ` +
      `source_surface=${writtenEvent.source_surface} kind=${writtenEvent.kind} ` +
      `mapped_verb=${mapped?.verb ?? '(generic)'} dimension=${(detail.dimension as string | undefined) ?? '<unset>'}\n`
  );
  if (applied.kind === 'applied') {
    process.stdout.write(
      `[ops:feedback-inject] APPLIED memory_signal ${applied.memory_signal_kind} ` +
        `action=${applied.memory_signal_action} scope_key_hash=${applied.scope_key_hash} ` +
        `(${applied.scope_key_hash.length} hex chars) ignored_count=${applied.ignored_count} ` +
        `confidence=${applied.confidence.toFixed(3)}\n` +
        `[ops:feedback-inject] AUDIT brevio.feedback.applied emitted.\n`
    );
  } else {
    process.stdout.write(
      `[ops:feedback-inject] NO-MATCH (no consumer arm for (source_surface=${writtenEvent.source_surface}, verb=${mapped?.verb ?? writtenEvent.kind}, dimension=${(detail.dimension as string | undefined) ?? '<unset>'})). ` +
        `In v0.5.9 only (email_alert, ignored, sender) triggers an apply.\n`
    );
  }

  process.exit(0);
}

main().catch((err) => {
  process.stderr.write(`[ops:feedback-inject] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
