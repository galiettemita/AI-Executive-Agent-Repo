import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { resolveAccessTokenSigningKey } from '../../../packages/shared/src/security.js';
import {
  TEST_AUTH_ACCESS_ISSUER,
  TEST_DEVICE_ACCESS_ISSUER,
  TEST_GATEWAY_SERVICE_ISSUER,
  testAccessTokenIssuers
} from '../../../packages/shared/src/security-test-fixtures.js';

import {
  bindExecuteRequest,
  buildSessionSummaries,
  deriveSymmetricKey,
  parseRelayAuthMode,
  protectQueuedInput,
  recoverQueuedInput,
  signRelayToken,
  verifyRelayToken,
} from './security.js';

describe('edge relay security', () => {
  const relayAudience = 'brevio-edge-relay';
  const deviceSigningKey = resolveAccessTokenSigningKey(undefined, undefined, 'test', 'TEST_DEVICE_ACCESS_PRIVATE_KEY', 'device-access');
  const authSigningKey = resolveAccessTokenSigningKey(undefined, undefined, 'test', 'TEST_AUTH_ACCESS_PRIVATE_KEY', 'auth-access');
  const issuers = testAccessTokenIssuers();

  it('resolves auth mode from environment and secret presence', () => {
    assert.equal(parseRelayAuthMode(undefined, 'local', false), 'optional');
    assert.equal(parseRelayAuthMode(undefined, 'production', true), 'required');
    assert.equal(parseRelayAuthMode(undefined, 'production', false), 'required');
    assert.equal(parseRelayAuthMode('optional', 'production', true), 'optional');
  });

  it('signs and verifies relay tokens', () => {
    const nowSeconds = Math.floor(Date.now() / 1000);
    const token = signRelayToken(deviceSigningKey, {
      version: 2,
      role: 'device',
      iss: TEST_DEVICE_ACCESS_ISSUER,
      aud: relayAudience,
      sub: 'device_456',
      token_use: 'device_access',
      iat: nowSeconds,
      user_id: 'user_123',
      device_id: 'device_456',
      cert_fingerprint: 'thumbprint-123',
      scopes: ['edge:connect'],
      allowed_skills: ['voice-wake-say'],
      exp: nowSeconds + 60,
    });

    const claims = verifyRelayToken(
      {
        issuers,
        expectedAudience: relayAudience,
        allowedRoles: ['device'],
        expectedConfirmationThumbprint: 'thumbprint-123'
      },
      token
    );
    assert.equal(claims.role, 'device');
    assert.equal(claims.user_id, 'user_123');
    assert.equal(claims.device_id, 'device_456');
    assert.equal(claims.cert_fingerprint, 'thumbprint-123');
    assert.deepEqual(claims.allowed_skills, ['voice-wake-say']);
  });

  it('rejects tampered relay tokens', () => {
    const nowSeconds = Math.floor(Date.now() / 1000);
    const token = signRelayToken(authSigningKey, {
      version: 2,
      role: 'admin',
      iss: TEST_AUTH_ACCESS_ISSUER,
      aud: relayAudience,
      sub: 'admin-user',
      token_use: 'admin_access',
      iat: nowSeconds,
      scopes: ['edge:read'],
      exp: nowSeconds + 60,
    });

    assert.throws(
      () =>
        verifyRelayToken(
          {
            issuers,
            expectedAudience: relayAudience,
            allowedRoles: ['admin']
          },
          `${token}tampered`
        ),
      /invalid_token_signature|invalid_token_format|invalid_token_encoding/
    );
  });

  it('binds execute requests to the authenticated subject', () => {
    const request = bindExecuteRequest(
      {
        tenant_id: 'tenant_a',
        workspace_id: 'workspace_a',
        skill_id: 'voice-wake-say',
        tool: 'voice-wake-say.speak',
        operation: 'speak',
        input: { text: 'hello' },
        run_id: 'run_a',
        task_id: 'task_a',
        step_id: 'step_a',
        attempt: 1
      },
          {
            version: 2,
            role: 'device',
            iss: TEST_DEVICE_ACCESS_ISSUER,
            aud: relayAudience,
            sub: 'device_a',
            token_use: 'device_access',
            iat: Math.floor(Date.now() / 1000),
            scopes: ['edge:connect'],
            user_id: 'user_a',
            device_id: 'device_a',
            allowed_skills: ['voice-wake-say'],
            exp: Math.floor(Date.now() / 1000) + 60,
          },
    );

    assert.equal(request.userId, 'user_a');
    assert.equal(request.deviceId, 'device_a');
    assert.equal(request.tenantId, 'tenant_a');
    assert.equal(request.workspaceId, 'workspace_a');
    assert.equal(request.runId, 'run_a');
    assert.equal(request.taskId, 'task_a');
    assert.equal(request.stepId, 'step_a');
    assert.equal(request.attempt, 1);
    assert.equal(request.tool, 'voice-wake-say.speak');
    assert.equal(request.operation, 'speak');
    assert.deepEqual(request.input, { text: 'hello' });
  });

  it('rejects execute requests that violate token binding', () => {
    assert.throws(
      () =>
        bindExecuteRequest(
          {
            user_id: 'user_b',
            device_id: 'device_a',
            skill_id: 'voice-wake-say',
          },
          {
            version: 2,
            role: 'operator',
            iss: TEST_GATEWAY_SERVICE_ISSUER,
            aud: relayAudience,
            sub: 'brevio-gateway',
            token_use: 'service_access',
            iat: Math.floor(Date.now() / 1000),
            scopes: ['edge:operate'],
            user_id: 'user_a',
            device_id: 'device_a',
            allowed_skills: ['voice-wake-say'],
            exp: Math.floor(Date.now() / 1000) + 60,
          },
        ),
      /user_id does not match relay token/,
    );
  });

  it('encrypts and decrypts queued payloads with bound context', () => {
    const key = deriveSymmetricKey('queue-secret');
    const envelope = protectQueuedInput({ text: 'private note', amount: 42 }, key, 'ctx:user:device:skill');
    const decrypted = recoverQueuedInput(envelope, key, 'ctx:user:device:skill');

    assert.deepEqual(decrypted, { text: 'private note', amount: 42 });
    assert.throws(() => recoverQueuedInput(envelope, key, 'ctx:other'), /Unsupported state or unable to authenticate data|authenticate/i);
  });

  it('redacts session listings into stable references', () => {
    const sessions = buildSessionSummaries(
      [
        {
          userId: 'user_a',
          deviceId: 'device_a',
          connectedAt: Date.UTC(2026, 3, 7, 12, 0, 0),
          lastSeenAt: Date.UTC(2026, 3, 7, 12, 5, 0),
          supportedSkills: ['voice-wake-say', 'apple-remind-me'],
          certFingerprint: 'cert-123',
          authBound: true,
        },
      ],
      'log-salt',
    );

    assert.equal(sessions.length, 1);
    assert.equal(sessions[0].supported_skill_count, 2);
    assert.equal(sessions[0].attested, true);
    assert.equal(sessions[0].auth_bound, true);
    assert.equal('user_id' in sessions[0], false);
    assert.equal('device_id' in sessions[0], false);
  });
});
