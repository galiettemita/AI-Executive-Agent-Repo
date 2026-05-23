import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from './kill-switches.ts';
import { decidePolicy, type PolicyGateDeps } from './policy-gate.ts';
import { createToolRegistry, type ToolId, type ToolRegistry } from './tool-registry.ts';

// makeDeps composes the default test dependencies. The optional `implemented`
// list flips the registry's executor_status to 'implemented' for the named
// tools, so tests that exercise downstream gate logic (kill switches, consent,
// OAuth) can isolate that logic from the new external+declared → not_implemented
// short-circuit.
function makeDeps(
  overrides: Partial<PolicyGateDeps> & { implemented?: readonly ToolId[] } = {}
): PolicyGateDeps {
  const { implemented, ...rest } = overrides;
  let registry: ToolRegistry = rest.registry ?? createToolRegistry();
  if (implemented && implemented.length > 0) {
    const baseRegistry = registry;
    const set = new Set<ToolId>(implemented);
    registry = {
      ...baseRegistry,
      getTool(id: string) {
        const t = baseRegistry.getTool(id);
        if (!t) return null;
        if (set.has(t.id)) {
          return Object.freeze({ ...t, executor_status: 'implemented' as const });
        }
        return t;
      }
    };
  }
  return {
    switches: SAFE_DEFAULT_KILL_SWITCHES,
    hasConsent: () => true,
    hasOAuth: () => true,
    ...rest,
    registry
  };
}

describe('decidePolicy — unknown tool', () => {
  it('denies with code unknown_tool', () => {
    const d = decidePolicy({ tool_id: 'booking.flights', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'unknown_tool');
    assert.equal(d.tool_id, 'booking.flights');
    assert.equal(d.user_id, 'u1');
    assert.match(d.reason, /not in the v0\.1 registry/);
  });

  it('denies empty tool_id', () => {
    const d = decidePolicy({ tool_id: '', user_id: 'u1' }, makeDeps());
    assert.equal(d.code, 'unknown_tool');
  });
});

describe('decidePolicy — not_implemented (external + declared)', () => {
  it('denies gmail.read because no executor is wired in Phase 2B', () => {
    const d = decidePolicy({ tool_id: 'gmail.read', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'not_implemented');
    assert.match(d.reason, /external tool gmail\.read is declared.*no executor is wired/);
  });

  it('denies sendblue.send_user_message because no executor is wired in Phase 2B', () => {
    const d = decidePolicy({ tool_id: 'sendblue.send_user_message', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'not_implemented');
  });

  it('not_implemented short-circuits before consent / oauth checks', () => {
    let consentCalls = 0;
    let oauthCalls = 0;
    decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({
        hasConsent: () => {
          consentCalls++;
          return false;
        },
        hasOAuth: () => {
          oauthCalls++;
          return false;
        }
      })
    );
    assert.equal(consentCalls, 0, 'hasConsent should not run before not_implemented');
    assert.equal(oauthCalls, 0, 'hasOAuth should not run before not_implemented');
  });

  it('not_implemented short-circuits before kill-switch evaluation', () => {
    // sendblue is send-tier; FOMO_SEND_ENABLED=true would otherwise allow it.
    // Because sendblue is declared (no executor), the gate must still deny —
    // and with code not_implemented, not send_disabled.
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      makeDeps({ switches })
    );
    assert.equal(d.code, 'not_implemented');
  });
});

describe('decidePolicy — internal capabilities can be declared without implementations', () => {
  // Per the design: internal/control capabilities are substrate the system
  // uses on its own behalf, not user-invokable. They're allowed to be
  // declared in the registry before their backing implementation lands.
  it('allows audit.write (internal+declared) under default kill switches', () => {
    const d = decidePolicy({ tool_id: 'audit.write', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
  });

  it('allows feedback.write (internal+declared)', () => {
    assert.equal(decidePolicy({ tool_id: 'feedback.write', user_id: 'u1' }, makeDeps()).allowed, true);
  });

  it('allows memory_signal.write (internal+declared)', () => {
    assert.equal(decidePolicy({ tool_id: 'memory_signal.write', user_id: 'u1' }, makeDeps()).allowed, true);
  });

  it('denies slack.founder_review (internal+declared+send-tier) under default kill switches via send_disabled, NOT not_implemented', () => {
    // slack is internal so not_implemented does not apply, but it is send-tier
    // and FOMO_SEND_ENABLED defaults to false → the kill switch denies.
    const d = decidePolicy({ tool_id: 'slack.founder_review', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'send_disabled');
  });
});

describe('decidePolicy — send-tier kill switches (tested on implemented external tool)', () => {
  it('denies sendblue.send_user_message when FOMO_SEND_ENABLED is false (default)', () => {
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      makeDeps({ implemented: ['sendblue.send_user_message'] })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'send_disabled');
    assert.match(d.reason, /FOMO_SEND_ENABLED is false/);
  });

  it('allows sendblue with manual_send intent when FOMO_SEND_ENABLED is true', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'manual_send' },
      makeDeps({ switches, implemented: ['sendblue.send_user_message'] })
    );
    assert.equal(d.allowed, true);
  });

  it('denies auto_send when FOMO_AUTO_SEND_ENABLED is false (even with FOMO_SEND_ENABLED=true)', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'auto_send' },
      makeDeps({ switches, implemented: ['sendblue.send_user_message'] })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'auto_send_disabled');
  });

  it('allows auto_send when both kill switches are true', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true', FOMO_AUTO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'auto_send' },
      makeDeps({ switches, implemented: ['sendblue.send_user_message'] })
    );
    assert.equal(d.allowed, true);
  });

  it('treats missing intent as manual_send (does not require FOMO_AUTO_SEND_ENABLED)', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      makeDeps({ switches, implemented: ['sendblue.send_user_message'] })
    );
    assert.equal(d.allowed, true);
  });
});

describe('decidePolicy — consent (tested on implemented external tool)', () => {
  it('denies gmail.read without consent', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ implemented: ['gmail.read'], hasConsent: () => false })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'consent_missing');
    assert.match(d.reason, /has not consented/);
  });

  it('does not require consent for tools that do not need it', () => {
    let consentCalls = 0;
    const d = decidePolicy(
      { tool_id: 'audit.write', user_id: 'u1' },
      makeDeps({
        hasConsent: () => {
          consentCalls++;
          return false;
        }
      })
    );
    assert.equal(d.allowed, true);
    assert.equal(consentCalls, 0, 'hasConsent should not be called for non-consent tools');
  });
});

describe('decidePolicy — OAuth (tested on implemented external tool)', () => {
  it('denies gmail.read when google OAuth is not connected', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ implemented: ['gmail.read'], hasOAuth: () => false })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'oauth_not_connected');
    assert.match(d.reason, /no live google OAuth connection/);
  });

  it('allows gmail.read when consent + oauth both pass', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ implemented: ['gmail.read'], hasConsent: () => true, hasOAuth: () => true })
    );
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
  });

  it('does not check OAuth for tools that do not require it', () => {
    let oauthCalls = 0;
    const d = decidePolicy(
      { tool_id: 'audit.write', user_id: 'u1' },
      makeDeps({
        hasOAuth: () => {
          oauthCalls++;
          return false;
        }
      })
    );
    assert.equal(d.allowed, true);
    assert.equal(oauthCalls, 0, 'hasOAuth should not be called for non-OAuth tools');
  });
});

describe('decidePolicy — fail-closed when dep callbacks throw', () => {
  it('maps hasConsent throw to deny policy_check_error', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({
        implemented: ['gmail.read'],
        hasConsent: () => {
          throw new Error('consent store unreachable');
        }
      })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'policy_check_error');
    assert.match(d.reason, /consent store unreachable/);
  });

  it('maps hasOAuth throw to deny policy_check_error', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({
        implemented: ['gmail.read'],
        hasConsent: () => true,
        hasOAuth: () => {
          throw new Error('token store unreachable');
        }
      })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'policy_check_error');
    assert.match(d.reason, /token store unreachable/);
  });
});

describe('decidePolicy — check ordering and short-circuit', () => {
  it('reports unknown_tool before any other check (and does not call deps)', () => {
    let consentCalls = 0;
    let oauthCalls = 0;
    const d = decidePolicy(
      { tool_id: 'mystery.tool', user_id: 'u1' },
      makeDeps({
        hasConsent: () => {
          consentCalls++;
          return true;
        },
        hasOAuth: () => {
          oauthCalls++;
          return true;
        }
      })
    );
    assert.equal(d.code, 'unknown_tool');
    assert.equal(consentCalls, 0);
    assert.equal(oauthCalls, 0);
  });

  it('reports not_implemented before send_disabled for declared external send-tier tools', () => {
    // sendblue is external+declared+send. Under default kill switches BOTH
    // not_implemented and send_disabled would apply; the gate must report
    // not_implemented because it's the more fundamental defect.
    const d = decidePolicy({ tool_id: 'sendblue.send_user_message', user_id: 'u1' }, makeDeps());
    assert.equal(d.code, 'not_implemented');
  });

  it('reports send_disabled before consent_missing for implemented send-tier tools without send_enabled', () => {
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      makeDeps({ implemented: ['sendblue.send_user_message'], hasConsent: () => false })
    );
    assert.equal(d.code, 'send_disabled');
  });

  it('reports consent_missing before oauth_not_connected for implemented tools', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ implemented: ['gmail.read'], hasConsent: () => false, hasOAuth: () => false })
    );
    assert.equal(d.code, 'consent_missing');
  });
});

describe('decidePolicy — unknown risk tier (defensive)', () => {
  it('denies with code unknown_tier when a tool surfaces a tier outside the known set', () => {
    // Defensive only: typed risk_tier prevents this at compile time, but if the
    // registry is ever extended without updating the gate, the gate must
    // fail-close. We simulate by stubbing the registry to return a forged tool.
    // The forged tool is internal+(no executor_status) so the not_implemented
    // check skips and we reach the risk-tier switch with 'mystery'.
    const baseRegistry = createToolRegistry();
    const forgedTool = {
      id: 'forged.tool',
      surface: 'internal',
      executor_status: 'implemented',
      category: 'control',
      risk_tier: 'mystery',
      description: 'tier not in the gate switch',
      requires_consent: false,
      requires_oauth_provider: null
    } as unknown as ReturnType<typeof baseRegistry.getTool>;
    const deps: PolicyGateDeps = {
      ...makeDeps(),
      registry: {
        ...baseRegistry,
        getTool: () => forgedTool,
        isActiveTool: () => true
      }
    };
    const d = decidePolicy({ tool_id: 'forged.tool', user_id: 'u1' }, deps);
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'unknown_tier');
    assert.match(d.reason, /unknown risk tier/);
  });
});

describe('decidePolicy — decision objects are frozen', () => {
  it('allow decision cannot be mutated', () => {
    const d = decidePolicy({ tool_id: 'audit.write', user_id: 'u1' }, makeDeps());
    assert.throws(() => {
      (d as unknown as { allowed: boolean }).allowed = false;
    });
  });

  it('deny decision cannot be mutated', () => {
    const d = decidePolicy({ tool_id: 'booking.flights', user_id: 'u1' }, makeDeps());
    assert.throws(() => {
      (d as unknown as { code: string }).code = 'allowed';
    });
  });
});
