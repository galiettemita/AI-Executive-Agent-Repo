import { createCipheriv, createDecipheriv, createHash, randomBytes } from 'node:crypto';
import {
  signAccessToken,
  verifyAccessToken,
  type AccessTokenClaims,
  type AccessTokenIssuerRegistry,
  type AccessTokenUse,
} from '../../../packages/shared/src/security.js';
import type {
  AnyEdgeExecutionAuthorizationEnvelope,
  EdgeExecutionPolicy,
} from '../../../packages/shared/src/edge-execution-contract.js';

export type RelayAuthMode = 'optional' | 'required';

export type RelayRole = 'device' | 'operator' | 'admin';

export interface RelayTokenClaims {
  version: 2;
  role: RelayRole;
  iss: string;
  aud: string;
  sub: string;
  token_use: AccessTokenUse;
  iat: number;
  scopes: string[];
  user_id?: string;
  device_id?: string;
  cert_fingerprint?: string;
  allowed_skills?: string[];
  exp: number;
}

export interface VerifyRelayTokenPolicy {
  issuers: AccessTokenIssuerRegistry;
  expectedAudience: string | string[];
  allowedRoles: readonly RelayRole[];
  expectedConfirmationThumbprint?: string;
}

export interface ProtectedInputEnvelope {
  alg: 'aes-256-gcm';
  nonce: string;
  ciphertext: string;
}

export interface SessionSummaryInput {
  userId: string;
  deviceId: string;
  connectedAt: number;
  lastSeenAt: number;
  supportedSkills: Iterable<string>;
  certFingerprint: string;
  authBound: boolean;
}

export interface ExecuteRequestInput {
  tenant_id?: unknown;
  workspace_id?: unknown;
  user_id?: unknown;
  device_id?: unknown;
  skill_id?: unknown;
  allowed_skills?: unknown;
  tool?: unknown;
  operation?: unknown;
  input?: unknown;
  policy?: unknown;
  authorization?: unknown;
  run_id?: unknown;
  task_id?: unknown;
  step_id?: unknown;
  attempt?: unknown;
}

export interface BoundExecuteRequest {
  tenantId?: string;
  workspaceId?: string;
  userId: string;
  deviceId: string;
  skillId: string;
  tool?: string;
  operation?: string;
  input: Record<string, unknown>;
  policy?: EdgeExecutionPolicy;
  authorization?: AnyEdgeExecutionAuthorizationEnvelope;
  runId?: string;
  taskId?: string;
  stepId?: string;
  attempt?: number;
}

function encodeBase64Url(value: Buffer | string): string {
  return Buffer.isBuffer(value) ? value.toString('base64url') : Buffer.from(value, 'utf8').toString('base64url');
}

function decodeBase64Url(value: string): Buffer {
  return Buffer.from(value, 'base64url');
}

function normalizeString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function ensureNonEmptyString(value: unknown, field: string): string {
  const normalized = normalizeString(value);
  if (!normalized) {
    throw new Error(`${field} must be a non-empty string`);
  }
  return normalized;
}

function ensureRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function normalizeStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const normalized = value
    .map((entry) => normalizeString(entry))
    .filter((entry): entry is string => Boolean(entry));
  return normalized.length > 0 ? normalized : undefined;
}

function normalizePositiveInt(value: unknown, field: string): number | undefined {
  if (value === undefined || value === null || value === '') {
    return undefined;
  }
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) {
    throw new Error(`${field} must be a positive integer`);
  }
  return value;
}

function normalizeExecutionPolicy(value: unknown): EdgeExecutionPolicy | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined;
  }
  const policy = value as Record<string, unknown>;
  return {
    consent_requirement:
      policy.consent_requirement === 'none' ||
      policy.consent_requirement === 'recommended' ||
      policy.consent_requirement === 'required'
        ? policy.consent_requirement
        : undefined,
    consent_record: normalizeString(policy.consent_record),
    human_review:
      policy.human_review === 'none' ||
      policy.human_review === 'recommended' ||
      policy.human_review === 'required'
        ? policy.human_review
        : undefined,
    human_review_record: normalizeString(policy.human_review_record),
    recipient_verification:
      policy.recipient_verification === 'not_applicable' ||
      policy.recipient_verification === 'required' ||
      policy.recipient_verification === 'verified'
        ? policy.recipient_verification
        : undefined
  };
}

function normalizeExecutionAuthorization(value: unknown): AnyEdgeExecutionAuthorizationEnvelope | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined;
  }
  const envelope = value as Record<string, unknown>;
  const nonce = normalizeString(envelope.nonce);
  const issuedAt = normalizeString(envelope.issued_at);
  const expiresAt = normalizeString(envelope.expires_at);
  const dispatchReceiptId = normalizeString(envelope.dispatch_receipt_id);
  const policyHash = normalizeString(envelope.policy_hash);
  const signature = normalizeString(envelope.signature);
  const approved = typeof envelope.approved === 'boolean' ? envelope.approved : undefined;
  if (!nonce || !issuedAt || !expiresAt || !dispatchReceiptId || !policyHash || !signature || approved === undefined) {
    return undefined;
  }
  if (envelope.key_id === 'edge-execution-v1') {
    return {
      key_id: 'edge-execution-v1',
      nonce,
      issued_at: issuedAt,
      expires_at: expiresAt,
      dispatch_receipt_id: dispatchReceiptId,
      policy_hash: policyHash,
      approved,
      signature
    };
  }
  if (envelope.key_id === 'edge-execution-v2') {
    const requestId = normalizeString(envelope.request_id);
    const userId = normalizeString(envelope.user_id);
    const deviceId = normalizeString(envelope.device_id);
    const skillId = normalizeString(envelope.skill_id);
    const tool = normalizeString(envelope.tool);
    const operation = normalizeString(envelope.operation);
    const inputHash = normalizeString(envelope.input_hash);
    if (!requestId || !userId || !deviceId || !skillId || !tool || !operation || !inputHash) {
      return undefined;
    }
    return {
      key_id: 'edge-execution-v2',
      nonce,
      issued_at: issuedAt,
      expires_at: expiresAt,
      dispatch_receipt_id: dispatchReceiptId,
      policy_hash: policyHash,
      request_id: requestId,
      user_id: userId,
      device_id: deviceId,
      skill_id: skillId,
      tool,
      operation,
      input_hash: inputHash,
      approved,
      signature
    };
  }
  return undefined;
}

export function parseRelayAuthMode(raw: string | undefined, environment: string, hasSecret: boolean): RelayAuthMode {
  const normalized = raw?.trim().toLowerCase() ?? 'auto';
  if (normalized === 'optional') {
    return 'optional';
  }
  if (normalized === 'required') {
    return 'required';
  }
  if (normalized !== 'auto') {
    throw new Error('EDGE_AUTH_MODE must be auto, optional, or required');
  }
  if (environment.trim().toLowerCase() === 'local') {
    return 'optional';
  }
  return hasSecret ? 'required' : 'required';
}

export function deriveSymmetricKey(raw: string | undefined, fallbackSeed?: string): Buffer {
  const material = normalizeString(raw) ?? fallbackSeed ?? randomBytes(32).toString('base64url');
  return createHash('sha256').update(material).digest();
}

export function pseudonymize(value: string, salt: string): string {
  return createHash('sha256').update(`${salt}:${value}`).digest('hex').slice(0, 16);
}

function tokenUseForRole(role: RelayRole): AccessTokenUse {
  switch (role) {
    case 'admin':
      return 'admin_access';
    case 'operator':
      return 'service_access';
    case 'device':
      return 'device_access';
  }
}

function roleForTokenUse(tokenUse: AccessTokenUse): RelayRole {
  switch (tokenUse) {
    case 'admin_access':
      return 'admin';
    case 'service_access':
      return 'operator';
    case 'device_access':
      return 'device';
    case 'user_access':
      throw new Error('user_access tokens are not valid relay credentials');
  }
}

function allowedTokenUsesForRoles(roles: readonly RelayRole[]): AccessTokenUse[] {
  return [...new Set(roles.map((role) => tokenUseForRole(role)))];
}

export function signRelayToken(signingKey: string, claims: RelayTokenClaims, keyId = 'brevio-relay-access-v2'): string {
  const normalizedClaims = {
    ...claims,
    version: 2,
    token_use: tokenUseForRole(claims.role),
    scopes: [...new Set((claims.scopes ?? []).map((scope) => scope.trim()).filter((scope) => scope.length > 0))],
    user_id: normalizeString(claims.user_id),
    device_id: normalizeString(claims.device_id),
    allowed_skills: claims.allowed_skills?.map((skill) => skill.trim()).filter((skill) => skill.length > 0),
    cnf: claims.cert_fingerprint ? { 'x5t#S256': claims.cert_fingerprint.trim() } : undefined
  };
  return signAccessToken(signingKey, normalizedClaims as AccessTokenClaims, keyId);
}

export function verifyRelayToken(policy: VerifyRelayTokenPolicy, token: string, nowMs = Date.now()): RelayTokenClaims {
  const verified = verifyAccessToken(
    policy.issuers,
    token,
    {
      expectedAudience: policy.expectedAudience,
      allowedTokenUses: allowedTokenUsesForRoles(policy.allowedRoles),
      expectedConfirmationThumbprint: policy.expectedConfirmationThumbprint
    },
    nowMs
  );
  const claims = verified as RelayTokenClaims & Record<string, unknown>;
  const role = roleForTokenUse(verified.token_use);
  const explicitRole = normalizeString(claims.role);
  if (explicitRole && explicitRole !== role) {
    throw new Error('relay_role_mismatch');
  }
  const allowedSkills = normalizeStringArray(claims.allowed_skills);
  return {
    version: 2,
    role,
    iss: verified.iss,
    aud: verified.aud,
    sub: verified.sub,
    token_use: verified.token_use,
    iat: verified.iat,
    exp: verified.exp,
    scopes: verified.scopes,
    user_id: normalizeString(claims.user_id),
    device_id: normalizeString(claims.device_id) ?? (role === 'device' ? verified.sub : undefined),
    cert_fingerprint: normalizeString(verified.cnf?.['x5t#S256']),
    allowed_skills: allowedSkills
  };
}

export function bindExecuteRequest(input: ExecuteRequestInput, principal: RelayTokenClaims | null): BoundExecuteRequest {
  const tenantId = normalizeString(input.tenant_id);
  const workspaceId = normalizeString(input.workspace_id);
  const requestedUserId = normalizeString(input.user_id);
  const requestedDeviceId = normalizeString(input.device_id);
  const skillId = ensureNonEmptyString(input.skill_id, 'skill_id');
  const tool = normalizeString(input.tool);
  const operation = normalizeString(input.operation);
  const bodyInput = ensureRecord(input.input);
  const policy = normalizeExecutionPolicy(input.policy);
  const authorization = normalizeExecutionAuthorization(input.authorization);
  const runId = normalizeString(input.run_id);
  const taskId = normalizeString(input.task_id);
  const stepId = normalizeString(input.step_id);
  const attempt = normalizePositiveInt(input.attempt, 'attempt');

  const userId = principal?.user_id ?? requestedUserId;
  const deviceId = principal?.device_id ?? requestedDeviceId;
  if (!userId) {
    throw new Error('user_id must be a non-empty string');
  }
  if (!deviceId) {
    throw new Error('device_id must be a non-empty string');
  }
  if (principal?.user_id && requestedUserId && requestedUserId !== principal.user_id) {
    throw new Error('user_id does not match relay token');
  }
  if (principal?.device_id && requestedDeviceId && requestedDeviceId !== principal.device_id) {
    throw new Error('device_id does not match relay token');
  }
  if (principal?.allowed_skills && principal.allowed_skills.length > 0 && !principal.allowed_skills.includes(skillId)) {
    throw new Error('skill_id is not permitted by relay token');
  }

  return {
    tenantId,
    workspaceId,
    userId,
    deviceId,
    skillId,
    tool,
    operation,
    input: bodyInput,
    policy,
    authorization,
    runId,
    taskId,
    stepId,
    attempt
  };
}

export function protectQueuedInput(input: Record<string, unknown>, key: Buffer, context: string): ProtectedInputEnvelope {
  const nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  cipher.setAAD(Buffer.from(context, 'utf8'));
  const plaintext = Buffer.from(JSON.stringify(input), 'utf8');
  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const tag = cipher.getAuthTag();
  return {
    alg: 'aes-256-gcm',
    nonce: encodeBase64Url(nonce),
    ciphertext: encodeBase64Url(Buffer.concat([encrypted, tag]))
  };
}

export function recoverQueuedInput(envelope: ProtectedInputEnvelope, key: Buffer, context: string): Record<string, unknown> {
  if (envelope.alg !== 'aes-256-gcm') {
    throw new Error('unsupported queue encryption algorithm');
  }
  const nonce = decodeBase64Url(envelope.nonce);
  const ciphertext = decodeBase64Url(envelope.ciphertext);
  if (ciphertext.length < 17) {
    throw new Error('invalid encrypted payload');
  }
  const body = ciphertext.subarray(0, ciphertext.length - 16);
  const tag = ciphertext.subarray(ciphertext.length - 16);
  const decipher = createDecipheriv('aes-256-gcm', key, nonce);
  decipher.setAAD(Buffer.from(context, 'utf8'));
  decipher.setAuthTag(tag);
  const plaintext = Buffer.concat([decipher.update(body), decipher.final()]).toString('utf8');
  const decoded = JSON.parse(plaintext);
  return ensureRecord(decoded);
}

export function buildSessionSummaries(sessions: SessionSummaryInput[], salt: string): Array<Record<string, unknown>> {
  return sessions.map((session) => ({
    session_ref: `${pseudonymize(session.userId, salt)}:${pseudonymize(session.deviceId, salt)}`,
    user_ref: pseudonymize(session.userId, salt),
    device_ref: pseudonymize(session.deviceId, salt),
    connected_at: new Date(session.connectedAt).toISOString(),
    last_seen_at: new Date(session.lastSeenAt).toISOString(),
    supported_skill_count: Array.from(session.supportedSkills).length,
    attested: session.certFingerprint !== '' && session.certFingerprint !== 'unknown',
    auth_bound: session.authBound
  }));
}
