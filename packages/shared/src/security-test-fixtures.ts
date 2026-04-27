import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  hashTokenBinding,
  resolveAccessTokenSigningKey,
  resolveAccessTokenVerificationKey,
  resolveCallerContextSigningKey,
  resolveCallerContextVerificationKey,
  signAccessToken,
  signCallerContextEnvelope,
  type AccessTokenClaims,
  type AccessTokenIssuerRegistry,
  type CallerContextClaims,
  type CallerContextIssuerRegistry
} from './security.js';

export const TEST_AUTH_ACCESS_ISSUER = 'https://auth.brevio.internal';
export const TEST_GATEWAY_SERVICE_ISSUER = 'https://gateway.brevio.internal';
export const TEST_GATEWAY_CALLER_CONTEXT_ISSUER = 'https://gateway.brevio.internal/caller-context';
export const TEST_DEVICE_ACCESS_ISSUER = 'https://device-auth.brevio.internal';

const TEST_ENVIRONMENT = 'test';

function authSigningKey(): string {
  return resolveAccessTokenSigningKey(undefined, undefined, TEST_ENVIRONMENT, 'TEST_AUTH_ACCESS_PRIVATE_KEY', 'auth-access');
}

function gatewayServiceSigningKey(): string {
  return resolveAccessTokenSigningKey(undefined, undefined, TEST_ENVIRONMENT, 'TEST_GATEWAY_SERVICE_PRIVATE_KEY', 'gateway-service');
}

function deviceSigningKey(): string {
  return resolveAccessTokenSigningKey(undefined, undefined, TEST_ENVIRONMENT, 'TEST_DEVICE_ACCESS_PRIVATE_KEY', 'device-access');
}

function callerContextSigningKey(): string {
  return resolveCallerContextSigningKey(undefined, TEST_ENVIRONMENT, 'TEST_GATEWAY_CALLER_CONTEXT_PRIVATE_KEY', 'gateway-caller-context');
}

export function testAccessTokenIssuers(): AccessTokenIssuerRegistry {
  return buildAccessTokenIssuerRegistry([
    {
      issuer: TEST_AUTH_ACCESS_ISSUER,
      verificationKey: resolveAccessTokenVerificationKey(undefined, undefined, undefined, TEST_ENVIRONMENT, 'TEST_AUTH_ACCESS_PUBLIC_KEY', 'auth-access'),
      allowedTokenUses: ['user_access', 'admin_access']
    },
    {
      issuer: TEST_GATEWAY_SERVICE_ISSUER,
      verificationKey: resolveAccessTokenVerificationKey(undefined, undefined, undefined, TEST_ENVIRONMENT, 'TEST_GATEWAY_SERVICE_PUBLIC_KEY', 'gateway-service'),
      allowedTokenUses: ['service_access']
    },
    {
      issuer: TEST_DEVICE_ACCESS_ISSUER,
      verificationKey: resolveAccessTokenVerificationKey(undefined, undefined, undefined, TEST_ENVIRONMENT, 'TEST_DEVICE_ACCESS_PUBLIC_KEY', 'device-access'),
      allowedTokenUses: ['device_access']
    }
  ]);
}

export function testCallerContextIssuers(): CallerContextIssuerRegistry {
  return buildCallerContextIssuerRegistry([
    {
      issuer: TEST_GATEWAY_CALLER_CONTEXT_ISSUER,
      verificationKey: resolveCallerContextVerificationKey(undefined, TEST_ENVIRONMENT, 'TEST_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY', 'gateway-caller-context')
    }
  ]);
}

export function signTestAuthAccessToken(
  claims: Omit<AccessTokenClaims, 'iss' | 'iat' | 'exp'> & { iat?: number; exp?: number }
): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signAccessToken(authSigningKey(), {
    ...claims,
    iss: TEST_AUTH_ACCESS_ISSUER,
    iat: claims.iat ?? nowSeconds,
    exp: claims.exp ?? nowSeconds + 300
  });
}

export function signTestGatewayServiceToken(
  claims: Omit<AccessTokenClaims, 'iss' | 'iat' | 'exp' | 'token_use'> & { iat?: number; exp?: number }
): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signAccessToken(gatewayServiceSigningKey(), {
    ...claims,
    iss: TEST_GATEWAY_SERVICE_ISSUER,
    token_use: 'service_access',
    iat: claims.iat ?? nowSeconds,
    exp: claims.exp ?? nowSeconds + 300
  });
}

export function signTestDeviceAccessToken(
  claims: Omit<AccessTokenClaims, 'iss' | 'iat' | 'exp' | 'token_use'> & { iat?: number; exp?: number }
): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signAccessToken(deviceSigningKey(), {
    ...claims,
    iss: TEST_DEVICE_ACCESS_ISSUER,
    token_use: 'device_access',
    iat: claims.iat ?? nowSeconds,
    exp: claims.exp ?? nowSeconds + 300
  });
}

export function signTestCallerContext(
  claims: Omit<CallerContextClaims, 'iss' | 'iat' | 'exp' | 'ath' | 'jti'> & {
    accessToken: string;
    iat?: number;
    exp?: number;
    nbf?: number;
    jti?: string;
  }
): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signCallerContextEnvelope(callerContextSigningKey(), {
    ...claims,
    iss: TEST_GATEWAY_CALLER_CONTEXT_ISSUER,
    iat: claims.iat ?? nowSeconds,
    exp: claims.exp ?? nowSeconds + 300,
    nbf: claims.nbf,
    jti: claims.jti ?? `jti-${nowSeconds}-${Math.random().toString(16).slice(2)}`,
    ath: hashTokenBinding(claims.accessToken)
  });
}
