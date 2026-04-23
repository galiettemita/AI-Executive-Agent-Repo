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
        enabled_skills: ['apple-music'],
        preferences: { music_provider: 'apple_music' }
      }
    });

    assert.deepEqual(result.skills, ['apple-music']);
  });

  it('selects the preferred task app for task-management intents', () => {
    const result = classifyIntent({
      message_text: 'add task to call the bank tomorrow',
      user_profile: {
        enabled_skills: ['linear'],
        preferences: { task_app: 'linear' }
      }
    });

    assert.equal(result.intent, 'tasks.manage');
    assert.deepEqual(result.skills, ['linear']);
  });

  it('routes attached audio to transcription skills', () => {
    const result = classifyIntent({
      message_text: 'please handle this',
      content_parts: [{ type: 'audio', asset_id: 'audio-1' }],
      media_assets: [{ asset_id: 'audio-1', mime_type: 'audio/ogg' }],
      user_profile: {
        enabled_skills: ['asr', 'gemini-stt']
      }
    });

    assert.equal(result.intent, 'speech.transcribe');
    assert.deepEqual(result.skills, ['asr', 'gemini-stt']);
  });

  it('routes document parsing requests to pdf-tools', () => {
    const result = classifyIntent({
      message_text: 'extract text from this pdf',
      user_profile: {
        enabled_skills: ['pdf-tools']
      }
    });

    assert.equal(result.intent, 'document.parse');
    assert.deepEqual(result.skills, ['pdf-tools']);
  });
});
