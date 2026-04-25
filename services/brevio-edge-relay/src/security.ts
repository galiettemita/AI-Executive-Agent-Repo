import { createCipheriv, createDecipheriv, createHash, createHmac, randomBytes, timingSafeEqual } from 'node:crypto';
import type { EdgeExecutionAuthorizationEnvelope, EdgeExecutionPolicy } from '@brevio/shared';

export type RelayAuthMode = 'optional' | 'required';

export type RelayRole = 'device' | 'operator' | 'admin';

export interface RelayTokenClaims {
  version: 1;
  role: RelayRole;
  user_id?: string;
  device_id?: string;
  cert_fingerprint?: string;
  allowed_skills?: string[];
  exp: number;
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
  authorization?: EdgeExecutionAuthorizationEnvelope;
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

function normalizeExecutionAuthorization(value: unknown): EdgeExecutionAuthorizationEnvelope | undefined {
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
  if (
    envelope.key_id !== 'edge-execution-v1' ||
    !nonce ||
    !issuedAt ||
    !expiresAt ||
    !dispatchReceiptId ||
    !policyHash ||
    !signature ||
    approved === undefined
  ) {
    return undefined;
  }
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

export function signRelayToken(secret: string, claims: RelayTokenClaims): string {
  const normalizedClaims: RelayTokenClaims = {
    ...claims,
    version: 1,
    role: claims.role,
    exp: claims.exp,
    user_id: normalizeString(claims.user_id),
    device_id: normalizeString(claims.device_id),
    cert_fingerprint: normalizeString(claims.cert_fingerprint),
    allowed_skills: claims.allowed_skills?.map((skill) => skill.trim()).filter((skill) => skill.length > 0)
  };
  const payload = encodeBase64Url(JSON.stringify(normalizedClaims));
  const signature = createHmac('sha256', secret).update(payload).digest();
  return `brev1.${payload}.${encodeBase64Url(signature)}`;
}

export function verifyRelayToken(secret: string, token: string, nowMs = Date.now()): RelayTokenClaims {
  const parts = token.split('.');
  if (parts.length !== 3 || parts[0] !== 'brev1') {
    throw new Error('invalid relay token');
  }

  const payload = parts[1];
  const signature = decodeBase64Url(parts[2]);
  const expected = createHmac('sha256', secret).update(payload).digest();
  if (signature.length !== expected.length || !timingSafeEqual(signature, expected)) {
    throw new Error('invalid relay token signature');
  }

  const decoded = JSON.parse(decodeBase64Url(payload).toString('utf8')) as Partial<RelayTokenClaims>;
  if (decoded.version !== 1) {
    throw new Error('unsupported relay token version');
  }
  if (decoded.role !== 'device' && decoded.role !== 'operator' && decoded.role !== 'admin') {
    throw new Error('invalid relay token role');
  }
  if (typeof decoded.exp !== 'number' || !Number.isFinite(decoded.exp)) {
    throw new Error('invalid relay token expiry');
  }
  if (nowMs >= decoded.exp * 1000) {
    throw new Error('relay token expired');
  }

  return {
    version: 1,
    role: decoded.role,
    user_id: normalizeString(decoded.user_id),
    device_id: normalizeString(decoded.device_id),
    cert_fingerprint: normalizeString(decoded.cert_fingerprint),
    allowed_skills: decoded.allowed_skills?.map((skill) => skill.trim()).filter((skill) => skill.length > 0),
    exp: decoded.exp
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
