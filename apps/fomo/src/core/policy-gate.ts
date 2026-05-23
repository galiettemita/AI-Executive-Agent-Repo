// Permission Gate — the fail-closed allow/deny decision in front of every
// tool invocation. Composes Tool Registry + Kill Switches + injected consent
// and OAuth lookups.
//
// FOMO_PLAN §9.3 lists the v0.1 substrate checks:
//   * tool exists / is real        (Tool Registry)
//   * kill switch allows           (Kill Switches)
//   * user consent exists          (injected)
//   * OAuth connected              (injected)
//
// The remaining §9.3 checks — daily cap, sender suppression, Slack approval,
// model output validity — depend on data and callers that do not yet exist in
// v0.1 and will layer on in later phases.
//
// Failure mode is always deny: any error from a dep callback maps to a deny
// decision with code 'policy_check_error' rather than throwing. The gate is a
// safety boundary, not an exception channel.

import type { KillSwitches } from './kill-switches.js';
import type { ToolDescriptor, ToolRegistry } from './tool-registry.js';

export type PolicyIntent = 'manual_send' | 'auto_send' | 'read' | 'control';

export interface PolicyRequest {
  tool_id: string;
  user_id: string;
  // Defaults to 'manual_send' if omitted. Auto-sends must declare 'auto_send'
  // explicitly so they go through the FOMO_AUTO_SEND_ENABLED gate.
  intent?: PolicyIntent;
}

export type PolicyDecisionCode =
  | 'allowed'
  | 'unknown_tool'
  | 'not_implemented'
  | 'unknown_tier'
  | 'send_disabled'
  | 'auto_send_disabled'
  | 'consent_missing'
  | 'oauth_not_connected'
  | 'policy_check_error';

export interface PolicyDecision {
  readonly allowed: boolean;
  readonly code: PolicyDecisionCode;
  // Operator-facing reason for the audit log. NOT user-facing copy — the
  // assistant voice layer is responsible for translating denies into human text
  // (FOMO_DESIGN §10).
  readonly reason: string;
  readonly tool_id: string;
  readonly user_id: string;
}

export interface PolicyGateDeps {
  registry: ToolRegistry;
  switches: KillSwitches;
  // hasConsent: returns true iff the user has granted consent for this tool.
  // Phase 2B uses an injected callback; Phase 3 wires it to a real store.
  hasConsent: (userId: string, toolId: string) => boolean;
  // hasOAuth: returns true iff the user has a live OAuth connection for the
  // given provider. Phase 2B uses an injected callback; Phase 3 wires it to
  // the token store + 'needs_reauth' flag.
  hasOAuth: (userId: string, provider: string) => boolean;
}

function deny(
  code: Exclude<PolicyDecisionCode, 'allowed'>,
  reason: string,
  tool_id: string,
  user_id: string
): PolicyDecision {
  return Object.freeze({ allowed: false, code, reason, tool_id, user_id });
}

function allow(tool: ToolDescriptor, user_id: string): PolicyDecision {
  return Object.freeze({
    allowed: true,
    code: 'allowed',
    reason: `tool ${tool.id} allowed for user ${user_id}`,
    tool_id: tool.id,
    user_id
  });
}

export function decidePolicy(req: PolicyRequest, deps: PolicyGateDeps): PolicyDecision {
  const { tool_id, user_id } = req;
  const intent: PolicyIntent = req.intent ?? 'manual_send';

  // 1. Tool must be in the registry. Unknown / future tools deny by name.
  const tool = deps.registry.getTool(tool_id);
  if (!tool) {
    return deny('unknown_tool', `tool ${tool_id} is not in the v0.1 registry`, tool_id, user_id);
  }

  // 2. External tools must be implemented. A user-invokable surface that maps
  // to a declared-but-not-wired handler is exactly the "fake/stub tool on a
  // user-reachable path" failure mode FOMO_DESIGN §9 prohibits. Deny early.
  // Internal capabilities with executor_status='declared' are fine — they are
  // substrate, not user-facing, and have no path to reach a missing executor.
  if (tool.surface === 'external' && tool.executor_status === 'declared') {
    return deny(
      'not_implemented',
      `external tool ${tool.id} is declared in the v0.1 registry but no executor is wired yet`,
      tool_id,
      user_id
    );
  }

  // 3. Risk-tier handling. Exhaustive switch so an unknown tier — should one
  // ever be added to the registry without updating the gate — fail-closes
  // here rather than silently passing.
  switch (tool.risk_tier) {
    case 'send':
      if (!deps.switches.send_enabled) {
        return deny(
          'send_disabled',
          `FOMO_SEND_ENABLED is false; send-tier tool ${tool.id} blocked`,
          tool_id,
          user_id
        );
      }
      if (intent === 'auto_send' && !deps.switches.auto_send_enabled) {
        return deny(
          'auto_send_disabled',
          `FOMO_AUTO_SEND_ENABLED is false; auto-send of ${tool.id} blocked`,
          tool_id,
          user_id
        );
      }
      break;
    case 'read':
    case 'internal':
      // No kill-switch gates for read/internal tiers.
      break;
    default: {
      const exhaustive: never = tool.risk_tier;
      return deny(
        'unknown_tier',
        `unknown risk tier on tool ${tool.id}: ${String(exhaustive)}`,
        tool_id,
        user_id
      );
    }
  }

  // 4. Consent (currently only gmail.read).
  if (tool.requires_consent) {
    let consented: boolean;
    try {
      consented = deps.hasConsent(user_id, tool.id);
    } catch (err) {
      return deny(
        'policy_check_error',
        `hasConsent check threw for ${tool.id}: ${err instanceof Error ? err.message : String(err)}`,
        tool_id,
        user_id
      );
    }
    if (!consented) {
      return deny('consent_missing', `user ${user_id} has not consented to ${tool.id}`, tool_id, user_id);
    }
  }

  // 5. OAuth (currently only gmail.read → google).
  if (tool.requires_oauth_provider !== null) {
    const provider = tool.requires_oauth_provider;
    let connected: boolean;
    try {
      connected = deps.hasOAuth(user_id, provider);
    } catch (err) {
      return deny(
        'policy_check_error',
        `hasOAuth check threw for ${tool.id}/${provider}: ${err instanceof Error ? err.message : String(err)}`,
        tool_id,
        user_id
      );
    }
    if (!connected) {
      return deny(
        'oauth_not_connected',
        `user ${user_id} has no live ${provider} OAuth connection (required by ${tool.id})`,
        tool_id,
        user_id
      );
    }
  }

  return allow(tool, user_id);
}
