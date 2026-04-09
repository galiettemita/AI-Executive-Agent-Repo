import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { executeImplementedLocalSkill, implementedLocalSkills, resolveSupportedLocalSkills } from './local-skills.js';

describe('edge local skills', () => {
  it('defaults supported skills to the implemented local handlers', () => {
    assert.deepEqual(resolveSupportedLocalSkills(undefined), implementedLocalSkills());
  });

  it('filters configured skills to implemented handlers only', () => {
    assert.deepEqual(resolveSupportedLocalSkills('voice-wake-say,apple-notes-skill,apple-remind-me'), [
      'voice-wake-say',
      'apple-remind-me'
    ]);
  });

  it('executes implemented skills and rejects unknown ones', () => {
    assert.deepEqual(executeImplementedLocalSkill('voice-wake-say', { text: 'hello' }), {
      data: {
        spoken_text: 'hello',
        transport: 'local_say'
      }
    });
    assert.equal(executeImplementedLocalSkill('apple-notes-skill', {}), null);
  });
});
