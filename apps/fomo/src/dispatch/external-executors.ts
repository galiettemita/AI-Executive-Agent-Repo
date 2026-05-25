// External-capability executors — Phase 3B.2 wiring for gmail.read,
// Phase 3D.1 adds slack.founder_review.
//
// Where internal-executors.ts wraps in-process substrate stores,
// external-executors.ts wraps real external adapters (Gmail HTTP API in
// Phase 3B.2; Slack chat.postMessage in 3D.1; SendBlue HTTP in 3E).
//
// gmail.read shape: { message_id } → RawEmailContext for that one
// message. Per-message granularity gives every read its own gate
// decision + audit entry, and matches natural-language requests like
// "what did Sarah email me yesterday?" that future flows will reuse.
// The polling worker dispatches once per new message_id discovered via
// GmailClient.listHistorySince (a non-tool metadata operation).
//
// 401 handling: when Gmail rejects the access token, the executor
// marks the token row needs_reauth (via TokenStore.markNeedsReauth) and
// re-throws. dispatch.execute() converts the throw into
// ok:false / code:'executor_error'. The caller (polling worker)
// recognizes the throw and skips this user for the remainder of the
// cycle. No refresh-token flow in 3B.2 — fail-closed skip.

import { GmailClient, GmailUnauthorizedError } from '../adapters/gmail/client.js';
import {
  SlackApiError,
  SlackAuthError,
  type SlackClient,
  type SlackPostInput,
  type SlackPostResult
} from '../adapters/slack/client.js';
import { type RawEmailContext } from '../core/egress-policy.js';
import { type TokenStore } from '../security/oauth/token-store.js';

import { type DispatchTable, type Executor } from './dispatcher.js';

/* ---------------------------------------------------------------------- */
/* gmail.read                                                             */
/* ---------------------------------------------------------------------- */

export interface GmailReadArgs {
  // Gmail message ID (opaque, from GmailHistoryListResult.added_message_ids
  // or a /users/me/messages list).
  readonly message_id: string;
}

// Error thrown when the user has no OAuth token (or the row is marked
// needs_reauth). Bubbles up as executor_error from dispatch; the
// polling worker uses the message prefix to recognize this case.
export class GmailReadTokenMissingError extends Error {
  constructor(user_id: string) {
    super(`gmail.read: no usable Google OAuth token for user ${user_id} (needs_reauth or absent)`);
    this.name = 'GmailReadTokenMissingError';
  }
}

export interface GmailReadExecutorDeps {
  readonly client: GmailClient;
  readonly tokenStore: TokenStore;
}

export function gmailReadExecutor(deps: GmailReadExecutorDeps): Executor<GmailReadArgs, RawEmailContext> {
  return async (args, context) => {
    if (!args || typeof args.message_id !== 'string' || args.message_id.length === 0) {
      throw new Error("gmail.read: args.message_id is required (non-empty string)");
    }

    // Token presence check first. If the row is marked needs_reauth or
    // absent, loadAccessToken returns null. We do NOT consult the
    // needs_reauth flag separately; the gate already gates with
    // hasOAuth, which is wired to reject needs_reauth=true rows.
    const accessToken = await deps.tokenStore.loadAccessToken(context.user_id, 'google');
    if (accessToken === null) {
      throw new GmailReadTokenMissingError(context.user_id);
    }

    try {
      return await deps.client.getMessage(accessToken, args.message_id);
    } catch (err) {
      if (err instanceof GmailUnauthorizedError) {
        // Defense-in-depth: even if the gate let us through, Gmail can
        // reject the token (revoked, expired) at any moment. Mark the
        // row and re-throw so dispatch surfaces executor_error.
        await deps.tokenStore.markNeedsReauth(context.user_id, 'google');
      }
      throw err;
    }
  };
}

/* ---------------------------------------------------------------------- */
/* slack.founder_review                                                   */
/* ---------------------------------------------------------------------- */

// Args mirror SlackPostInput but the executor accepts unknown via the
// dispatch boundary and validates at the executor entry point.
export type SlackFounderReviewArgs = SlackPostInput;

// Re-export for callers that want to type-check args before dispatch.
export type SlackFounderReviewOutput = SlackPostResult;

export interface SlackFounderReviewExecutorDeps {
  // Optional: when undefined, the executor is registered but every
  // dispatch returns executor_error('slack adapter not wired'). This
  // gives the polling worker a uniform code path even when the kill
  // switch is off — the worker should still skip rather than dispatch
  // when FOMO_SLACK_REVIEW_ENABLED=false, but defense-in-depth.
  readonly client?: SlackClient;
}

export function slackFounderReviewExecutor(
  deps: SlackFounderReviewExecutorDeps
): Executor<SlackFounderReviewArgs, SlackFounderReviewOutput> {
  return async (args /* , context */) => {
    if (!deps.client) {
      throw new Error(
        'slack.founder_review: SlackClient not wired (FOMO_SLACK_REVIEW_ENABLED=true requires SLACK_BOT_TOKEN + SLACK_FOUNDER_CHANNEL_ID)'
      );
    }
    if (!args || typeof args !== 'object') {
      throw new Error('slack.founder_review: args must be a SlackPostInput object');
    }
    if (typeof args.alert_id !== 'string' || args.alert_id.length === 0) {
      throw new Error('slack.founder_review: args.alert_id is required (non-empty string)');
    }
    if (!args.view || typeof args.view !== 'object') {
      throw new Error('slack.founder_review: args.view is required (egress-redacted SlackEgressView)');
    }
    if (!args.rank || typeof args.rank !== 'object') {
      throw new Error('slack.founder_review: args.rank is required (label/score/reason/model_name/prompt_version)');
    }
    try {
      return await deps.client.postFounderReviewCard(args);
    } catch (err) {
      // Both auth and api errors bubble as executor_error via dispatch.
      // The polling worker reads them off the cycle outcome and decides
      // whether to retry / fail / pause. We preserve the original Error
      // shape so the worker can inspect (err instanceof SlackAuthError)
      // or err.retryable.
      if (err instanceof SlackAuthError || err instanceof SlackApiError) {
        throw err;
      }
      throw err;
    }
  };
}

/* ---------------------------------------------------------------------- */
/* Wireup helper                                                          */
/* ---------------------------------------------------------------------- */

export interface ExternalExecutorDeps {
  readonly gmailClient: GmailClient;
  readonly tokenStore: TokenStore;
  // Phase 3D.1 — optional. When undefined, slack.founder_review is
  // STILL registered (so dispatch returns executor_error instead of
  // no_executor_for_tool), but every call surfaces "adapter not wired".
  // The polling worker's slackReviewDep guards the call site so this
  // path only fires as defense-in-depth.
  readonly slackClient?: SlackClient;
}

// Single entry point. Registers all external executors that have
// landed. Callers that flipped a tool's executor_status to 'implemented'
// in the tool registry MUST call this before any gate decision goes
// through dispatch — otherwise the gate allows but dispatch returns
// no_executor_for_tool. Phase 3B.2 registers gmail.read; 3D.1 adds
// slack.founder_review.
export function wireExternalExecutors(table: DispatchTable, deps: ExternalExecutorDeps): void {
  table.register('gmail.read', gmailReadExecutor({
    client: deps.gmailClient,
    tokenStore: deps.tokenStore
  }));
  table.register('slack.founder_review', slackFounderReviewExecutor({
    client: deps.slackClient
  }));
}
