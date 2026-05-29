import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import { loadFomoConfig } from './config.js';
import { handleHealth } from './routes/health.js';
import { tryHandleOAuthGoogleRequest, type OAuthGoogleRouteDeps } from './routes/oauth-google.js';
import { GmailClient } from './adapters/gmail/client.js';
import { loadDbClient } from './db/client.js';
import {
  PendingMigrationsError,
  verifyMigrationsOrThrow
} from './db/migration-verifier.js';
import { createStores, type SubstrateStoresHandle } from './db/store-factory.js';
import { loadKillSwitches, type KillSwitches } from './core/kill-switches.js';
import { createToolRegistry } from './core/tool-registry.js';
import { type PolicyGateDeps } from './core/policy-gate.js';
import { createDispatchTable, type DispatchTable } from './dispatch/dispatcher.js';
import { wireInternalExecutors } from './dispatch/internal-executors.js';
import { wireExternalExecutors } from './dispatch/external-executors.js';
import {
  runOnce as runPollOnce,
  type GmailPollRankerDep,
  type GmailPollSlackReviewDep
} from './workers/gmail-poll.js';
import { findUsersNeedingReauth } from './workers/needs-reauth-boot-check.js';
import { snapshotMemorySignalsForBoot } from './workers/memory-signals-boot-snapshot.js';
import { OpenAIBackend } from './core/model-backends/openai.js';
import { createModelRouter } from './core/model-router.js';
import { rankEmail } from './ranker/index.js';
import { RANKER_OPENAI_RESPONSE_FORMAT } from './ranker/openai-response-format.js';
import { SlackClient } from './adapters/slack/client.js';
import { SendBlueClient } from './adapters/sendblue/client.js';
import {
  tryHandleSlackInteractivityRequest,
  type SlackInteractivityRouteDeps
} from './routes/slack-interactivity.js';
import {
  tryHandleSendBlueInboundRequest,
  type SendBlueInboundRouteDeps
} from './routes/sendblue-inbound.js';
import { parseReply } from './reply-parser/index.js';
import { REPLY_PARSER_OPENAI_RESPONSE_FORMAT } from './reply-parser/openai-response-format.js';
import { type RawEmailContext } from './core/egress-policy.js';
import {
  runOutboundOnce,
  type OutboundPollingHandle,
  type OutboundSenderDeps
} from './workers/outbound-sender.js';
import { loadCryptoConfig } from './security/token-crypto.js';
import { loadSessionConfig } from './security/session.js';
import { InMemoryNonceStore, loadOAuthStateConfig } from './security/oauth/state.js';
import { loadProviderConfig } from './security/oauth/providers/index.js';
import type { FomoConfig, RequestContext } from './types.js';

interface FomoRuntime {
  config: FomoConfig;
  server: http.Server;
  startedAtMs: number;
  storesHandle: SubstrateStoresHandle;
  close(): Promise<void>;
}

interface PollingHandle {
  stop(): Promise<void>;
}

function getHeader(req: http.IncomingMessage, name: string): string | undefined {
  const value = req.headers[name.toLowerCase()];
  if (typeof value === 'string') {
    return value;
  }
  if (Array.isArray(value) && value.length > 0) {
    return value[0];
  }
  return undefined;
}

function requestContext(req: http.IncomingMessage): RequestContext {
  return {
    traceId: getHeader(req, 'x-trace-id') ?? randomUUID(),
    spanId: getHeader(req, 'x-span-id') ?? randomUUID(),
    requestId: getHeader(req, 'x-request-id') ?? randomUUID(),
    userId: getHeader(req, 'x-user-id')
  };
}

function logEvent(
  config: FomoConfig,
  ctx: RequestContext | undefined,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: config.serviceName,
      env: config.environment,
      trace_id: ctx?.traceId,
      span_id: ctx?.spanId,
      request_id: ctx?.requestId,
      user_id: ctx?.userId,
      event,
      severity,
      attrs
    }) + '\n'
  );
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

// Build OAuth route deps from env. Returns null if Google OAuth is not
// configured (GOOGLE_CLIENT_ID / SECRET / REDIRECT_URI env vars missing).
// In that case the server still boots and /health works; OAuth routes
// just don't exist. Production deploys should have all three set.
function buildOAuthGoogleDeps(
  storesHandle: SubstrateStoresHandle,
  config: FomoConfig,
  gmailClient: GmailClient
): OAuthGoogleRouteDeps | null {
  const providerConfig = loadProviderConfig('google');
  if (!providerConfig) {
    logEvent(config, undefined, 'fomo.oauth.google.not_configured', 'WARN', {
      detail: 'GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / BREVIO_OAUTH_REDIRECT_URI_GOOGLE not set — /oauth/google/* routes disabled'
    });
    return null;
  }
  return {
    providerConfig,
    stateConfig: loadOAuthStateConfig(),
    nonceStore: new InMemoryNonceStore(),
    tokenStore: storesHandle.stores.tokens,
    gmailCursorStore: storesHandle.stores.gmailCursors,
    gmailClient,
    sessionConfig: loadSessionConfig()
  };
}

/* ---------------------------------------------------------------------- */
/* Polling bootstrap (Phase 3B.2)                                         */
/* ---------------------------------------------------------------------- */

// hasConsent / hasOAuth are sync callbacks per the Permission Gate API.
// The polling worker iterates users per cycle, but the gate evaluations
// happen synchronously inside runOnce. We refresh a token-state snapshot
// once at the top of each cycle and the callbacks read from it.
//
// Identity: for v0.1, completing OAuth IS consent for gmail.read — the
// founder explicitly granted by walking through /oauth/google/start.
// Future phases that introduce a separate consent surface (e.g., per-tool
// consent toggles, friend-beta gating) will plug a real ConsentStore in
// here without changing the worker's API.
interface CycleTokenSnapshot {
  readonly has: boolean;
  readonly needsReauth: boolean;
}

function buildLiveGateDeps(
  killSwitches: KillSwitches,
  snapshot: Map<string, CycleTokenSnapshot>
): PolicyGateDeps {
  return {
    registry: createToolRegistry(),
    switches: killSwitches,
    hasConsent: (userId: string): boolean => snapshot.get(userId)?.has ?? false,
    hasOAuth: (userId: string, provider: string): boolean => {
      if (provider !== 'google') return false;
      const s = snapshot.get(userId);
      return !!s && s.has && !s.needsReauth;
    }
  };
}

/* ---------------------------------------------------------------------- */
/* Ranker bootstrap (Phase 3C.3)                                          */
/* ---------------------------------------------------------------------- */

// Build the ranker dep that the polling worker uses. Returns null when
// the kill switch is off (the safe default). THROWS when the kill
// switch is on but the OpenAI config is incomplete — "real or absent,
// never half-wired": if FOMO_RANKER_ENABLED=true we either deliver a
// working ranker or refuse to boot. Misconfig must surface at startup,
// not silently degrade to a no-op polling loop.
function buildRankerDep(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  env: NodeJS.ProcessEnv = process.env
): GmailPollRankerDep | null {
  if (!killSwitches.ranker_enabled) return null;

  const apiKey = env.OPENAI_API_KEY;
  if (!apiKey || apiKey.length === 0) {
    throw new Error(
      'FOMO_RANKER_ENABLED=true but OPENAI_API_KEY is missing. ' +
        'Set the key or set FOMO_RANKER_ENABLED=false. Refusing to boot a half-wired ranker.'
    );
  }

  const model = env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini';
  const backend = new OpenAIBackend({
    apiKey,
    model,
    responseFormat: RANKER_OPENAI_RESPONSE_FORMAT
  });
  const router = createModelRouter({ costStore: storesHandle.stores.cost });
  router.registerBackend('classification', backend);

  return Object.freeze({
    rank: (req: Parameters<GmailPollRankerDep['rank']>[0]) => rankEmail(req, { router }),
    store: storesHandle.stores.rankResults
  });
}

/* ---------------------------------------------------------------------- */
/* Slack candidate-review bootstrap (Phase 3D.1)                          */
/* ---------------------------------------------------------------------- */

// Builds the slackReview dep + the SlackClient that the
// slack.founder_review executor will use, AND the slack-interactivity
// route deps for the Phase 3D.2 approval-capture inbound webhook.
//
// Returns null fields when FOMO_SLACK_REVIEW_ENABLED is off (the safe
// default) — the polling worker skips the Slack flow entirely AND the
// /slack/interactivity route is NOT mounted on the HTTP server.
//
// THROWS at boot when the switch is on but any required Slack env var
// is missing — "real or absent, never half-wired":
//   * SLACK_BOT_TOKEN (xoxb-...) for outbound chat.postMessage + chat.update
//   * SLACK_FOUNDER_CHANNEL_ID (C0123...) — the channel cards land in,
//     AND the only channel approve/reject is accepted from
//   * SLACK_SIGNING_SECRET — the app's signing secret used to verify
//     inbound /slack/interactivity requests (Phase 3D.2)
//   * SLACK_FOUNDER_USER_ID (U0123...) is OPTIONAL but recommended;
//     when set, only that user's clicks are accepted (the route logs a
//     'best-effort' note when unset).
function buildSlackReviewWiring(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  env: NodeJS.ProcessEnv = process.env
): {
  dep: GmailPollSlackReviewDep | null;
  client: SlackClient | null;
  routeDeps: SlackInteractivityRouteDeps | null;
} {
  if (!killSwitches.slack_review_enabled) {
    return { dep: null, client: null, routeDeps: null };
  }

  const botToken = env.SLACK_BOT_TOKEN;
  const channelId = env.SLACK_FOUNDER_CHANNEL_ID;
  const signingSecret = env.SLACK_SIGNING_SECRET;
  const founderUserId = env.SLACK_FOUNDER_USER_ID?.trim() || undefined;

  if (!botToken || botToken.length === 0) {
    throw new Error(
      'FOMO_SLACK_REVIEW_ENABLED=true but SLACK_BOT_TOKEN is missing. ' +
        'Set the bot token (xoxb-...) or set FOMO_SLACK_REVIEW_ENABLED=false. ' +
        'Refusing to boot a half-wired Slack review path.'
    );
  }
  if (!channelId || channelId.length === 0) {
    throw new Error(
      'FOMO_SLACK_REVIEW_ENABLED=true but SLACK_FOUNDER_CHANNEL_ID is missing. ' +
        'Set the channel id (e.g. C0123...) or set FOMO_SLACK_REVIEW_ENABLED=false.'
    );
  }
  // Phase 3D.2: signing secret is REQUIRED to verify inbound
  // interactivity requests. We refuse to boot with the switch on but
  // no signing secret — the route would accept anything otherwise.
  if (!signingSecret || signingSecret.length === 0) {
    throw new Error(
      'FOMO_SLACK_REVIEW_ENABLED=true but SLACK_SIGNING_SECRET is missing. ' +
        "Set the app's signing secret (from Slack app Basic Information panel) " +
        'or set FOMO_SLACK_REVIEW_ENABLED=false. Refusing to boot an unsigned ' +
        'inbound Slack interactivity endpoint.'
    );
  }

  // SlackClient constructor throws synchronously on bad token/channel
  // shape — surfaced here at boot rather than at first dispatch.
  const client = new SlackClient({ botToken, channelId });

  // Phase 3D.2: the approval-capture route needs to recover the raw
  // email view to rebuild the resolution card. Since gmail.read goes
  // through dispatch with a session/user context, and the inbound
  // route has no such context, we cannot re-call gmail.read here.
  //
  // For 3D.2, resolveEmailContext is undefined — the route will skip
  // chat.update and just transition state + audit. Visual feedback
  // requires a later phase that re-fetches the redacted view from
  // some persisted source. The state transition is the load-bearing
  // outcome; the card update is purely cosmetic.
  //
  // A future enhancement: persist the egress-redacted SlackEgressView
  // on the alerts row at post time, then resolve from there. Not in
  // 3D.2 scope.
  const resolveEmailContext: ((u: string, m: string) => Promise<RawEmailContext | null>) | undefined =
    undefined;

  const routeDeps: SlackInteractivityRouteDeps = Object.freeze({
    signingSecret,
    founderChannelId: channelId,
    founderUserId,
    killSwitches,
    alertStore: storesHandle.stores.alerts,
    rankResultStore: storesHandle.stores.rankResults,
    transitions: storesHandle.stores.transitions,
    feedbackStore: storesHandle.stores.feedback,
    auditStore: storesHandle.stores.audit,
    slackClient: client,
    resolveEmailContext
  });

  return {
    dep: Object.freeze({
      alertStore: storesHandle.stores.alerts,
      transitions: storesHandle.stores.transitions
    }),
    client,
    routeDeps
  };
}

/* ---------------------------------------------------------------------- */
/* SendBlue outbound-sender bootstrap (Phase 3E.1)                        */
/* ---------------------------------------------------------------------- */

// Builds the SendBlueClient + the outbound-sender dep factory. Returns
// `{ client: null, runDeps: null }` when FOMO_SEND_ENABLED is off (the
// safe default) — the outbound worker is never started AND the
// sendblue.send_user_message executor's dispatched calls return
// executor_error('SendBlue adapter not wired') as defense-in-depth.
//
// THROWS at boot when FOMO_SEND_ENABLED=true but any required env var
// is missing — "real or absent, never half-wired":
//   * SENDBLUE_API_KEY_ID
//   * SENDBLUE_API_SECRET_KEY
//   * SENDBLUE_FROM_NUMBER (E.164) — the SendBlue-assigned sender
//     phone, REQUIRED in every send-message POST body. 3E.2 smoke
//     test surfaced that SendBlue returns HTTP 400
//     `missing required parameter: "from_number"` without it.
//   * FOMO_FOUNDER_PHONE_NUMBER (E.164) — the destination phone
//     allowlist (where iMessages are delivered TO).
//   * FOMO_FOUNDER_USER_ID — the user_id whose alerts the worker is
//     allowed to text. Defense-in-depth allowlist: destinationFor
//     returns FOMO_FOUNDER_PHONE_NUMBER ONLY for this user_id and
//     null for anyone else, so a future multi-user environment cannot
//     accidentally text strangers.
//
// The returned `runDeps(gateDeps)` factory accepts a fresh gateDeps
// per cycle (mirroring the gmail-poll snapshot pattern) so token
// state stays current.
function buildSendWiring(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  dispatch: DispatchTable,
  env: NodeJS.ProcessEnv = process.env
): {
  client: SendBlueClient | null;
  runDeps: ((gateDeps: PolicyGateDeps) => OutboundSenderDeps) | null;
  founderUserId: string | null;
} {
  if (!killSwitches.send_enabled) {
    return { client: null, runDeps: null, founderUserId: null };
  }

  const apiKeyId = env.SENDBLUE_API_KEY_ID;
  const apiSecretKey = env.SENDBLUE_API_SECRET_KEY;
  const fromNumber = env.SENDBLUE_FROM_NUMBER?.trim();
  const founderPhone = env.FOMO_FOUNDER_PHONE_NUMBER?.trim();
  const founderUserId = env.FOMO_FOUNDER_USER_ID?.trim();

  if (!apiKeyId || apiKeyId.length === 0) {
    throw new Error(
      'FOMO_SEND_ENABLED=true but SENDBLUE_API_KEY_ID is missing. ' +
        'Set the SendBlue API key id (from your SendBlue dashboard) or set ' +
        'FOMO_SEND_ENABLED=false. Refusing to boot a half-wired send path.'
    );
  }
  if (!apiSecretKey || apiSecretKey.length === 0) {
    throw new Error(
      'FOMO_SEND_ENABLED=true but SENDBLUE_API_SECRET_KEY is missing. ' +
        'Set the SendBlue API secret key or set FOMO_SEND_ENABLED=false.'
    );
  }
  if (!fromNumber || fromNumber.length === 0) {
    throw new Error(
      'FOMO_SEND_ENABLED=true but SENDBLUE_FROM_NUMBER is missing. ' +
        'Set the SendBlue-assigned sender phone (E.164, e.g. +12143547196) or set ' +
        'FOMO_SEND_ENABLED=false. SendBlue rejects /api/send-message with HTTP 400 ' +
        '`missing required parameter: "from_number"` without this — surfaced by the ' +
        '3E.2 smoke test.'
    );
  }
  if (!/^\+\d{7,15}$/.test(fromNumber)) {
    throw new Error(
      `FOMO_SEND_ENABLED=true but SENDBLUE_FROM_NUMBER is not in E.164 format ` +
        `(got '${fromNumber.slice(0, 4)}...'). Expected '+' followed by 7-15 digits.`
    );
  }
  if (!founderPhone || founderPhone.length === 0) {
    throw new Error(
      'FOMO_SEND_ENABLED=true but FOMO_FOUNDER_PHONE_NUMBER is missing. ' +
        'Set the founder phone number (E.164, e.g. +14155551234) or set ' +
        'FOMO_SEND_ENABLED=false. The outbound-sender worker refuses to ' +
        'dispatch without a founder-allowlisted destination.'
    );
  }
  // Soft validation: E.164 numbers start with + and contain only digits.
  if (!/^\+\d{7,15}$/.test(founderPhone)) {
    throw new Error(
      `FOMO_SEND_ENABLED=true but FOMO_FOUNDER_PHONE_NUMBER is not in E.164 format ` +
        `(got '${founderPhone.slice(0, 4)}...'). Expected '+' followed by 7-15 digits, ` +
        `e.g. '+14155551234'. Refusing to boot.`
    );
  }
  if (!founderUserId || founderUserId.length === 0) {
    throw new Error(
      'FOMO_SEND_ENABLED=true but FOMO_FOUNDER_USER_ID is missing. ' +
        'The outbound-sender worker uses this id to enforce the ' +
        'founder-phone allowlist (destinationFor returns the founder ' +
        'phone ONLY for this user_id). Set it or set FOMO_SEND_ENABLED=false.'
    );
  }

  const client = new SendBlueClient({ apiKeyId, apiSecretKey, fromNumber });

  const runDeps = (gateDeps: PolicyGateDeps): OutboundSenderDeps =>
    Object.freeze({
      dispatch,
      auditStore: storesHandle.stores.audit,
      toolInvocationStore: storesHandle.stores.toolInvocations,
      gateDeps,
      cursorStore: storesHandle.stores.gmailCursors,
      alertStore: storesHandle.stores.alerts,
      rankResultStore: storesHandle.stores.rankResults,
      transitions: storesHandle.stores.transitions,
      // Phase 3F.1 — outbound-sender consults this BEFORE every
      // cycle's send to honor STOP. Per founder directive 2026-05-26.
      memoryStore: storesHandle.stores.memory,
      destinationFor: (uid: string): string | null =>
        uid === founderUserId ? founderPhone : null
    });

  return { client, runDeps, founderUserId };
}

function startGmailPolling(
  storesHandle: SubstrateStoresHandle,
  gmailClient: GmailClient,
  dispatch: DispatchTable,
  killSwitches: KillSwitches,
  config: FomoConfig,
  ranker: GmailPollRankerDep | null,
  slackReview: GmailPollSlackReviewDep | null
): PollingHandle {
  const stores = storesHandle.stores;
  let stopped = false;
  let inflight: Promise<void> = Promise.resolve();
  let timer: ReturnType<typeof setTimeout> | null = null;
  let cyclesRun = 0;
  const cap = killSwitches.polling_max_cycles; // null = unbounded

  const tick = (): void => {
    if (stopped) return;
    inflight = (async () => {
      try {
        const userIds = await stores.gmailCursors.listUserIds();
        const snapshot = new Map<string, CycleTokenSnapshot>();
        for (const uid of userIds) {
          const tokens = await stores.tokens.list(uid);
          const google = tokens.find((t) => t.provider === 'google');
          snapshot.set(uid, {
            has: !!google,
            needsReauth: google?.needs_reauth ?? false
          });
        }
        const gateDeps = buildLiveGateDeps(killSwitches, snapshot);
        const report = await runPollOnce({
          gmailClient,
          tokenStore: stores.tokens,
          cursorStore: stores.gmailCursors,
          dispatch,
          auditStore: stores.audit,
          toolInvocationStore: stores.toolInvocations,
          gateDeps,
          ranker: ranker ?? undefined,
          slackReview: slackReview ?? undefined
        });
        cyclesRun++;
        if (stopped) return;
        logEvent(config, undefined, 'fomo.poll.cycle', 'INFO', {
          cycle_number: cyclesRun,
          cycle_cap: cap,
          users_total: report.users_total,
          users_polled: report.users_polled,
          users_skipped: report.users_skipped,
          // Phase 3G.1 item #3 — surface needs_reauth count distinctly
          // from the generic users_skipped bucket so operators see
          // silent OAuth drift without parsing per-user outcomes.
          users_needs_reauth: report.users_needs_reauth,
          users_unauthorized: report.users_unauthorized,
          users_api_error: report.users_api_error,
          messages_observed: report.messages_observed,
          messages_dispatched: report.messages_dispatched,
          messages_failed: report.messages_failed,
          // Phase 3C.3: only meaningful when ranker dep was built; zero
          // when ranker_enabled=false. Visible in the same log line so
          // ops can confirm the ranker is firing without correlating
          // audit rows.
          messages_ranked: report.messages_ranked,
          messages_rank_already: report.messages_rank_already,
          messages_rank_failed: report.messages_rank_failed,
          // Phase 3D.1: only meaningful when slackReview dep was built;
          // zero when slack_review_enabled=false.
          alerts_created: report.alerts_created,
          slack_posts: report.slack_posts,
          slack_posts_already: report.slack_posts_already,
          slack_posts_failed: report.slack_posts_failed
        });
      } catch (err) {
        if (stopped) return;
        logEvent(config, undefined, 'fomo.poll.error', 'ERROR', {
          error: err instanceof Error ? err.message : String(err)
        });
      }
    })();
    void inflight.finally(() => {
      if (stopped) return;
      // Phase 3B.3: bounded smoke test. When FOMO_GMAIL_POLLING_MAX_CYCLES
      // is set, auto-stop after that many cycles and emit one terminal
      // log event so ops can confirm the cap fired.
      if (cap !== null && cyclesRun >= cap) {
        stopped = true;
        logEvent(config, undefined, 'fomo.poll.cycle_cap_reached', 'INFO', {
          cycles_run: cyclesRun,
          cycle_cap: cap
        });
        return;
      }
      timer = setTimeout(tick, killSwitches.polling_interval_ms);
    });
  };

  tick();

  return {
    async stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      await inflight.catch(() => undefined);
    }
  };
}

/* ---------------------------------------------------------------------- */
/* SendBlue inbound reply bootstrap (Phase 3F.1)                          */
/* ---------------------------------------------------------------------- */

// Builds the /sendblue/inbound route deps. Returns null when
// FOMO_SENDBLUE_INBOUND_ENABLED is off (the safe default) — the route
// is NOT mounted on the HTTP server.
//
// THROWS at boot when the switch is on but any required env var is
// missing — "real or absent, never half-wired":
//   * SENDBLUE_WEBHOOK_SECRET — the secret you configured in the
//     SendBlue dashboard. SendBlue echoes this verbatim in a
//     request header on every inbound webhook POST (per docs at
//     docs.sendblue.com/getting-started/webhooks). NOT an HMAC
//     signing key — it's a plain shared secret compared with
//     timing-safe equality against the header value.
//   * FOMO_FOUNDER_PHONE_NUMBER (already required by buildSendWiring
//     when FOMO_SEND_ENABLED=true; re-validated here for the inbound
//     from-number allowlist)
//   * FOMO_FOUNDER_USER_ID (same as above; required for attribution)
//   * OPENAI_API_KEY — the reply parser's classifier path uses OpenAI
//
// Optional env:
//   * SENDBLUE_WEBHOOK_SECRET_HEADER — overrides the default
//     header name `sb-signing-secret`. SendBlue's public docs don't
//     name the header explicitly; the founder confirms / overrides
//     during 3F.2 smoke after observing a real SendBlue request.
//
// The reply parser shares the same OpenAI router instance as the
// ranker (3C.3 backend) — both are 'classification' capability. The
// classifier response_format is reply-parser-specific (intent +
// confidence + snooze_hint).
function buildSendBlueInboundWiring(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  env: NodeJS.ProcessEnv = process.env
): {
  routeDeps: SendBlueInboundRouteDeps | null;
  inboundEnabled: boolean;
} {
  if (!killSwitches.sendblue_inbound_enabled) {
    return { routeDeps: null, inboundEnabled: false };
  }

  const webhookSecret = env.SENDBLUE_WEBHOOK_SECRET;
  const webhookSecretHeader =
    env.SENDBLUE_WEBHOOK_SECRET_HEADER?.trim().toLowerCase() || 'sb-signing-secret';
  const founderPhone = env.FOMO_FOUNDER_PHONE_NUMBER?.trim();
  const founderUserId = env.FOMO_FOUNDER_USER_ID?.trim();
  const openaiKey = env.OPENAI_API_KEY;

  if (!webhookSecret || webhookSecret.length === 0) {
    throw new Error(
      'FOMO_SENDBLUE_INBOUND_ENABLED=true but SENDBLUE_WEBHOOK_SECRET is missing. ' +
        "Set the webhook secret (the value you configured in your SendBlue " +
        "dashboard's webhook settings — SendBlue echoes it back in a request " +
        'header on every inbound POST per docs.sendblue.com/getting-started/' +
        'webhooks) or set FOMO_SENDBLUE_INBOUND_ENABLED=false. Refusing to ' +
        'boot an unauthenticated /sendblue/inbound endpoint.'
    );
  }
  if (!founderPhone || !/^\+\d{7,15}$/.test(founderPhone)) {
    throw new Error(
      'FOMO_SENDBLUE_INBOUND_ENABLED=true but FOMO_FOUNDER_PHONE_NUMBER is missing/malformed. ' +
        'The inbound route allowlists this phone — without it no reply is acceptable. ' +
        'Set the founder phone (E.164) or set FOMO_SENDBLUE_INBOUND_ENABLED=false.'
    );
  }
  if (!founderUserId || founderUserId.length === 0) {
    throw new Error(
      'FOMO_SENDBLUE_INBOUND_ENABLED=true but FOMO_FOUNDER_USER_ID is missing. ' +
        'Inbound replies are attributed to this user_id. ' +
        'Set it or set FOMO_SENDBLUE_INBOUND_ENABLED=false.'
    );
  }
  if (!openaiKey || openaiKey.length === 0) {
    throw new Error(
      'FOMO_SENDBLUE_INBOUND_ENABLED=true but OPENAI_API_KEY is missing. ' +
        'The reply parser\'s classifier path uses OpenAI to parse soft intents. ' +
        'Set OPENAI_API_KEY or set FOMO_SENDBLUE_INBOUND_ENABLED=false.'
    );
  }

  // Build a dedicated reply-parser router. We could share the ranker's
  // router, but keeping them independent makes per-capability backend
  // swaps (e.g. trying a cheaper model for parsing) trivial without
  // touching ranker behavior.
  const replyBackend = new OpenAIBackend({
    apiKey: openaiKey,
    model: env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini',
    responseFormat: REPLY_PARSER_OPENAI_RESPONSE_FORMAT
  });
  const replyRouter = createModelRouter({ costStore: storesHandle.stores.cost });
  replyRouter.registerBackend('classification', replyBackend);

  const routeDeps: SendBlueInboundRouteDeps = Object.freeze({
    webhookSecret,
    webhookSecretHeader,
    founderPhoneNumber: founderPhone,
    founderUserId,
    killSwitches,
    inboundReplyStore: storesHandle.stores.inboundReplies,
    alertStore: storesHandle.stores.alerts,
    rankResultStore: storesHandle.stores.rankResults,
    transitions: storesHandle.stores.transitions,
    feedbackStore: storesHandle.stores.feedback,
    memoryStore: storesHandle.stores.memory,
    auditStore: storesHandle.stores.audit,
    replyParser: {
      parse: (req: Parameters<typeof parseReply>[0]) => parseReply(req, { router: replyRouter })
    }
  });

  return { routeDeps, inboundEnabled: true };
}

// Mirrors startGmailPolling for the outbound-sender worker: each tick
// refreshes the per-user token snapshot, builds a fresh gateDeps off
// of it, and calls runOutboundOnce(deps). The interval is the same
// FOMO_GMAIL_POLLING_INTERVAL_MS used by the polling worker (we don't
// add a second knob for v0.1 — one worker rhythm to reason about).
function startOutboundSenderInterval(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  config: FomoConfig,
  runDepsBuilder: (gateDeps: PolicyGateDeps) => OutboundSenderDeps
): OutboundPollingHandle {
  const stores = storesHandle.stores;
  let stopped = false;
  let inflight: Promise<void> = Promise.resolve();
  let timer: ReturnType<typeof setTimeout> | null = null;
  let cyclesRun = 0;
  const cap = killSwitches.outbound_max_cycles; // null = unbounded

  const tick = (): void => {
    if (stopped) return;
    inflight = (async () => {
      try {
        const userIds = await stores.gmailCursors.listUserIds();
        const snapshot = new Map<string, CycleTokenSnapshot>();
        for (const uid of userIds) {
          const tokens = await stores.tokens.list(uid);
          const google = tokens.find((t) => t.provider === 'google');
          snapshot.set(uid, {
            has: !!google,
            needsReauth: google?.needs_reauth ?? false
          });
        }
        const gateDeps = buildLiveGateDeps(killSwitches, snapshot);
        const deps = runDepsBuilder(gateDeps);
        const report = await runOutboundOnce(deps);
        cyclesRun++;
        if (stopped) return;
        logEvent(config, undefined, 'fomo.outbound.cycle', 'INFO', {
          cycle_number: cyclesRun,
          cycle_cap: cap,
          users_total: report.users_total,
          users_with_approved_alerts: report.users_with_approved_alerts,
          alerts_considered: report.alerts_considered,
          alerts_sent: report.alerts_sent,
          alerts_failed: report.alerts_failed,
          alerts_status_unknown: report.alerts_status_unknown,
          alerts_unauthorized: report.alerts_unauthorized,
          alerts_preflight_skipped: report.alerts_preflight_skipped
        });
      } catch (err) {
        if (stopped) return;
        logEvent(config, undefined, 'fomo.outbound.error', 'ERROR', {
          error: err instanceof Error ? err.message : String(err)
        });
      }
    })();
    void inflight.finally(() => {
      if (stopped) return;
      // Phase 3E.2: bounded smoke window. When FOMO_OUTBOUND_MAX_CYCLES
      // is set, auto-stop after that many cycles and emit one terminal
      // log event so ops can confirm the cap fired. The 3E.2 founder
      // smoke runbook sets this to a small N (1-3) so the worker
      // cannot accidentally keep firing real iMessages against
      // SendBlue during the smoke window.
      if (cap !== null && cyclesRun >= cap) {
        stopped = true;
        logEvent(config, undefined, 'fomo.outbound.cycle_cap_reached', 'INFO', {
          cycles_run: cyclesRun,
          cycle_cap: cap
        });
        return;
      }
      timer = setTimeout(tick, killSwitches.polling_interval_ms);
    });
  };

  tick();

  return {
    async stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      await inflight.catch(() => undefined);
    }
  };
}

export function createFomoRuntime(config: FomoConfig = loadFomoConfig()): FomoRuntime {
  const startedAtMs = Date.now();

  // Substrate stores — throws in production if BREVIO_TOKEN_KEK missing
  // and BREVIO_DEV_MODE is not 'true'. Same fail-closed behavior as the
  // Phase 2E client.
  const cryptoConfig = loadCryptoConfig();
  const storesHandle = createStores({ env: process.env, crypto: cryptoConfig });

  // Kill switches — read once at boot. Per FOMO_PLAN §16.5, defaults are
  // safe (everything off). FOMO_GMAIL_POLLING_ENABLED controls whether
  // the polling worker installs its interval.
  const killSwitches = loadKillSwitches(process.env);

  // Shared GmailClient — used by both the OAuth callback (to seed the
  // cursor at connect time) and the polling worker (to drive
  // listHistorySince + getMessage every cycle).
  const gmailClient = new GmailClient();

  // Dispatch table + executor wireup. Always wired regardless of polling
  // flag: an admin endpoint or ad-hoc caller could still invoke
  // gmail.read via dispatch when polling is off. The gate still gates
  // on consent + OAuth.
  const dispatch = createDispatchTable();
  wireInternalExecutors(dispatch, {
    audit: storesHandle.stores.audit,
    feedback: storesHandle.stores.feedback,
    memory: storesHandle.stores.memory
  });

  // Ranker — bootstrapped only when FOMO_RANKER_ENABLED=true. THROWS at
  // boot if the kill switch is on but OpenAI config is incomplete; safe
  // default (kill switch off) returns null and the polling worker
  // behaves exactly as in 3B.2/3B.3.
  const ranker = buildRankerDep(storesHandle, killSwitches);
  if (ranker) {
    logEvent(config, undefined, 'fomo.ranker.enabled', 'INFO', {
      model: process.env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini',
      prompt_version_loaded: true
    });
  } else {
    logEvent(config, undefined, 'fomo.ranker.disabled', 'INFO', {
      detail: 'FOMO_RANKER_ENABLED is not "true"; ranker dormant (rank_results stays empty)'
    });
  }

  // Slack candidate review — bootstrapped only when FOMO_SLACK_REVIEW_ENABLED=true.
  // THROWS at boot if the switch is on but SLACK_BOT_TOKEN or
  // SLACK_FOUNDER_CHANNEL_ID is missing. Safe default (switch off)
  // returns { dep: null, client: null } and the polling worker behaves
  // exactly as in 3C.3 (no alerts, no Slack calls).
  //
  // The SlackClient is wired through dispatch (slack.founder_review
  // executor uses it); the slackReview dep holds the alert persistence
  // + state transitions.
  const slackWiring = buildSlackReviewWiring(storesHandle, killSwitches);
  if (slackWiring.dep) {
    logEvent(config, undefined, 'fomo.slack.review.enabled', 'INFO', {
      channel_id: process.env.SLACK_FOUNDER_CHANNEL_ID,
      // Phase 3D.2: surface that the inbound route is mounted AND
      // whether the founder-user restriction is active. If
      // SLACK_FOUNDER_USER_ID is unset, the route accepts any user's
      // clicks from the founder channel (best-effort; runbook warns).
      interactivity_route_mounted: slackWiring.routeDeps !== null,
      founder_user_restricted: !!slackWiring.routeDeps?.founderUserId
    });
  } else {
    logEvent(config, undefined, 'fomo.slack.review.disabled', 'INFO', {
      detail:
        'FOMO_SLACK_REVIEW_ENABLED is not "true"; Slack candidate-review path dormant ' +
        '(alerts table stays empty; /slack/interactivity route NOT mounted)'
    });
  }

  // SendBlue outbound send — bootstrapped only when FOMO_SEND_ENABLED=true.
  // THROWS at boot if the switch is on but creds / founder phone /
  // founder user id are missing. Safe default (switch off) returns
  // null fields and the outbound worker is never started; the
  // sendblue.send_user_message executor is still registered (so the
  // gate's send_disabled denial path is exercisable) but every call
  // surfaces executor_error('SendBlue adapter not wired').
  const sendWiring = buildSendWiring(storesHandle, killSwitches, dispatch);
  if (sendWiring.client && sendWiring.runDeps) {
    logEvent(config, undefined, 'fomo.send.enabled', 'INFO', {
      founder_user_id: sendWiring.founderUserId,
      // Never log the full phone — only that it is configured.
      founder_phone_configured: true,
      auto_send_enabled: killSwitches.auto_send_enabled
    });
  } else {
    logEvent(config, undefined, 'fomo.send.disabled', 'INFO', {
      detail:
        'FOMO_SEND_ENABLED is not "true"; outbound sender worker dormant ' +
        '(no SendBlue calls, no approved → sent transitions)'
    });
  }

  wireExternalExecutors(dispatch, {
    gmailClient,
    tokenStore: storesHandle.stores.tokens,
    slackClient: slackWiring.client ?? undefined,
    sendBlueClient: sendWiring.client ?? undefined
  });

  // SendBlue inbound reply route (Phase 3F.1) — mounted only when
  // FOMO_SENDBLUE_INBOUND_ENABLED=true. THROWS at boot if the switch
  // is on but the webhook signing secret / founder phone / founder
  // user_id / OpenAI key are missing. Safe default: route NOT mounted,
  // returns 404, no reply parsing happens.
  const inboundWiring = buildSendBlueInboundWiring(storesHandle, killSwitches);
  if (inboundWiring.routeDeps) {
    logEvent(config, undefined, 'fomo.sendblue.inbound.enabled', 'INFO', {
      inbound_route_mounted: true,
      founder_user_id: process.env.FOMO_FOUNDER_USER_ID,
      // Surface the configured header name so the founder can see
      // exactly which header the route reads — useful during 3F.2
      // smoke when confirming SendBlue's actual header name.
      webhook_secret_header: inboundWiring.routeDeps.webhookSecretHeader
    });
  } else {
    logEvent(config, undefined, 'fomo.sendblue.inbound.disabled', 'INFO', {
      detail:
        'FOMO_SENDBLUE_INBOUND_ENABLED is not "true"; /sendblue/inbound route NOT mounted ' +
        '(no reply parsing, no STOP enforcement via inbound webhook)'
    });
  }

  // OAuth routes — graceful skip when not configured.
  const oauthGoogleDeps = buildOAuthGoogleDeps(storesHandle, config, gmailClient);

  // Polling worker — bootstrapped only when FOMO_GMAIL_POLLING_ENABLED=true.
  // Safe default: off (no autonomous Gmail reads until founder opts in).
  let pollingHandle: PollingHandle | null = null;
  if (killSwitches.polling_enabled) {
    pollingHandle = startGmailPolling(
      storesHandle,
      gmailClient,
      dispatch,
      killSwitches,
      config,
      ranker,
      slackWiring.dep
    );
    logEvent(config, undefined, 'fomo.poll.enabled', 'INFO', {
      interval_ms: killSwitches.polling_interval_ms,
      cycle_cap: killSwitches.polling_max_cycles,
      ranker_enabled: ranker !== null,
      slack_review_enabled: slackWiring.dep !== null
    });

    // Phase 3G.1 item #3 — needs_reauth visibility at boot.
    //
    // Real incident (2026-05-28): the polling worker silently skipped
    // every cycle for 18+ hours because `oauth_tokens.needs_reauth=true`
    // for the founder user. The fact was buried in per-user outcomes
    // and `users_skipped` count. An operator only discovered it via a
    // manual psql query.
    //
    // Fix: at boot, walk the SAME active-user set the worker iterates
    // (cursorStore.listUserIds()), check each user's token, and log
    // WARN per user with needs_reauth=true. Founder directive
    // 2026-05-29: must use the same active-user set the polling
    // worker uses (not a broader oauth_tokens scan).
    void (async (): Promise<void> => {
      try {
        const findings = await findUsersNeedingReauth({
          cursorStore: storesHandle.stores.gmailCursors,
          tokenStore: storesHandle.stores.tokens
        });
        for (const f of findings) {
          logEvent(config, undefined, 'fomo.poll.needs_reauth_at_boot', 'WARN', {
            user_id: f.user_id,
            provider: f.provider,
            detail:
              'oauth_tokens.needs_reauth=true; polling will skip this user every cycle. ' +
              'Run the founder OAuth re-auth flow (smoke-test-3b3-gmail.md §7) before this user can be polled.'
          });
        }
      } catch (err) {
        // Best-effort visibility surface. We do NOT block boot on a
        // failure here; the polling worker still runs and surfaces
        // any cycle-time issues via fomo.poll.cycle / fomo.poll.error.
        logEvent(config, undefined, 'fomo.poll.needs_reauth_check_failed', 'WARN', {
          error: err instanceof Error ? err.message : String(err)
        });
      }
    })();
  } else {
    logEvent(config, undefined, 'fomo.poll.disabled', 'INFO', {
      detail: 'FOMO_GMAIL_POLLING_ENABLED is not "true"; polling worker dormant'
    });
  }

  // Outbound-sender worker (Phase 3E.1) — runs independently of the
  // Gmail polling worker. It uses the same per-cycle token snapshot
  // pattern to keep gateDeps current, but iterates the alerts table
  // (not Gmail history) and dispatches sendblue.send_user_message for
  // any alert whose latest state is 'approved'. Safe default: off.
  let outboundHandle: OutboundPollingHandle | null = null;
  if (killSwitches.send_enabled && sendWiring.runDeps) {
    outboundHandle = startOutboundSenderInterval(
      storesHandle,
      killSwitches,
      config,
      sendWiring.runDeps
    );
    logEvent(config, undefined, 'fomo.outbound.enabled', 'INFO', {
      interval_ms: killSwitches.polling_interval_ms,
      cycle_cap: killSwitches.outbound_max_cycles,
      auto_send_enabled: killSwitches.auto_send_enabled
    });
  } else {
    logEvent(config, undefined, 'fomo.outbound.disabled', 'INFO', {
      detail:
        'FOMO_SEND_ENABLED is not "true"; outbound-sender worker dormant ' +
        '(approved alerts will not become sent)'
    });
  }

  const server = http.createServer((req, res) => {
    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const path = (req.url ?? '/').split('?')[0] ?? '/';

    if (method === 'GET' && path === '/health') {
      handleHealth(res, config, startedAtMs);
      return;
    }

    // Each route handler returns null when the request does not match
    // its routes. We try them in order; the FIRST handler that returns
    // a non-null response handles the request.
    void (async () => {
      try {
        // Slack interactivity (Phase 3D.2) — only when wired.
        if (slackWiring.routeDeps) {
          const slackResp = await tryHandleSlackInteractivityRequest(req, slackWiring.routeDeps);
          if (slackResp) {
            res.writeHead(slackResp.status, slackResp.headers);
            res.end(slackResp.body);
            return;
          }
        }

        // SendBlue inbound reply (Phase 3F.1) — only when wired.
        if (inboundWiring.routeDeps) {
          const inboundResp = await tryHandleSendBlueInboundRequest(req, inboundWiring.routeDeps);
          if (inboundResp) {
            res.writeHead(inboundResp.status, inboundResp.headers);
            res.end(inboundResp.body);
            return;
          }
        }

        // OAuth routes — only when wired.
        if (oauthGoogleDeps) {
          const oauthResp = await tryHandleOAuthGoogleRequest(req, oauthGoogleDeps);
          if (oauthResp) {
            res.writeHead(oauthResp.status, oauthResp.headers);
            res.end(oauthResp.body);
            return;
          }
        }

        sendJSON(res, 404, { error: 'not_found', request_id: ctx.requestId });
      } catch (err) {
        logEvent(config, ctx, 'fomo.http.unhandled', 'ERROR', {
          error: err instanceof Error ? err.message : String(err)
        });
        if (!res.headersSent) {
          sendJSON(res, 500, { error: 'internal', request_id: ctx.requestId });
        }
      }
    })();
  });

  const runtime: FomoRuntime = {
    config,
    server,
    startedAtMs,
    storesHandle,
    close: async () => {
      if (pollingHandle) {
        await pollingHandle.stop();
      }
      if (outboundHandle) {
        await outboundHandle.stop();
      }
      await new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
      });
      if (storesHandle.db?.ok) {
        await storesHandle.db.pool.end();
      }
    }
  };

  server.on('listening', () => {
    logEvent(config, undefined, 'fomo.server.listening', 'INFO', {
      port: config.port,
      store_backend: storesHandle.backend,
      oauth_google_wired: oauthGoogleDeps !== null,
      polling_enabled: killSwitches.polling_enabled,
      ranker_enabled: ranker !== null,
      slack_review_enabled: slackWiring.dep !== null,
      slack_interactivity_route_mounted: slackWiring.routeDeps !== null,
      send_enabled: sendWiring.runDeps !== null,
      outbound_worker_started: outboundHandle !== null,
      sendblue_inbound_route_mounted: inboundWiring.routeDeps !== null
    });

    // Phase 3G.1 item #10 — memory_signals snapshot for the founder
    // at boot. Surfaces every active signal (≥ 0.5 confidence)
    // with kind, age, source, confidence, and a single named-safe
    // active_flag boolean. NEVER logs the raw detail body.
    //
    // Real incident (2026-05-29 01:00): stop_active=true from
    // 2026-05-28 survived into the next day silently. This snapshot
    // makes that state loud at boot.
    const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
    if (founderUserId) {
      void (async (): Promise<void> => {
        try {
          const snap = await snapshotMemorySignalsForBoot(founderUserId, {
            memoryStore: storesHandle.stores.memory
          });
          logEvent(config, undefined, 'fomo.memory_signals.snapshot_at_boot', 'INFO', {
            user_id: founderUserId,
            signal_count: snap.length,
            signals: snap.map((s) => ({
              kind: s.kind,
              scope_key: s.scope_key,
              age_seconds: s.age_seconds,
              source: s.source,
              confidence: s.confidence,
              active_flag: s.active_flag
            }))
          });
        } catch (err) {
          logEvent(config, undefined, 'fomo.memory_signals.snapshot_failed', 'WARN', {
            error: err instanceof Error ? err.message : String(err)
          });
        }
      })();
    }
  });

  return runtime;
}

export async function main(): Promise<void> {
  // Phase 3G.1 item #1 — Neon migration verification at boot.
  //
  // Real incident (2026-05-28 01:06 UTC): the first 3 POSTs to
  // /sendblue/inbound during the 3F.2 smoke run returned HTTP 500
  // because migration 0004_inbound_replies had been applied via
  // PGlite in gated tests but not against the live Neon DB. Same
  // shape hit 3D.2 with the `alerts` table.
  //
  // Policy (founder-locked 2026-05-29): fail-loud everywhere. No
  // auto-apply. No env bypass. Refuse to boot when any required
  // table is missing, naming each one.
  //
  // In-memory dev mode (DATABASE_URL unset) skips this check — the
  // store-factory's in-memory bundle has no schema to verify.
  if ((process.env.DATABASE_URL ?? '').trim()) {
    const verifyConfig = loadFomoConfig();
    const dbResult = loadDbClient({ env: process.env });
    if (!dbResult.ok) {
      logEvent(verifyConfig, undefined, 'fomo.migrations.db_unavailable', 'ERROR', {
        reason: dbResult.reason
      });
      process.exit(1);
    }
    try {
      await verifyMigrationsOrThrow(dbResult.client);
      logEvent(verifyConfig, undefined, 'fomo.migrations.verified', 'INFO', {
        backend: 'postgres'
      });
    } catch (err) {
      if (err instanceof PendingMigrationsError) {
        logEvent(verifyConfig, undefined, 'fomo.migrations.pending', 'ERROR', {
          missing_count: err.missing_tables.length,
          missing: err.missing_tables.map((m) => ({ table: m.name, migration: m.migration }))
        });
        // Print the named list verbatim to stderr so a human reading
        // the terminal sees it without parsing JSON.
        process.stderr.write(err.message + '\n');
      } else {
        logEvent(verifyConfig, undefined, 'fomo.migrations.verifier_error', 'ERROR', {
          error: err instanceof Error ? err.message : String(err)
        });
      }
      await dbResult.pool.end();
      process.exit(1);
    }
    await dbResult.pool.end();
  }

  const runtime = createFomoRuntime();
  runtime.server.listen(runtime.config.port);

  const shutdown = async (signal: string): Promise<void> => {
    logEvent(runtime.config, undefined, 'fomo.server.shutting_down', 'INFO', { signal });
    try {
      await runtime.close();
    } catch (err) {
      logEvent(runtime.config, undefined, 'fomo.server.shutdown_error', 'ERROR', {
        error: err instanceof Error ? err.message : String(err)
      });
      process.exitCode = 1;
    }
  };

  process.once('SIGTERM', () => {
    void shutdown('SIGTERM');
  });
  process.once('SIGINT', () => {
    void shutdown('SIGINT');
  });
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  void main();
}
