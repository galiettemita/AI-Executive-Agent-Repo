import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { normalizeWebhook } from './normalize.js';
import { GatewayState } from './state.js';
import type { GatewayConfig } from './types.js';

const config: GatewayConfig = {
  serviceName: 'brevio-gateway',
  version: 'test',
  environment: 'test',
  port: 0,
  shutdownTimeoutMs: 1000,
  whatsappWebhookSecret: 'secret',
  whatsappVerifyToken: 'verify',
  imessageAPIKey: 'key',
  temporalWebhookAPIKey: 'temporal',
  temporalWorkerBaseUrl: undefined,
  temporalWorkerTimeoutMs: 1000,
  idempotencyTtlMs: 60_000,
  sessionIdleMs: 60_000,
  rateLimitWindowMs: 60 * 60 * 1000,
  rateLimitMinuteWindowMs: 60 * 1000,
  rateLimitPerMinute: 30,
  rateLimitFreePerHour: 100,
  rateLimitProPerHour: 500
};

describe('normalizeWebhook', () => {
  it('keeps user profile hash stable across messages in the same channel session', () => {
    const state = new GatewayState();
    const nowMs = Date.UTC(2026, 3, 9, 12, 0, 0);

    const first = normalizeWebhook(
      'API',
      {
        user_id: '11111111-1111-4111-8111-111111111111',
        message_id: 'm-1',
        text: 'hello'
      },
      Buffer.from('{"message_id":"m-1"}'),
      nowMs,
      state,
      config,
      'pro'
    );

    const second = normalizeWebhook(
      'API',
      {
        user_id: '11111111-1111-4111-8111-111111111111',
        message_id: 'm-2',
        text: 'follow up'
      },
      Buffer.from('{"message_id":"m-2"}'),
      nowMs + 1_000,
      state,
      config,
      'pro'
    );

    assert.equal(first.envelope.metadata.session_id, second.envelope.metadata.session_id);
    assert.equal(first.envelope.context.user_profile_hash, second.envelope.context.user_profile_hash);
    assert.notEqual(first.envelope.metadata.channel_message_id, second.envelope.metadata.channel_message_id);
  });

  it('scopes sessions by channel as well as user id', () => {
    const state = new GatewayState();
    const nowMs = Date.UTC(2026, 3, 9, 12, 0, 0);
    const payload = {
      user_id: '11111111-1111-4111-8111-111111111111',
      text: 'hello'
    };

    const apiMessage = normalizeWebhook(
      'API',
      { ...payload, message_id: 'api-1' },
      Buffer.from('{"message_id":"api-1"}'),
      nowMs,
      state,
      config,
      'pro'
    );

    const imessage = normalizeWebhook(
      'IMESSAGE',
      { ...payload, message_id: 'imsg-1' },
      Buffer.from('{"message_id":"imsg-1"}'),
      nowMs,
      state,
      config,
      'pro'
    );

    assert.notEqual(apiMessage.envelope.metadata.session_id, imessage.envelope.metadata.session_id);
    assert.notEqual(apiMessage.envelope.context.user_profile_hash, imessage.envelope.context.user_profile_hash);
  });

  it('hydrates active skills from the shared capability inventory when the webhook omits them', () => {
    const state = new GatewayState();
    const nowMs = Date.UTC(2026, 3, 9, 12, 0, 0);

    const normalized = normalizeWebhook(
      'API',
      {
        user_id: '22222222-2222-4222-8222-222222222222',
        message_id: 'm-cap-1',
        text: 'create a task'
      },
      Buffer.from('{"message_id":"m-cap-1"}'),
      nowMs,
      state,
      {
        ...config,
        capabilityInventoryJson: JSON.stringify([
          {
            user_id: '22222222-2222-4222-8222-222222222222',
            enabled_skills: ['todoist']
          }
        ])
      },
      'pro'
    );

    assert.deepEqual(normalized.envelope.context.active_skills, ['todoist']);
  });
});
