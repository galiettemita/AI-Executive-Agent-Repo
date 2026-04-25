import assert from 'node:assert/strict';
import { createHmac, generateKeyPairSync } from 'node:crypto';
import { describe, it } from 'node:test';

import { loadBrevioEnvironment, signAccessToken, verifyAccessToken } from './security.ts';

function encodeBase64Url(value: string): string {
  return Buffer.from(value, 'utf8').toString('base64url');
}

function signLegacyToken(secret: string, claims: Record<string, unknown>): string {
  const header = encodeBase64Url(JSON.stringify({ alg: 'HS256', typ: 'brevio-access+jwt', kid: 'legacy' }));
  const payload = encodeBase64Url(JSON.stringify(claims));
  const signingInput = `${header}.${payload}`;
  const signature = createHmac('sha256', secret).update(signingInput).digest('base64url');
  return `${signingInput}.${signature}`;
}

describe('shared security', () => {
  it('requires explicit BREVIO_ENV outside local/test mappings', () => {
    assert.equal(loadBrevioEnvironment(undefined, 'development'), 'local');
    assert.equal(loadBrevioEnvironment(undefined, 'test'), 'test');
    assert.throws(() => loadBrevioEnvironment(undefined, 'production'), /BREVIO_ENV is required/);
  });

  it('signs and verifies access tokens with RSA key material', () => {
    const { privateKey, publicKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
    const token = signAccessToken(
      privateKey.export({ type: 'pkcs8', format: 'pem' }).toString(),
      {
        sub: 'gateway-service',
        iss: 'https://auth.brevio.internal',
        aud: 'brevio-temporal-worker',
        iat: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + 60,
        token_use: 'service_access',
        version: 2
      },
      'rsa-test'
    );

    const verified = verifyAccessToken(publicKey.export({ type: 'spki', format: 'pem' }).toString(), token, {
      expectedAudience: 'brevio-temporal-worker',
      expectedIssuer: 'https://auth.brevio.internal',
      allowedTokenUses: ['service_access']
    });

    assert.equal(verified.sub, 'gateway-service');
    assert.equal(verified.token_use, 'service_access');
    assert.equal(verified.version, 2);
  });

  it('rejects legacy internal tokens when compatibility mode is disabled', () => {
    const priorFlag = process.env.BREVIO_ALLOW_LEGACY_INTERNAL_TOKENS;
    delete process.env.BREVIO_ALLOW_LEGACY_INTERNAL_TOKENS;
    try {
      const token = signLegacyToken('shared-secret', {
        sub: 'legacy-service',
        iss: 'https://auth.brevio.internal',
        aud: 'brevio-brain',
        iat: Math.floor(Date.now() / 1000),
        exp: Math.floor(Date.now() / 1000) + 60,
        version: 1
      });

      assert.throws(
        () =>
          verifyAccessToken('shared-secret', token, {
            expectedAudience: 'brevio-brain',
            expectedIssuer: 'https://auth.brevio.internal',
            allowedTokenUses: ['service_access']
          }),
        /token_version_mismatch/
      );
    } finally {
      if (priorFlag === undefined) {
        delete process.env.BREVIO_ALLOW_LEGACY_INTERNAL_TOKENS;
      } else {
        process.env.BREVIO_ALLOW_LEGACY_INTERNAL_TOKENS = priorFlag;
      }
    }
  });
});
