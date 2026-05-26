// Tool Registry — declares the v0.1 active tools per FOMO_PLAN §9.1.
//
// A "tool" is a capability the FOMO agent can be asked to use. The registry
// is the single source of truth for what tools exist; the Permission Gate
// consults it before allowing any execution. Tools outside this list are
// rejected by name at the gate — no fake or stub tools on user-reachable
// paths (FOMO_DESIGN §9, "Real or absent. Never half-wired").
//
// Surface distinction (load-bearing — do not collapse):
//
//   external — user-facing or third-party-facing capabilities a user could
//              plausibly ask the assistant to use. v0.1 external surface:
//              gmail.read (read inbox) and sendblue.send_user_message (text me).
//
//   internal — control-plane capabilities the SYSTEM uses on its own behalf.
//              Never exposed as something the user can invoke directly. v0.1
//              internal capabilities: slack.founder_review (founder approval
//              loop), audit.write, feedback.write, memory_signal.write.
//
// Keeping these two surfaces visibly separate prevents a future caller from
// accidentally surfacing an internal capability (e.g. memory_signal.write) as
// if it were a user-invokable tool.
//
// Executor status:
//
//   declared     — the v0.1 plan lists this tool but no executor is wired
//                  yet (no Gmail/SendBlue/Slack adapter for external tools,
//                  no dispatch table mapping the tool id to its substrate
//                  store for internal capabilities). Permission Gate denies
//                  ALL declared tools with 'not_implemented' regardless of
//                  surface — declared = recognized but not executable.
//                  Substrate primitives like InMemoryAuditStore still exist
//                  and can be called DIRECTLY by the system; they are not
//                  invoked through the tool-dispatch path the gate fronts.
//   implemented  — a real handler is wired and the registry flips. The gate
//                  evaluates normal consent / OAuth / kill-switch rules.
//
// Phase 2 shipped every tool as 'declared'. Phase 3A flips the three
// internal capabilities (audit.write, feedback.write, memory_signal.write)
// to 'implemented' alongside the dispatch wiring in
// apps/fomo/src/dispatch/internal-executors.ts. External tools
// (gmail.read, sendblue.send_user_message, slack.founder_review) stay
// 'declared' through the rest of Phase 3 until their real adapters land
// (3B, 3D, 3E respectively). The Permission Gate's honest invariant:
// a tool the user (or system) can ASK to run via tool dispatch must
// have a real implementation.

export type ToolSurface = 'external' | 'internal';

export type ToolExecutorStatus = 'declared' | 'implemented';

export type ToolCategory = 'context' | 'action' | 'control';

// Risk classification used by the Permission Gate:
//   read     — pulls data from a context provider (no outbound user/3p effect)
//   send     — produces a user-visible or third-party-visible effect
//   internal — within-system bookkeeping (audit, feedback, memory signals)
export type ToolRiskTier = 'read' | 'send' | 'internal';

export type ToolId =
  | 'gmail.read'
  | 'sendblue.send_user_message'
  | 'slack.founder_review'
  | 'audit.write'
  | 'feedback.write'
  | 'memory_signal.write';

export interface ToolDescriptor {
  readonly id: ToolId;
  readonly surface: ToolSurface;
  readonly executor_status: ToolExecutorStatus;
  readonly category: ToolCategory;
  readonly risk_tier: ToolRiskTier;
  readonly description: string;
  readonly requires_consent: boolean;
  readonly requires_oauth_provider: 'google' | null;
}

const ACTIVE_TOOLS: readonly ToolDescriptor[] = Object.freeze([
  Object.freeze({
    id: 'gmail.read',
    surface: 'external',
    // Phase 3B.2: dispatch wired to GmailClient.getMessage via
    // apps/fomo/src/dispatch/external-executors.ts#gmailReadExecutor.
    // The gate still requires consent + a non-needs-reauth Google
    // OAuth token before allowing a dispatch.
    executor_status: 'implemented',
    category: 'context',
    risk_tier: 'read',
    description: 'Read-only access to a user\'s Gmail inbox for FOMO ranking.',
    requires_consent: true,
    requires_oauth_provider: 'google'
  }),
  Object.freeze({
    id: 'sendblue.send_user_message',
    surface: 'external',
    // Phase 3E.1: dispatch wired to SendBlueClient via
    // apps/fomo/src/dispatch/external-executors.ts#sendBlueSendExecutor.
    // The Permission Gate still fail-closes via risk_tier='send':
    //   - FOMO_SEND_ENABLED=false → deny('send_disabled')
    //   - intent='auto_send' + FOMO_AUTO_SEND_ENABLED=false → deny('auto_send_disabled')
    // 3E.1 only sends MANUAL (founder-approved) intents; auto-send
    // stays off through v0.1. Defense-in-depth at the outbound-sender
    // worker boundary additionally enforces the founder-phone
    // allowlist (FOMO_FOUNDER_PHONE_NUMBER) before any dispatch.
    executor_status: 'implemented',
    category: 'action',
    risk_tier: 'send',
    description: 'Send an iMessage/SMS to the user via SendBlue after approval.',
    requires_consent: false,
    requires_oauth_provider: null
  }),
  Object.freeze({
    id: 'slack.founder_review',
    surface: 'internal',
    // Phase 3D.1: dispatch wired to SlackClient via
    // apps/fomo/src/dispatch/external-executors.ts#slackFounderReviewExecutor.
    // Posting-only in 3D.1 (no inbound approval capture yet — that's 3D.2).
    executor_status: 'implemented',
    category: 'control',
    // Phase 3D.1: risk_tier='internal' (NOT 'send'). Posting to the
    // founder's own Slack channel for review is internal observability,
    // not a user-facing send. FOMO_SEND_ENABLED gates user-facing
    // SendBlue sends; FOMO_SLACK_REVIEW_ENABLED gates Slack review and
    // is enforced at TWO layers for defense-in-depth:
    //   1. bootstrap (apps/fomo/src/index.ts#buildSlackReviewWiring) —
    //      throws when the switch is on but creds are missing; returns
    //      null deps when the switch is off (worker skips the flow)
    //   2. policy gate (apps/fomo/src/core/policy-gate.ts#decidePolicy) —
    //      denies dispatch with 'slack_review_disabled' when the switch
    //      is false, regardless of how the executor was wired
    // The 'internal' risk_tier is therefore NOT a bypass — the kill
    // switch still blocks at the action boundary.
    risk_tier: 'internal',
    description: 'Post a candidate alert to the founder Slack channel for review.',
    requires_consent: false,
    requires_oauth_provider: null
  }),
  Object.freeze({
    id: 'audit.write',
    surface: 'internal',
    // Phase 3A: dispatch wired to InMemoryAuditStore / PostgresAuditStore
    // via apps/fomo/src/dispatch/internal-executors.ts#auditWriteExecutor.
    executor_status: 'implemented',
    category: 'control',
    risk_tier: 'internal',
    description: 'Append an entry to the audit log.',
    requires_consent: false,
    requires_oauth_provider: null
  }),
  Object.freeze({
    id: 'feedback.write',
    surface: 'internal',
    // Phase 3A: dispatch wired to InMemoryFeedbackStore / PostgresFeedbackStore
    // via apps/fomo/src/dispatch/internal-executors.ts#feedbackWriteExecutor.
    executor_status: 'implemented',
    category: 'control',
    risk_tier: 'internal',
    description: 'Record a feedback event from founder review or user reply.',
    requires_consent: false,
    requires_oauth_provider: null
  }),
  Object.freeze({
    id: 'memory_signal.write',
    surface: 'internal',
    // Phase 3A: dispatch wired to InMemoryMemorySignalStore /
    // PostgresMemorySignalStore via
    // apps/fomo/src/dispatch/internal-executors.ts#memorySignalUpsertExecutor.
    executor_status: 'implemented',
    category: 'control',
    risk_tier: 'internal',
    description: 'Write or update a learned memory signal for the user.',
    requires_consent: false,
    requires_oauth_provider: null
  })
] satisfies ToolDescriptor[]) as readonly ToolDescriptor[];

export interface ToolRegistry {
  // All registered tools (both surfaces). Used by the Permission Gate.
  getActiveTools(): readonly ToolDescriptor[];
  // User-facing capabilities. Anything a user could plausibly invoke.
  getExternalTools(): readonly ToolDescriptor[];
  // Control-plane capabilities. Never exposed as user-invokable tools.
  getInternalCapabilities(): readonly ToolDescriptor[];
  getTool(id: string): ToolDescriptor | null;
  isActiveTool(id: string): boolean;
}

export function isToolId(id: string): id is ToolId {
  return ACTIVE_TOOLS.some((t) => t.id === id);
}

export function createToolRegistry(): ToolRegistry {
  const externals = Object.freeze(ACTIVE_TOOLS.filter((t) => t.surface === 'external'));
  const internals = Object.freeze(ACTIVE_TOOLS.filter((t) => t.surface === 'internal'));
  return {
    getActiveTools(): readonly ToolDescriptor[] {
      return ACTIVE_TOOLS;
    },
    getExternalTools(): readonly ToolDescriptor[] {
      return externals;
    },
    getInternalCapabilities(): readonly ToolDescriptor[] {
      return internals;
    },
    getTool(id: string): ToolDescriptor | null {
      if (!isToolId(id)) return null;
      return ACTIVE_TOOLS.find((t) => t.id === id) ?? null;
    },
    isActiveTool(id: string): boolean {
      return isToolId(id);
    }
  };
}
