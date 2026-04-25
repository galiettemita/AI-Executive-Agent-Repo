import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  executeImplementedLocalSkill,
  implementedLocalSkills,
  resolveSupportedLocalSkills,
  supportsLocalOperation
} from './local-skills.js';

describe('edge local skills', () => {
  it('defaults supported skills to the implemented local handlers', () => {
    assert.deepEqual(resolveSupportedLocalSkills(undefined), implementedLocalSkills());
  });

  it('filters configured skills to implemented handlers only', () => {
    assert.deepEqual(resolveSupportedLocalSkills('voice-wake-say,apple-notes-skill,apple-remind-me,camsnap'), [
      'voice-wake-say',
      'apple-remind-me',
      'camsnap'
    ]);
  });

  it('executes implemented skills and rejects unknown ones', () => {
    assert.equal(supportsLocalOperation('voice-wake-say', 'speak'), true);
    assert.equal(supportsLocalOperation('voice-wake-say', 'search'), false);
    assert.deepEqual(executeImplementedLocalSkill('voice-wake-say', 'speak', { text: 'hello' }), {
      status: 'SUCCESS',
      data: {
        spoken_text: 'hello',
        transport: 'local_say',
        command_argv: ['say', '--', 'hello']
      }
    });
    assert.deepEqual(executeImplementedLocalSkill('camsnap', 'capture', {}), {
      status: 'NEEDS_CONSENT',
      data: {
        provider: 'camsnap',
        operation: 'capture',
        status: 'permission_required',
        consent_required: true,
        output_modalities: ['image']
      }
    });
    assert.deepEqual(executeImplementedLocalSkill('apple-remind-me', 'create', {}), {
      status: 'SIMULATED',
      data: {
        reminder_title: 'Reminder from Brevio',
        created: false,
        simulated: true
      }
    });
    assert.equal(executeImplementedLocalSkill('apple-notes-skill', 'create_note', {}), null);
  });
});
