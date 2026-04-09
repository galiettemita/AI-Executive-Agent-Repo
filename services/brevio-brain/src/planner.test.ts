import assert from 'node:assert/strict';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { loadBrainConfig, loadDisambiguationRules } from './config.js';
import { normalizeReasoningInput } from './normalize.js';
import { buildPlannerProposal } from './planner.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const configPath = path.join(repoRoot, 'config', 'skill-disambiguation.yaml');
const rules = loadDisambiguationRules(configPath);

describe('buildPlannerProposal', () => {
  it('produces a dispatch-ready plan instead of synthetic success results', async () => {
    const config = {
      ...loadBrainConfig(),
      plannerProvider: 'deterministic' as const,
      disambiguationConfigPath: configPath
    };

    const request = normalizeReasoningInput({
      message_text: 'send email to the team and then schedule a kickoff meeting',
      user_profile: {
        enabled_skills: ['google-workspace', 'google-calendar']
      },
      user_preferences: { email_provider: 'google' }
    });

    const result = await buildPlannerProposal(request, rules, config);

    assert.equal(result.plan.actions.length, 2);
    assert.ok(result.plan.run_id.length > 0);
    assert.equal(result.plan.thread_id, result.plan.run_id);
    assert.equal(result.plan.requires_approval, true);
    assert.ok(result.plan.actions.every((action) => !('status_code' in action)));
    assert.ok(result.plan.actions.every((action) => action.action_type === 'execute_skill'));
    assert.ok(result.plan.actions.every((action) => action.run_id === result.plan.run_id));
    assert.ok(result.plan.actions.every((action) => action.attempt === 1));
    assert.equal(result.plan.policy_summary.requires_consent, true);
  });

  it('keeps deterministic planning when model augmentation is configured without credentials', async () => {
    const config = {
      ...loadBrainConfig(),
      plannerProvider: 'openai_responses' as const,
      disambiguationConfigPath: configPath
    };

    const request = normalizeReasoningInput({
      message_text: 'play music',
      user_profile: {
        enabled_skills: ['spotify-web-api']
      },
      user_preferences: { music_provider: 'spotify', allow_external_reasoning: true }
    });

    const result = await buildPlannerProposal(request, rules, config);

    assert.equal(result.plan.planner_mode, 'deterministic');
    assert.ok(result.plan.run_id.length > 0);
    assert.match(result.plan.reasoning.join(' '), /credentials are unavailable/);
  });

  it('preserves supplied run and thread ids for planner ownership', async () => {
    const config = {
      ...loadBrainConfig(),
      plannerProvider: 'deterministic' as const,
      disambiguationConfigPath: configPath
    };

    const request = normalizeReasoningInput({
      message_text: 'play music',
      run_id: 'run-fixed-123',
      thread_id: 'thread-fixed-456',
      user_profile: {
        enabled_skills: ['spotify-web-api']
      },
      user_preferences: { music_provider: 'spotify' }
    });

    const result = await buildPlannerProposal(request, rules, config);

    assert.equal(result.plan.run_id, 'run-fixed-123');
    assert.equal(result.plan.thread_id, 'thread-fixed-456');
    assert.ok(result.plan.actions.every((action) => action.run_id === 'run-fixed-123'));
  });

  it('fans out research tasks across multiple approved specialists and adds reconciliation', async () => {
    const config = {
      ...loadBrainConfig(),
      plannerProvider: 'deterministic' as const,
      disambiguationConfigPath: configPath
    };

    const request = normalizeReasoningInput({
      message_text: 'research the latest A2A standards',
      user_profile: {
        enabled_skills: ['tavily', 'brave-search', 'firecrawl-search']
      }
    });

    const result = await buildPlannerProposal(request, rules, config);
    const executeActions = result.plan.actions.filter((action) => action.action_type === 'execute_skill');
    const reconcileActions = result.plan.actions.filter((action) => action.action_type === 'reconcile_results');

    assert.equal(executeActions.length, 3);
    assert.equal(reconcileActions.length, 1);
    assert.ok(executeActions.every((action) => action.fanout_group_id === 'fanout_t1'));
    assert.deepEqual(
      reconcileActions[0]?.step_dependencies?.sort(),
      executeActions.map((action) => action.step_id).sort()
    );
  });
});
