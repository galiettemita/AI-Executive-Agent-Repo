// Store factory — the env-gated seam between in-memory and Postgres-backed
// substrate stores. Returns one bundle implementing every Store interface
// the rest of the system depends on.
//
// Rules:
//   * DATABASE_URL set                    → Postgres-backed bundle
//   * DATABASE_URL missing + dev          → in-memory bundle
//   * DATABASE_URL missing + production   → throws (loadDbClient rejects)
//   * BREVIO_DEV_MODE=true                → bypasses the production check
//                                            and returns in-memory bundle
//
// Tokens require a KEK. The factory takes an optional CryptoConfig; if not
// provided, it calls loadCryptoConfig() which has its own production
// fail-closed (no KEK + non-dev → throws). The two safety boundaries
// compose: a production instance without DATABASE_URL or without
// BREVIO_TOKEN_KEK refuses to start.

import { type AuditStore, InMemoryAuditStore } from '../core/audit.js';
import {
  type AlertStateTransitionStore,
  InMemoryAlertStateTransitionStore
} from '../core/alert-state-transitions.js';
import { type CostStore, InMemoryCostStore } from '../core/cost-tracking.js';
import {
  type ToolInvocationStore,
  InMemoryToolInvocationStore
} from '../core/tool-invocations.js';
import { type FeedbackStore, InMemoryFeedbackStore } from '../memory/feedback-events.js';
import { type GmailCursorStore, InMemoryGmailCursorStore } from '../memory/gmail-cursors.js';
import { type MemorySignalStore, InMemoryMemorySignalStore } from '../memory/memory-signals.js';
import { type CryptoConfig, loadCryptoConfig } from '../security/token-crypto.js';
import { InMemoryTokenStore, type TokenStore } from '../security/oauth/token-store.js';

import { type DbClientResult, loadDbClient } from './client.js';
import { PostgresAuditStore } from './stores/audit-postgres.js';
import { PostgresAlertStateTransitionStore } from './stores/transitions-postgres.js';
import { PostgresCostStore } from './stores/cost-postgres.js';
import { PostgresFeedbackStore } from './stores/feedback-postgres.js';
import { PostgresGmailCursorStore } from './stores/gmail-cursors-postgres.js';
import { PostgresMemorySignalStore } from './stores/memory-postgres.js';
import { PostgresTokenStore } from './stores/token-postgres.js';
import { PostgresToolInvocationStore } from './stores/tool-invocations-postgres.js';

export interface SubstrateStores {
  readonly audit: AuditStore;
  readonly feedback: FeedbackStore;
  readonly memory: MemorySignalStore;
  readonly cost: CostStore;
  readonly transitions: AlertStateTransitionStore;
  readonly toolInvocations: ToolInvocationStore;
  readonly tokens: TokenStore;
  readonly gmailCursors: GmailCursorStore;
}

export type StoreBackend = 'in_memory' | 'postgres';

export interface SubstrateStoresHandle {
  readonly backend: StoreBackend;
  readonly stores: SubstrateStores;
  // Only set when backend === 'postgres' — exposes the db client result so
  // the caller can shut down the pool on SIGTERM.
  readonly db: DbClientResult | null;
}

export interface CreateStoresOptions {
  env?: NodeJS.ProcessEnv;
  // Inject a CryptoConfig to skip the env-driven KEK load. Useful for tests
  // that do not want to set BREVIO_TOKEN_KEK / BREVIO_DEV_MODE process-wide.
  crypto?: CryptoConfig;
}

export function createStores(opts: CreateStoresOptions = {}): SubstrateStoresHandle {
  const env = opts.env ?? process.env;
  const dbResult = loadDbClient({ env });
  const crypto = opts.crypto ?? loadCryptoConfig();

  if (dbResult.ok) {
    return Object.freeze({
      backend: 'postgres' as const,
      stores: Object.freeze({
        audit: new PostgresAuditStore(dbResult.client),
        feedback: new PostgresFeedbackStore(dbResult.client),
        memory: new PostgresMemorySignalStore(dbResult.client),
        cost: new PostgresCostStore(dbResult.client),
        transitions: new PostgresAlertStateTransitionStore(dbResult.client),
        toolInvocations: new PostgresToolInvocationStore(dbResult.client),
        tokens: new PostgresTokenStore(dbResult.client, crypto),
        gmailCursors: new PostgresGmailCursorStore(dbResult.client)
      }),
      db: dbResult
    });
  }

  return Object.freeze({
    backend: 'in_memory' as const,
    stores: Object.freeze({
      audit: new InMemoryAuditStore(),
      feedback: new InMemoryFeedbackStore(),
      memory: new InMemoryMemorySignalStore(),
      cost: new InMemoryCostStore(),
      transitions: new InMemoryAlertStateTransitionStore(),
      toolInvocations: new InMemoryToolInvocationStore(),
      tokens: new InMemoryTokenStore(crypto),
      gmailCursors: new InMemoryGmailCursorStore()
    }),
    db: null
  });
}
