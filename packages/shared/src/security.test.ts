import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  hashTokenBinding,
  loadBrevioEnvironment,
  resolveAccessTokenSigningKey,
  resolveAccessTokenVerificationKey,
  resolveCallerContextSigningKey,
  resolveCallerContextVerificationKey,
  signAccessToken,
  signCallerContextEnvelope,
  verifyAccessToken,
  verifyCallerContextEnvelope
} from './security.ts';

describe('shared security', () => {
  it('requires explicit BREVIO_ENV outside local/test mappings', () => {
    assert.equal(loadBrevioEnvironment(undefined, 'test'), 'test');
    assert.throws(() => loadBrevioEnvironment(undefined, 'development'), /BREVIO_ENV is required/);
    assert.throws(() => loadBrevioEnvironment(undefined, 'production'), /BREVIO_ENV is required/);
  });

  it('signs and verifies access tokens through issuer registries', () => {
    const privateKey = resolveAccessTokenSigningKey(undefined, undefined, 'test', 'TEST_GATEWAY_SERVICE_PRIVATE_KEY', 'gateway-service');
    const publicKey = resolveAccessTokenVerificationKey(undefined, undefined, undefined, 'test', 'TEST_GATEWAY_SERVICE_PUBLIC_KEY', 'gateway-service');
    const token = signAccessToken(privateKey, {
      sub: 'gateway-service',
      iss: 'https://gateway.brevio.internal',
      aud: 'brevio-temporal-worker',
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 60,
      token_use: 'service_access',
      version: 2
    });

    const verified = verifyAccessToken(buildAccessTokenIssuerRegistry([
      {
        issuer: 'https://gateway.brevio.internal',
        verificationKey: publicKey,
        allowedTokenUses: ['service_access']
      }
    ]), token, {
      expectedAudience: 'brevio-temporal-worker',
      expectedIssuer: 'https://gateway.brevio.internal',
      allowedTokenUses: ['service_access']
    });

    assert.equal(verified.sub, 'gateway-service');
    assert.equal(verified.token_use, 'service_access');
    assert.equal(verified.version, 2);
  });

  it('verifies caller-context envelopes with audience and token binding', () => {
    const accessToken = signAccessToken(
      resolveAccessTokenSigningKey(undefined, undefined, 'test', 'TEST_GATEWAY_SERVICE_PRIVATE_KEY', 'gateway-service'),
      {
        sub: 'brevio-gateway',
        iss: 'https://gateway.brevio.internal',
        aud: 'brevio-hands',
        iat: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + 60,
        token_use: 'service_access',
        scopes: ['hands:execute']
      }
    );
    const callerContext = signCallerContextEnvelope(
      resolveCallerContextSigningKey(undefined, 'test', 'TEST_GATEWAY_CALLER_CONTEXT_PRIVATE_KEY', 'gateway-caller-context'),
      {
        version: 2,
        iss: 'https://gateway.brevio.internal/caller-context',
        aud: 'brevio-hands',
        sub: 'user-123',
        user_id: 'user-123',
        actor_service: 'brevio-gateway',
        auth_strength: 'service_token',
        provenance: 'gateway:test',
        jti: 'ctx-1',
        iat: Math.floor(Date.now() / 1000),
        nbf: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + 60,
        ath: hashTokenBinding(accessToken)
      }
    );

    const verified = verifyCallerContextEnvelope(
      buildCallerContextIssuerRegistry([
        {
          issuer: 'https://gateway.brevio.internal/caller-context',
          verificationKey: resolveCallerContextVerificationKey(undefined, 'test', 'TEST_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY', 'gateway-caller-context')
        }
      ]),
      callerContext,
      {
        expectedAudience: 'brevio-hands',
        expectedAccessTokenHash: hashTokenBinding(accessToken)
      }
    );

    assert.equal(verified.user_id, 'user-123');
    assert.equal(verified.actor_service, 'brevio-gateway');
  });
});
