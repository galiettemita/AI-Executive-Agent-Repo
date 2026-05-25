import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { createToolRegistry, type ToolDescriptor, type ToolId } from './tool-registry.ts';

const EXPECTED_TOOL_IDS: ToolId[] = [
  'gmail.read',
  'sendblue.send_user_message',
  'slack.founder_review',
  'audit.write',
  'feedback.write',
  'memory_signal.write'
];

const EXPECTED_EXTERNAL_TOOL_IDS: ToolId[] = ['gmail.read', 'sendblue.send_user_message'];
const EXPECTED_INTERNAL_CAPABILITY_IDS: ToolId[] = [
  'slack.founder_review',
  'audit.write',
  'feedback.write',
  'memory_signal.write'
];

describe('createToolRegistry', () => {
  it('declares exactly the six v0.1 active tools', () => {
    const registry = createToolRegistry();
    const ids = registry.getActiveTools().map((t) => t.id).sort();
    assert.deepEqual(ids, [...EXPECTED_TOOL_IDS].sort());
  });

  it('every active tool has well-formed metadata', () => {
    const registry = createToolRegistry();
    for (const tool of registry.getActiveTools()) {
      assert.ok(tool.id.length > 0, `tool ${tool.id} has empty id`);
      assert.match(tool.surface, /^(external|internal)$/, `tool ${tool.id} bad surface ${tool.surface}`);
      assert.match(tool.executor_status, /^(declared|implemented)$/, `tool ${tool.id} bad executor_status ${tool.executor_status}`);
      assert.match(tool.category, /^(context|action|control)$/, `tool ${tool.id} bad category ${tool.category}`);
      assert.match(tool.risk_tier, /^(read|send|internal)$/, `tool ${tool.id} bad risk_tier ${tool.risk_tier}`);
      assert.ok(tool.description.length > 0, `tool ${tool.id} has empty description`);
      assert.equal(typeof tool.requires_consent, 'boolean');
      assert.ok(
        tool.requires_oauth_provider === null || tool.requires_oauth_provider === 'google',
        `tool ${tool.id} oauth provider must be null or google in v0.1`
      );
    }
  });

  it('getTool returns the tool by id', () => {
    const registry = createToolRegistry();
    const gmail = registry.getTool('gmail.read');
    assert.ok(gmail);
    assert.equal(gmail?.id, 'gmail.read');
    assert.equal(gmail?.surface, 'external');
    assert.equal(gmail?.executor_status, 'implemented');
    assert.equal(gmail?.category, 'context');
    assert.equal(gmail?.risk_tier, 'read');
    assert.equal(gmail?.requires_consent, true);
    assert.equal(gmail?.requires_oauth_provider, 'google');
  });

  it('getTool returns null for unknown tools', () => {
    const registry = createToolRegistry();
    assert.equal(registry.getTool('booking.flights'), null);
    assert.equal(registry.getTool(''), null);
    assert.equal(registry.getTool('gmail.write'), null); // close to but not active
  });

  it('isActiveTool reports membership correctly', () => {
    const registry = createToolRegistry();
    for (const id of EXPECTED_TOOL_IDS) {
      assert.equal(registry.isActiveTool(id), true, `expected ${id} to be active`);
    }
    assert.equal(registry.isActiveTool('booking.flights'), false);
    assert.equal(registry.isActiveTool('calendar.write'), false);
    assert.equal(registry.isActiveTool('email.send'), false);
  });

  it('Gmail read is the only consent-requiring tool in v0.1', () => {
    const registry = createToolRegistry();
    const consentTools = registry.getActiveTools().filter((t) => t.requires_consent);
    assert.equal(consentTools.length, 1);
    assert.equal(consentTools[0]?.id, 'gmail.read');
  });

  it('SendBlue is the only send-tier tool in v0.1 (Phase 3D.1: slack.founder_review is internal-tier)', () => {
    const registry = createToolRegistry();
    const sendTools = registry.getActiveTools().filter((t) => t.risk_tier === 'send').map((t) => t.id).sort();
    assert.deepEqual(sendTools, ['sendblue.send_user_message']);
  });

  it('slack.founder_review is internal-tier, not send-tier (Phase 3D.1)', () => {
    const registry = createToolRegistry();
    const slack = registry.getTool('slack.founder_review');
    assert.ok(slack);
    assert.equal(slack?.risk_tier, 'internal');
    assert.equal(slack?.category, 'control');
    assert.equal(slack?.surface, 'internal');
  });
});

describe('createToolRegistry — external vs internal surface separation', () => {
  it('getExternalTools returns only gmail.read + sendblue.send_user_message', () => {
    const registry = createToolRegistry();
    const ids = registry.getExternalTools().map((t) => t.id).sort();
    assert.deepEqual(ids, [...EXPECTED_EXTERNAL_TOOL_IDS].sort());
  });

  it('getInternalCapabilities returns slack.founder_review + the three writers', () => {
    const registry = createToolRegistry();
    const ids = registry.getInternalCapabilities().map((t) => t.id).sort();
    assert.deepEqual(ids, [...EXPECTED_INTERNAL_CAPABILITY_IDS].sort());
  });

  it('external and internal are disjoint and partition the active set', () => {
    const registry = createToolRegistry();
    const ext = new Set(registry.getExternalTools().map((t) => t.id));
    const int = new Set(registry.getInternalCapabilities().map((t) => t.id));
    for (const id of ext) {
      assert.ok(!int.has(id), `tool ${id} appears in both external and internal sets`);
    }
    const union = new Set([...ext, ...int]);
    assert.equal(union.size, registry.getActiveTools().length);
  });

  it('every external tool has surface=external', () => {
    const registry = createToolRegistry();
    for (const t of registry.getExternalTools()) {
      assert.equal(t.surface, 'external');
    }
  });

  it('every internal capability has surface=internal', () => {
    const registry = createToolRegistry();
    for (const t of registry.getInternalCapabilities()) {
      assert.equal(t.surface, 'internal');
    }
  });

  it('slack.founder_review is internal (not user-invokable)', () => {
    const registry = createToolRegistry();
    const slack = registry.getTool('slack.founder_review');
    assert.equal(slack?.surface, 'internal');
  });
});

describe('createToolRegistry — executor_status honesty (Phase 3A + 3B.2)', () => {
  // Phase 3A wired dispatch executors for the three internal capabilities.
  // Phase 3B.2 flipped gmail.read to 'implemented' alongside the
  // gmailReadExecutor wireup. Phase 3D.1 flipped slack.founder_review to
  // 'implemented' alongside the slackFounderReviewExecutor + SlackClient
  // adapter wireup. sendblue.send_user_message remains declared until 3E.

  it('all internal capabilities, gmail.read, and slack.founder_review are implemented', () => {
    const registry = createToolRegistry();
    const implemented = registry
      .getActiveTools()
      .filter((t) => t.executor_status === 'implemented')
      .map((t) => t.id);
    assert.deepEqual(
      [...implemented].sort(),
      ['audit.write', 'feedback.write', 'gmail.read', 'memory_signal.write', 'slack.founder_review']
    );
  });

  it('only sendblue.send_user_message remains declared (3E will flip it)', () => {
    const registry = createToolRegistry();
    const declared = registry
      .getActiveTools()
      .filter((t) => t.executor_status === 'declared')
      .map((t) => t.id);
    assert.deepEqual([...declared].sort(), ['sendblue.send_user_message']);
  });

  it('gmail.read is the only implemented EXTERNAL tool (slack.founder_review is internal; sendblue still declared)', () => {
    const registry = createToolRegistry();
    const externalImpl = registry
      .getExternalTools()
      .filter((t) => t.executor_status === 'implemented')
      .map((t) => t.id);
    assert.deepEqual(externalImpl, ['gmail.read']);
  });
});

describe('createToolRegistry — immutability', () => {
  it('returned tool descriptors are frozen (cannot be mutated at runtime)', () => {
    const registry = createToolRegistry();
    const [first] = registry.getActiveTools();
    assert.ok(first);
    assert.throws(() => {
      (first as unknown as { id: string }).id = 'mutated';
    });
  });

  it('returned active tools list itself is frozen', () => {
    const registry = createToolRegistry();
    const list = registry.getActiveTools();
    assert.throws(() => {
      (list as unknown as ToolDescriptor[]).pop();
    });
  });

  it('external and internal lists are frozen', () => {
    const registry = createToolRegistry();
    assert.throws(() => {
      (registry.getExternalTools() as unknown as ToolDescriptor[]).pop();
    });
    assert.throws(() => {
      (registry.getInternalCapabilities() as unknown as ToolDescriptor[]).pop();
    });
  });
});
