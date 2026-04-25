import {
  createHash,
  createHmac,
  createPrivateKey,
  createPublicKey,
  createSign,
  createVerify,
  timingSafeEqual
} from 'node:crypto';
import type { IncomingHttpHeaders } from 'node:http';

export type BrevioEnvironment = 'local' | 'test' | 'staging' | 'production';

export type AccessTokenUse = 'user_access' | 'admin_access' | 'service_access' | 'device_access';

export interface AccessTokenConfirmation {
  'x5t#S256'?: string;
}

export interface AccessTokenClaims {
  version?: 1 | 2;
  sub: string;
  iss: string;
  aud: string;
  iat: number;
  exp: number;
  token_use?: AccessTokenUse;
  scopes?: string[];
  role?: string;
  service?: string;
  workspace_id?: string;
  tenant_id?: string;
  cnf?: AccessTokenConfirmation;
}

export interface VerifyAccessTokenPolicy {
  expectedAudience: string | string[];
  expectedIssuer?: string;
  allowedTokenUses?: AccessTokenUse[];
  requiredScopes?: string[];
  expectedConfirmationThumbprint?: string;
}

export interface VerifiedAccessToken extends AccessTokenClaims {
  version: 1 | 2;
  token_use: AccessTokenUse;
  scopes: string[];
}

export interface CallerContextClaims {
  version?: 1;
  subject: string;
  user_id: string;
  workspace_id?: string;
  tenant_id?: string;
  channel?: string;
  channel_subject?: string;
  auth_strength: 'webhook_signature' | 'api_key' | 'service_token' | 'admin_token';
  provenance: string;
  issued_at: string;
  expires_at: string;
}

export interface VerifiedCallerContext extends CallerContextClaims {
  version: 1;
}

interface TokenHeader {
  alg: 'HS256' | 'RS256';
  typ: string;
  kid: string;
}

const ACCESS_TOKEN_TYP = 'brevio-access+jwt';
const CALLER_CONTEXT_TYP = 'brevio-caller-context+jwt';

function encodeBase64Url(value: Buffer | string): string {
  return Buffer.isBuffer(value) ? value.toString('base64url') : Buffer.from(value, 'utf8').toString('base64url');
}

function decodeBase64Url(value: string): Buffer {
  return Buffer.from(value, 'base64url');
}

export function normalizeString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export function extractHeaderValue(headers: IncomingHttpHeaders | Record<string, string | string[] | undefined>, key: string): string | undefined {
  const value = headers[key.toLowerCase()];
  if (typeof value === 'string') {
    return value.trim() || undefined;
  }
  if (Array.isArray(value) && value.length > 0) {
    return value[0]?.trim() || undefined;
  }
  return undefined;
}

export function extractBearerToken(authorizationHeader: string | undefined): string | undefined {
  const normalized = normalizeString(authorizationHeader);
  if (!normalized || !normalized.toLowerCase().startsWith('bearer ')) {
    return undefined;
  }
  return normalized.slice(7).trim() || undefined;
}

export function loadBrevioEnvironment(
  rawEnvironment = process.env.BREVIO_ENV,
  rawNodeEnvironment = process.env.NODE_ENV
): BrevioEnvironment {
  const explicitEnvironment = rawEnvironment?.trim().toLowerCase();
  const fallbackEnvironment = rawNodeEnvironment?.trim().toLowerCase();
  const normalized = explicitEnvironment || fallbackEnvironment || 'local';
  if (!explicitEnvironment && fallbackEnvironment && !['local', 'development', 'dev', 'test'].includes(fallbackEnvironment)) {
    throw new Error('BREVIO_ENV is required outside local/test environments');
  }
  switch (normalized) {
    case 'local':
    case 'development':
    case 'dev':
      return 'local';
    case 'test':
      return 'test';
    case 'staging':
    case 'stage':
      return 'staging';
    case 'production':
    case 'prod':
      return 'production';
    default:
      throw new Error(`unsupported BREVIO_ENV: ${normalized}`);
  }
}

export function isPermissiveEnvironment(environment: BrevioEnvironment): boolean {
  return environment === 'local' || environment === 'test';
}

export function requireSharedSecret(
  rawSecret: string | undefined,
  fieldName: string,
  environment: BrevioEnvironment,
  devSeed?: string
): string {
  const normalized = normalizeString(rawSecret);
  if (normalized) {
    return normalized;
  }
  if (isPermissiveEnvironment(environment)) {
    return createHash('sha256')
      .update(`${fieldName}:${environment}:${devSeed ?? 'brevio-dev-secret'}`)
      .digest('hex');
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function stableSerialize(value: unknown): string {
  if (value === null || value === undefined) {
    return 'null';
  }
  if (typeof value !== 'object') {
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((entry) => stableSerialize(entry)).join(',')}]`;
  }
  const entries = Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left.localeCompare(right));
  return `{${entries.map(([key, entry]) => `${JSON.stringify(key)}:${stableSerialize(entry)}`).join(',')}}`;
}

export function pseudonymizedRef(value: string | undefined, salt: string, length = 16): string | undefined {
  const normalized = normalizeString(value);
  if (!normalized) {
    return undefined;
  }
  return createHash('sha256').update(`${salt}:${normalized}`).digest('hex').slice(0, length);
}

function parseSignedToken(token: string): { header: TokenHeader; payload: Record<string, unknown>; signingInput: string; signature: Buffer } {
  const parts = token.trim().split('.');
  if (parts.length !== 3) {
    throw new Error('invalid_token_format');
  }

  let header: TokenHeader;
  let payload: Record<string, unknown>;
  try {
    header = JSON.parse(decodeBase64Url(parts[0]).toString('utf8')) as TokenHeader;
    payload = JSON.parse(decodeBase64Url(parts[1]).toString('utf8')) as Record<string, unknown>;
  } catch {
    throw new Error('invalid_token_encoding');
  }
  return {
    header,
    payload,
    signingInput: `${parts[0]}.${parts[1]}`,
    signature: decodeBase64Url(parts[2])
  };
}

function normalizeKeyMaterial(keyMaterial: string): string {
  return keyMaterial.includes('BEGIN ') ? keyMaterial.replace(/\\n/g, '\n').trim() : keyMaterial.trim();
}

function isPemPrivateKey(keyMaterial: string): boolean {
  return normalizeKeyMaterial(keyMaterial).includes('BEGIN PRIVATE KEY');
}

function isPemKey(keyMaterial: string): boolean {
  const normalized = normalizeKeyMaterial(keyMaterial);
  return normalized.includes('BEGIN PRIVATE KEY') || normalized.includes('BEGIN PUBLIC KEY');
}

function signSignedToken(keyMaterial: string, header: TokenHeader, payload: Record<string, unknown>): string {
  const headerBase64 = encodeBase64Url(JSON.stringify(header));
  const payloadBase64 = encodeBase64Url(JSON.stringify(payload));
  const signingInput = `${headerBase64}.${payloadBase64}`;
  let signature: Buffer;
  if (header.alg === 'RS256') {
    const signer = createSign('RSA-SHA256');
    signer.update(signingInput);
    signer.end();
    signature = signer.sign(createPrivateKey(normalizeKeyMaterial(keyMaterial)));
  } else {
    signature = createHmac('sha256', keyMaterial).update(signingInput).digest();
  }
  return `${signingInput}.${encodeBase64Url(signature)}`;
}

function verifySignedToken(keyMaterial: string, token: string, expectedTyp: string): Record<string, unknown> {
  const parsed = parseSignedToken(token);
  if ((parsed.header.alg !== 'HS256' && parsed.header.alg !== 'RS256') || parsed.header.typ !== expectedTyp) {
    throw new Error('unsupported_token_type');
  }
  if (parsed.header.alg === 'RS256') {
    const verifier = createVerify('RSA-SHA256');
    verifier.update(parsed.signingInput);
    verifier.end();
    const verificationKey = isPemPrivateKey(keyMaterial)
      ? createPublicKey(createPrivateKey(normalizeKeyMaterial(keyMaterial)))
      : createPublicKey(normalizeKeyMaterial(keyMaterial));
    if (!verifier.verify(verificationKey, parsed.signature)) {
      throw new Error('invalid_token_signature');
    }
  } else {
    const expectedSignature = createHmac('sha256', keyMaterial).update(parsed.signingInput).digest();
    if (parsed.signature.length !== expectedSignature.length || !timingSafeEqual(parsed.signature, expectedSignature)) {
      throw new Error('invalid_token_signature');
    }
  }
  return parsed.payload;
}

export function resolveAccessTokenSigningKey(
  rawPrivateKey: string | undefined,
  rawSharedSecret: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  devSeed?: string
): string {
  const privateKey = normalizeString(rawPrivateKey);
  if (privateKey) {
    return normalizeKeyMaterial(privateKey);
  }
  const sharedSecret = normalizeString(rawSharedSecret);
  const allowSharedSecretFallback =
    isPermissiveEnvironment(environment) || process.env.BREVIO_ALLOW_SHARED_INTERNAL_AUTH_SECRET?.trim() === 'true';
  if (sharedSecret && allowSharedSecretFallback) {
    return sharedSecret;
  }
  if (isPermissiveEnvironment(environment) && !sharedSecret) {
    return requireSharedSecret(undefined, fieldName, environment, devSeed);
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function resolveAccessTokenVerificationKey(
  rawPublicKey: string | undefined,
  rawPrivateKey: string | undefined,
  rawSharedSecret: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  devSeed?: string
): string {
  const publicKey = normalizeString(rawPublicKey);
  if (publicKey) {
    return normalizeKeyMaterial(publicKey);
  }
  const privateKey = normalizeString(rawPrivateKey);
  if (privateKey) {
    return normalizeKeyMaterial(privateKey);
  }
  const sharedSecret = normalizeString(rawSharedSecret);
  const allowSharedSecretFallback =
    isPermissiveEnvironment(environment) || process.env.BREVIO_ALLOW_SHARED_INTERNAL_AUTH_SECRET?.trim() === 'true';
  if (sharedSecret && allowSharedSecretFallback) {
    return sharedSecret;
  }
  if (isPermissiveEnvironment(environment) && !sharedSecret) {
    return requireSharedSecret(undefined, fieldName, environment, devSeed);
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function signAccessToken(secret: string, claims: AccessTokenClaims, keyId = 'brevio-internal-v2'): string {
  const normalizedClaims: AccessTokenClaims = {
    ...claims,
    version: claims.version ?? 2,
    token_use: claims.token_use ?? 'service_access',
    scopes: [...new Set((claims.scopes ?? []).map((scope) => scope.trim()).filter((scope) => scope.length > 0))]
  };
  return signSignedToken(
    secret,
    { alg: isPemPrivateKey(secret) ? 'RS256' : 'HS256', typ: ACCESS_TOKEN_TYP, kid: keyId },
    normalizedClaims as Record<string, unknown>
  );
}

export function verifyAccessToken(
  secret: string,
  token: string,
  policy: VerifyAccessTokenPolicy,
  nowMs = Date.now()
): VerifiedAccessToken {
  const payload = verifySignedToken(secret, token, ACCESS_TOKEN_TYP);
  const claims = payload as AccessTokenClaims;
  const allowLegacyTokens = process.env.BREVIO_ALLOW_LEGACY_INTERNAL_TOKENS?.trim() === 'true';
  const version = claims.version === 1 || claims.version === 2 ? claims.version : 1;
  const tokenUse = claims.token_use ?? (allowLegacyTokens
    ? typeof claims.role === 'string' && claims.role.toLowerCase().includes('admin')
      ? 'admin_access'
      : 'service_access'
    : undefined);
  if (!allowLegacyTokens && version !== 2) {
    throw new Error('token_version_mismatch');
  }
  if (!tokenUse) {
    throw new Error('token_use_required');
  }
  if (!allowLegacyTokens && !isPemKey(secret) && !isPermissiveEnvironment(loadBrevioEnvironment())) {
    throw new Error('shared_secret_tokens_not_allowed');
  }
  const scopes = Array.isArray(claims.scopes)
    ? claims.scopes.map((scope) => normalizeString(scope)).filter((scope): scope is string => Boolean(scope))
    : [];
  if (!normalizeString(claims.sub) || !normalizeString(claims.iss) || !normalizeString(claims.aud)) {
    throw new Error('invalid_token_claims');
  }
  if (typeof claims.iat !== 'number' || typeof claims.exp !== 'number') {
    throw new Error('invalid_token_timestamps');
  }
  if (claims.exp * 1000 <= nowMs) {
    throw new Error('token_expired');
  }
  const expectedAudiences = Array.isArray(policy.expectedAudience) ? policy.expectedAudience : [policy.expectedAudience];
  if (!expectedAudiences.includes(claims.aud)) {
    throw new Error('audience_mismatch');
  }
  if (policy.expectedIssuer && claims.iss !== policy.expectedIssuer) {
    throw new Error('issuer_mismatch');
  }
  if (policy.allowedTokenUses && !policy.allowedTokenUses.includes(tokenUse)) {
    throw new Error('token_use_mismatch');
  }
  if (policy.requiredScopes && policy.requiredScopes.some((scope) => !scopes.includes(scope))) {
    throw new Error('scope_mismatch');
  }
  const confirmationThumbprint = normalizeString(claims.cnf?.['x5t#S256']);
  if (policy.expectedConfirmationThumbprint && confirmationThumbprint !== policy.expectedConfirmationThumbprint) {
    throw new Error('confirmation_mismatch');
  }
  return {
    ...claims,
    version,
    token_use: tokenUse,
    scopes,
    sub: claims.sub.trim(),
    iss: claims.iss.trim(),
    aud: claims.aud.trim()
  };
}

export function signCallerContextEnvelope(secret: string, claims: CallerContextClaims, keyId = 'brevio-caller-v1'): string {
  const normalizedClaims: VerifiedCallerContext = {
    ...claims,
    version: 1,
    subject: claims.subject.trim(),
    user_id: claims.user_id.trim()
  };
  return signSignedToken(secret, { alg: 'HS256', typ: CALLER_CONTEXT_TYP, kid: keyId }, normalizedClaims as Record<string, unknown>);
}

export function verifyCallerContextEnvelope(secret: string, token: string, nowMs = Date.now()): VerifiedCallerContext {
  const payload = verifySignedToken(secret, token, CALLER_CONTEXT_TYP);
  const claims = payload as CallerContextClaims;
  const issuedAt = Date.parse(claims.issued_at);
  const expiresAt = Date.parse(claims.expires_at);
  if (!normalizeString(claims.subject) || !normalizeString(claims.user_id) || !normalizeString(claims.provenance)) {
    throw new Error('invalid_caller_context_claims');
  }
  if (!Number.isFinite(issuedAt) || !Number.isFinite(expiresAt) || expiresAt <= nowMs) {
    throw new Error('caller_context_expired');
  }
  return {
    ...claims,
    version: 1,
    subject: claims.subject.trim(),
    user_id: claims.user_id.trim()
  };
}
