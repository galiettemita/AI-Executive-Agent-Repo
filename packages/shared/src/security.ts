import {
  createHash,
  createPrivateKey,
  createPublicKey,
  createSign,
  createVerify,
  generateKeyPairSync
} from 'node:crypto';
import type { IncomingHttpHeaders } from 'node:http';

export type BrevioEnvironment = 'local' | 'test' | 'staging' | 'production';

export type AccessTokenUse = 'user_access' | 'admin_access' | 'service_access' | 'device_access';

export interface AccessTokenConfirmation {
  'x5t#S256'?: string;
  jkt?: string;
}

export interface AccessTokenClaims {
  version?: 2;
  sub: string;
  iss: string;
  aud: string;
  iat: number;
  exp: number;
  token_use: AccessTokenUse;
  scopes?: string[];
  service?: string;
  workspace_id?: string;
  tenant_id?: string;
  cnf?: AccessTokenConfirmation;
}

export interface VerifyAccessTokenPolicy {
  expectedAudience: string | string[];
  expectedIssuer?: string | string[];
  allowedTokenUses?: AccessTokenUse[];
  requiredScopes?: string[];
  expectedConfirmationThumbprint?: string;
}

export interface VerifiedAccessToken extends AccessTokenClaims {
  version: 2;
  scopes: string[];
}

export interface AccessTokenIssuerProfile {
  verificationKey: string;
  allowedTokenUses: AccessTokenUse[];
}

export type AccessTokenIssuerRegistry = Record<string, AccessTokenIssuerProfile>;

export interface CallerContextClaims {
  version?: 2;
  iss: string;
  aud: string;
  sub: string;
  user_id: string;
  workspace_id?: string;
  tenant_id?: string;
  actor_service: string;
  channel?: string;
  channel_subject?: string;
  auth_strength: 'webhook_signature' | 'api_key' | 'service_token' | 'admin_token';
  provenance: string;
  jti: string;
  iat: number;
  nbf?: number;
  exp: number;
  ath: string;
}

export interface VerifiedCallerContext extends CallerContextClaims {
  version: 2;
}

export interface CallerContextIssuerProfile {
  verificationKey: string;
}

export type CallerContextIssuerRegistry = Record<string, CallerContextIssuerProfile>;

export interface VerifyCallerContextPolicy {
  expectedAudience: string;
  expectedIssuer?: string | string[];
  expectedAccessTokenHash?: string;
  maxClockSkewSeconds?: number;
  preventReplay?: boolean;
}

interface TokenHeader {
  alg: 'RS256';
  typ: string;
  kid: string;
}

const ACCESS_TOKEN_TYPS: Record<AccessTokenUse, string> = {
  user_access: 'brevio-user+jwt',
  admin_access: 'brevio-admin+jwt',
  service_access: 'brevio-service+jwt',
  device_access: 'brevio-device+jwt'
};
const CALLER_CONTEXT_TYP = 'brevio-caller-context-v2+jwt';
const replayCache = new Map<string, number>();
const developmentKeyPairs = new Map<string, { privateKey: string; publicKey: string }>();

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

function runningUnderNodeTest(): boolean {
  return process.execArgv.includes('--test') || process.argv.includes('--test');
}

export function loadBrevioEnvironment(
  rawEnvironment = process.env.BREVIO_ENV,
  rawNodeEnvironment = process.env.NODE_ENV
): BrevioEnvironment {
  const explicitEnvironment = rawEnvironment?.trim().toLowerCase();
  const nodeEnvironment = rawNodeEnvironment?.trim().toLowerCase();
  if (!explicitEnvironment) {
    if (nodeEnvironment === 'test' || runningUnderNodeTest()) {
      return 'test';
    }
    throw new Error('BREVIO_ENV is required outside test processes');
  }
  switch (explicitEnvironment) {
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
      throw new Error(`unsupported BREVIO_ENV: ${explicitEnvironment}`);
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

function normalizeExpectedStrings(value: string | string[] | undefined): string[] | undefined {
  if (!value) {
    return undefined;
  }
  return (Array.isArray(value) ? value : [value])
    .map((entry) => normalizeString(entry))
    .filter((entry): entry is string => Boolean(entry));
}

function getDevelopmentKeyPair(purpose: string): { privateKey: string; publicKey: string } {
  const normalizedPurpose = purpose.trim().toLowerCase() || 'brevio-dev';
  const existing = developmentKeyPairs.get(normalizedPurpose);
  if (existing) {
    return existing;
  }
  const pair = generateKeyPairSync('rsa', {
    modulusLength: 2048,
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
    publicKeyEncoding: { type: 'spki', format: 'pem' }
  });
  const generated = { privateKey: pair.privateKey, publicKey: pair.publicKey };
  developmentKeyPairs.set(normalizedPurpose, generated);
  return generated;
}

function signSignedToken(privateKey: string, typ: string, payload: Record<string, unknown>, keyId: string): string {
  if (!isPemPrivateKey(privateKey)) {
    throw new Error('signing_key_must_be_private_key');
  }
  const header: TokenHeader = { alg: 'RS256', typ, kid: keyId };
  const headerBase64 = encodeBase64Url(JSON.stringify(header));
  const payloadBase64 = encodeBase64Url(JSON.stringify(payload));
  const signingInput = `${headerBase64}.${payloadBase64}`;
  const signer = createSign('RSA-SHA256');
  signer.update(signingInput);
  signer.end();
  const signature = signer.sign(createPrivateKey(normalizeKeyMaterial(privateKey)));
  return `${signingInput}.${encodeBase64Url(signature)}`;
}

function verifySignature(verificationKey: string, parsed: { header: TokenHeader; signingInput: string; signature: Buffer }): void {
  const verifier = createVerify('RSA-SHA256');
  verifier.update(parsed.signingInput);
  verifier.end();
  const publicKey = createPublicKey(normalizeKeyMaterial(verificationKey));
  if (!verifier.verify(publicKey, parsed.signature)) {
    throw new Error('invalid_token_signature');
  }
}

function resolveIssuerProfile<T extends { verificationKey: string }>(
  issuer: string,
  registry: Record<string, T>
): T {
  const profile = registry[issuer];
  if (!profile) {
    throw new Error('issuer_not_registered');
  }
  return profile;
}

function accessTokenTypFor(tokenUse: AccessTokenUse): string {
  return ACCESS_TOKEN_TYPS[tokenUse];
}

export function hashTokenBinding(token: string): string {
  return createHash('sha256').update(token).digest('base64url');
}

export function buildAccessTokenIssuerRegistry(
  entries: Array<{ issuer: string; verificationKey: string; allowedTokenUses: AccessTokenUse[] }>
): AccessTokenIssuerRegistry {
  return Object.fromEntries(
    entries.map((entry) => [
      entry.issuer,
      {
        verificationKey: normalizeKeyMaterial(entry.verificationKey),
        allowedTokenUses: [...new Set(entry.allowedTokenUses)]
      }
    ])
  );
}

export function buildCallerContextIssuerRegistry(
  entries: Array<{ issuer: string; verificationKey: string }>
): CallerContextIssuerRegistry {
  return Object.fromEntries(
    entries.map((entry) => [entry.issuer, { verificationKey: normalizeKeyMaterial(entry.verificationKey) }])
  );
}

export function resolveAccessTokenSigningKey(
  rawPrivateKey: string | undefined,
  _rawLegacySharedSecret: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  purpose = 'brevio-access'
): string {
  const privateKey = normalizeString(rawPrivateKey);
  if (privateKey) {
    return normalizeKeyMaterial(privateKey);
  }
  if (isPermissiveEnvironment(environment)) {
    return getDevelopmentKeyPair(purpose).privateKey;
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function resolveAccessTokenVerificationKey(
  rawPublicKey: string | undefined,
  _rawLegacyPrivateKey: string | undefined,
  _rawLegacySharedSecret: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  purpose = 'brevio-access'
): string {
  const publicKey = normalizeString(rawPublicKey);
  if (publicKey) {
    return normalizeKeyMaterial(publicKey);
  }
  if (isPermissiveEnvironment(environment)) {
    return getDevelopmentKeyPair(purpose).publicKey;
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function resolveCallerContextSigningKey(
  rawPrivateKey: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  purpose = 'gateway-caller-context'
): string {
  const privateKey = normalizeString(rawPrivateKey);
  if (privateKey) {
    return normalizeKeyMaterial(privateKey);
  }
  if (isPermissiveEnvironment(environment)) {
    return getDevelopmentKeyPair(purpose).privateKey;
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function resolveCallerContextVerificationKey(
  rawPublicKey: string | undefined,
  environment: BrevioEnvironment,
  fieldName: string,
  purpose = 'gateway-caller-context'
): string {
  const publicKey = normalizeString(rawPublicKey);
  if (publicKey) {
    return normalizeKeyMaterial(publicKey);
  }
  if (isPermissiveEnvironment(environment)) {
    return getDevelopmentKeyPair(purpose).publicKey;
  }
  throw new Error(`${fieldName} is required outside local/test environments`);
}

export function signAccessToken(privateKey: string, claims: AccessTokenClaims, keyId = 'brevio-access-v2'): string {
  if (!claims.token_use) {
    throw new Error('token_use_required');
  }
  const normalizedClaims: VerifiedAccessToken = {
    ...claims,
    version: 2,
    token_use: claims.token_use,
    scopes: [...new Set((claims.scopes ?? []).map((scope) => scope.trim()).filter((scope) => scope.length > 0))]
  };
  return signSignedToken(privateKey, accessTokenTypFor(normalizedClaims.token_use), normalizedClaims as Record<string, unknown>, keyId);
}

export function verifyAccessToken(
  issuers: AccessTokenIssuerRegistry,
  token: string,
  policy: VerifyAccessTokenPolicy,
  nowMs = Date.now()
): VerifiedAccessToken {
  const parsed = parseSignedToken(token);
  if (parsed.header.alg !== 'RS256') {
    throw new Error('unsupported_token_type');
  }
  const claims = parsed.payload as Partial<AccessTokenClaims>;
  const issuer = normalizeString(claims.iss);
  const subject = normalizeString(claims.sub);
  const audience = normalizeString(claims.aud);
  if (!issuer || !subject || !audience) {
    throw new Error('invalid_token_claims');
  }
  const issuerProfile = resolveIssuerProfile(issuer, issuers);
  verifySignature(issuerProfile.verificationKey, parsed);
  if (claims.version !== 2) {
    throw new Error('token_version_mismatch');
  }
  if (!claims.token_use) {
    throw new Error('token_use_required');
  }
  if (!issuerProfile.allowedTokenUses.includes(claims.token_use)) {
    throw new Error('issuer_token_use_mismatch');
  }
  if (parsed.header.typ !== accessTokenTypFor(claims.token_use)) {
    throw new Error('token_type_mismatch');
  }
  const expectedIssuers = normalizeExpectedStrings(policy.expectedIssuer);
  if (expectedIssuers && !expectedIssuers.includes(issuer)) {
    throw new Error('issuer_mismatch');
  }
  const expectedAudiences = normalizeExpectedStrings(policy.expectedAudience) ?? [];
  if (!expectedAudiences.includes(audience)) {
    throw new Error('audience_mismatch');
  }
  if (typeof claims.iat !== 'number' || typeof claims.exp !== 'number') {
    throw new Error('invalid_token_timestamps');
  }
  if (claims.exp * 1000 <= nowMs) {
    throw new Error('token_expired');
  }
  const scopes = Array.isArray(claims.scopes)
    ? claims.scopes.map((scope) => normalizeString(scope)).filter((scope): scope is string => Boolean(scope))
    : [];
  if (policy.allowedTokenUses && !policy.allowedTokenUses.includes(claims.token_use)) {
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
    version: 2,
    token_use: claims.token_use,
    scopes,
    sub: subject,
    iss: issuer,
    aud: audience
  } as VerifiedAccessToken;
}

export function signCallerContextEnvelope(
  privateKey: string,
  claims: CallerContextClaims,
  keyId = 'brevio-caller-context-v2'
): string {
  const normalizedClaims: VerifiedCallerContext = {
    ...claims,
    version: 2,
    iss: claims.iss.trim(),
    aud: claims.aud.trim(),
    sub: claims.sub.trim(),
    user_id: claims.user_id.trim(),
    actor_service: claims.actor_service.trim(),
    provenance: claims.provenance.trim(),
    jti: claims.jti.trim(),
    ath: claims.ath.trim()
  };
  return signSignedToken(privateKey, CALLER_CONTEXT_TYP, normalizedClaims as Record<string, unknown>, keyId);
}

function cleanReplayCache(nowMs: number): void {
  for (const [key, expiresAtMs] of replayCache.entries()) {
    if (expiresAtMs <= nowMs) {
      replayCache.delete(key);
    }
  }
}

export function verifyCallerContextEnvelope(
  issuers: CallerContextIssuerRegistry,
  token: string,
  policy: VerifyCallerContextPolicy,
  nowMs = Date.now()
): VerifiedCallerContext {
  const parsed = parseSignedToken(token);
  if (parsed.header.alg !== 'RS256' || parsed.header.typ !== CALLER_CONTEXT_TYP) {
    throw new Error('unsupported_token_type');
  }
  const claims = parsed.payload as Partial<CallerContextClaims>;
  const issuer = normalizeString(claims.iss);
  const audience = normalizeString(claims.aud);
  const subject = normalizeString(claims.sub);
  const userId = normalizeString(claims.user_id);
  const actorService = normalizeString(claims.actor_service);
  const provenance = normalizeString(claims.provenance);
  const jti = normalizeString(claims.jti);
  const ath = normalizeString(claims.ath);
  if (!issuer || !audience || !subject || !userId || !actorService || !provenance || !jti || !ath) {
    throw new Error('invalid_caller_context_claims');
  }
  const issuerProfile = resolveIssuerProfile(issuer, issuers);
  verifySignature(issuerProfile.verificationKey, parsed);
  if (claims.version !== 2) {
    throw new Error('caller_context_version_mismatch');
  }
  const expectedIssuers = normalizeExpectedStrings(policy.expectedIssuer);
  if (expectedIssuers && !expectedIssuers.includes(issuer)) {
    throw new Error('issuer_mismatch');
  }
  if (audience !== policy.expectedAudience) {
    throw new Error('audience_mismatch');
  }
  if (policy.expectedAccessTokenHash && ath !== policy.expectedAccessTokenHash) {
    throw new Error('caller_context_binding_mismatch');
  }
  if (typeof claims.iat !== 'number' || typeof claims.exp !== 'number') {
    throw new Error('invalid_caller_context_timestamps');
  }
  const maxClockSkewSeconds = policy.maxClockSkewSeconds ?? 10;
  if (claims.nbf && nowMs + maxClockSkewSeconds * 1000 < claims.nbf * 1000) {
    throw new Error('caller_context_not_yet_valid');
  }
  if (claims.exp * 1000 <= nowMs - maxClockSkewSeconds * 1000) {
    throw new Error('caller_context_expired');
  }
  if (policy.preventReplay !== false) {
    cleanReplayCache(nowMs);
    const replayKey = `${issuer}:${audience}:${jti}`;
    const existing = replayCache.get(replayKey);
    if (existing && existing > nowMs) {
      throw new Error('caller_context_replayed');
    }
    replayCache.set(replayKey, claims.exp * 1000);
  }
  return {
    ...claims,
    version: 2,
    iss: issuer,
    aud: audience,
    sub: subject,
    user_id: userId,
    actor_service: actorService,
    provenance,
    jti,
    ath
  } as VerifiedCallerContext;
}
