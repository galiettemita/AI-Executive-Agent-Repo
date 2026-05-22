import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SAFE_DEFAULT_KILL_SWITCHES, loadKillSwitches } from './kill-switches.ts';
import { decidePolicy, type PolicyGateDeps } from './policy-gate.ts';
import { createToolRegistry } from './tool-registry.ts';

function makeDeps(overrides: Partial<PolicyGateDeps> = {}): PolicyGateDeps {
  return {
    registry: createToolRegistry(),
    switches: SAFE_DEFAULT_KILL_SWITCHES,
    hasConsent: () => true,
    hasOAuth: () => true,
    ...overrides
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

describe('decidePolicy — internal tools (no consent / no OAuth / no send gates)', () => {
  it('allows audit.write under default kill switches', () => {
    const d = decidePolicy({ tool_id: 'audit.write', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, true);
    assert.equal(d.code, 'allowed');
  });

  it('allows feedback.write and memory_signal.write', () => {
    assert.equal(decidePolicy({ tool_id: 'feedback.write', user_id: 'u1' }, makeDeps()).allowed, true);
    assert.equal(decidePolicy({ tool_id: 'memory_signal.write', user_id: 'u1' }, makeDeps()).allowed, true);
  });
});

describe('decidePolicy — send-tier kill switches', () => {
  it('denies sendblue.send_user_message when FOMO_SEND_ENABLED is false (default)', () => {
    const d = decidePolicy({ tool_id: 'sendblue.send_user_message', user_id: 'u1' }, makeDeps());
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'send_disabled');
    assert.match(d.reason, /FOMO_SEND_ENABLED is false/);
  });

  it('denies slack.founder_review when FOMO_SEND_ENABLED is false', () => {
    const d = decidePolicy({ tool_id: 'slack.founder_review', user_id: 'u1' }, makeDeps());
    assert.equal(d.code, 'send_disabled');
  });

  it('allows sendblue with manual_send intent when FOMO_SEND_ENABLED is true', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'manual_send' },
      makeDeps({ switches })
    );
    assert.equal(d.allowed, true);
  });

  it('denies auto_send when FOMO_AUTO_SEND_ENABLED is false (even with FOMO_SEND_ENABLED=true)', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'auto_send' },
      makeDeps({ switches })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'auto_send_disabled');
  });

  it('allows auto_send when both kill switches are true', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true', FOMO_AUTO_SEND_ENABLED: 'true' });
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1', intent: 'auto_send' },
      makeDeps({ switches })
    );
    assert.equal(d.allowed, true);
  });

  it('treats missing intent as manual_send (does not require FOMO_AUTO_SEND_ENABLED)', () => {
    const switches = loadKillSwitches({ FOMO_SEND_ENABLED: 'true' });
    const d = decidePolicy({ tool_id: 'sendblue.send_user_message', user_id: 'u1' }, makeDeps({ switches }));
    assert.equal(d.allowed, true);
  });
});

describe('decidePolicy — consent', () => {
  it('denies gmail.read without consent', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ hasConsent: () => false })
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

describe('decidePolicy — OAuth', () => {
  it('denies gmail.read when google OAuth is not connected', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ hasOAuth: () => false })
    );
    assert.equal(d.allowed, false);
    assert.equal(d.code, 'oauth_not_connected');
    assert.match(d.reason, /no live google OAuth connection/);
  });

  it('allows gmail.read when consent + oauth both pass', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ hasConsent: () => true, hasOAuth: () => true })
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

  it('reports send_disabled before consent_missing for send-tier tools without send_enabled', () => {
    const d = decidePolicy(
      { tool_id: 'sendblue.send_user_message', user_id: 'u1' },
      makeDeps({ hasConsent: () => false })
    );
    assert.equal(d.code, 'send_disabled');
  });

  it('reports consent_missing before oauth_not_connected', () => {
    const d = decidePolicy(
      { tool_id: 'gmail.read', user_id: 'u1' },
      makeDeps({ hasConsent: () => false, hasOAuth: () => false })
    );
    assert.equal(d.code, 'consent_missing');
  });
});

describe('decidePolicy — unknown risk tier (defensive)', () => {
  it('denies with code unknown_tier when a tool surfaces a tier outside the known set', () => {
    // Defensive only: typed risk_tier prevents this at compile time, but if the
    // registry is ever extended without updating the gate, the gate must
    // fail-close. We simulate by stubbing the registry to return a forged tool.
    const baseRegistry = createToolRegistry();
    const forgedTool = {
      id: 'forged.tool',
      surface: 'internal',
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
