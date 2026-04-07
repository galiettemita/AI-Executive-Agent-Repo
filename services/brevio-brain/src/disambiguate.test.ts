import assert from 'node:assert/strict';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { loadDisambiguationRules } from './config.js';
import { disambiguateSkills } from './disambiguate.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const configPath = path.join(repoRoot, 'config', 'skill-disambiguation.yaml');
const rules = loadDisambiguationRules(configPath);

describe('disambiguateSkills', () => {
  it('requires clarification for mixed email send and search requests', () => {
    const result = disambiguateSkills(
      {
        message_text: 'send email to John and search inbox for his last reply',
        intent: 'email.search',
        candidate_skills: ['apple-mail', 'apple-mail-search'],
        enabled_skills: ['apple-mail', 'apple-mail-search']
      },
      rules
    );

    assert.equal(result.clarification_required, true);
    assert.deepEqual(result.resolved_skills, []);
  });

  it('passes through explicit modern skills that do not need grouped routing', () => {
    const result = disambiguateSkills(
      {
        message_text: 'play music',
        candidate_skills: ['apple-music'],
        enabled_skills: ['apple-music']
      },
      rules
    );

    assert.deepEqual(result.resolved_skills, ['apple-music']);
  });

  it('routes navigation requests to maps', () => {
    const result = disambiguateSkills(
      {
        message_text: 'navigate to jfk airport',
        intent: 'places.search',
        candidate_skills: ['local-places'],
        enabled_skills: ['google-maps']
      },
      rules
    );

    assert.deepEqual(result.resolved_skills, ['google-maps']);
    assert.deepEqual(result.group_hits, ['places-location']);
  });
});
