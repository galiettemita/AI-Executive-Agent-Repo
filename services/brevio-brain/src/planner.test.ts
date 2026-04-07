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
    assert.equal(result.plan.requires_approval, true);
    assert.ok(result.plan.actions.every((action) => !('status_code' in action)));
    assert.ok(result.plan.actions.every((action) => action.action_type === 'execute_skill'));
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
    assert.match(result.plan.reasoning.join(' '), /credentials are unavailable/);
  });
});
