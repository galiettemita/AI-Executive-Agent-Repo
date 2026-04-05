import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { classifyIntent } from './classify.js';

describe('classifyIntent', () => {
  it('hard-blocks disabled skills instead of routing anyway', () => {
    const result = classifyIntent({
      message_text: 'play music for me',
      deployment_mode: 'cloud',
      user_profile: {
        enabled_skills: ['todoist'],
        preferences: { music_provider: 'spotify' }
      }
    });

    assert.equal(result.intent, 'music.playback');
    assert.deepEqual(result.skills, []);
    assert.deepEqual(result.blocked_skills, ['spotify-web-api']);
    assert.equal(result.clarification_required, true);
  });

  it('uses user preferences for music routing', () => {
    const result = classifyIntent({
      message_text: 'play music',
      deployment_mode: 'local_mac',
      user_profile: {
        preferences: { music_provider: 'apple_music' }
      }
    });

    assert.deepEqual(result.skills, ['apple-music']);
  });

  it('selects the preferred task app for task-management intents', () => {
    const result = classifyIntent({
      message_text: 'add task to call the bank tomorrow',
      user_profile: {
        preferences: { task_app: 'linear' }
      }
    });

    assert.equal(result.intent, 'tasks.manage');
    assert.deepEqual(result.skills, ['linear']);
  });
});
