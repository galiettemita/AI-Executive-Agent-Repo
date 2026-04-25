import { createHash, createHmac, timingSafeEqual } from 'node:crypto';

export interface EdgeExecutionPolicy {
  consent_requirement?: 'none' | 'recommended' | 'required';
  consent_record?: string;
  human_review?: 'none' | 'recommended' | 'required';
  human_review_record?: string;
  recipient_verification?: 'not_applicable' | 'required' | 'verified';
}

export interface EdgeExecutionAuthorizationEnvelope {
  key_id: 'edge-execution-v1';
  nonce: string;
  issued_at: string;
  expires_at: string;
  dispatch_receipt_id: string;
  policy_hash: string;
  approved: boolean;
  signature: string;
}

export interface EdgeExecutionAuthorizationEnvelopeV2 {
  key_id: 'edge-execution-v2';
  nonce: string;
  issued_at: string;
  expires_at: string;
  dispatch_receipt_id: string;
  policy_hash: string;
  request_id: string;
  user_id: string;
  device_id: string;
  skill_id: string;
  tool: string;
  operation: string;
  input_hash: string;
  approved: boolean;
  signature: string;
}

export type AnyEdgeExecutionAuthorizationEnvelope =
  | EdgeExecutionAuthorizationEnvelope
  | EdgeExecutionAuthorizationEnvelopeV2;

export interface EdgeExecutionAuthorizationInput {
  key_id: 'edge-execution-v1';
  nonce: string;
  issued_at: string;
  expires_at: string;
  dispatch_receipt_id: string;
  policy_hash: string;
  approved: boolean;
  signature: string;
}

function stableSerialize(value: unknown): string {
  if (value === null || value === undefined) {
    return 'null';
  }
  if (typeof value !== 'object') {
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableSerialize(item)).join(',')}]`;
  }
  const entries = Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left.localeCompare(right));
  return `{${entries.map(([key, entry]) => `${JSON.stringify(key)}:${stableSerialize(entry)}`).join(',')}}`;
}

export function edgeExecutionPolicyHash(policy: EdgeExecutionPolicy | undefined): string {
  return createHash('sha256').update(stableSerialize(policy ?? {})).digest('hex');
}

export function edgeExecutionInputHash(input: Record<string, unknown>): string {
  return createHash('sha256').update(stableSerialize(input)).digest('hex');
}

function edgeExecutionSignatureBase(
  envelope: Omit<AnyEdgeExecutionAuthorizationEnvelope, 'signature'>
): string {
  return stableSerialize(envelope);
}

export function signEdgeExecutionAuthorization(
  secret: string,
  envelope: Omit<AnyEdgeExecutionAuthorizationEnvelope, 'signature'>
): AnyEdgeExecutionAuthorizationEnvelope {
  const signature = createHmac('sha256', secret).update(edgeExecutionSignatureBase(envelope)).digest('hex');
  return {
    ...envelope,
    signature
  };
}

export function verifyEdgeExecutionAuthorization(
  secret: string,
  envelope: AnyEdgeExecutionAuthorizationEnvelope | undefined,
  expected: {
    dispatchReceiptId: string;
    policy: EdgeExecutionPolicy | undefined;
    requestId?: string;
    userId?: string;
    deviceId?: string;
    skillId?: string;
    tool?: string;
    operation?: string;
    input?: Record<string, unknown>;
  },
  nowMs = Date.now()
): { valid: boolean; reason?: string } {
  if (!envelope) {
    return { valid: false, reason: 'missing_authorization' };
  }
  if (envelope.key_id !== 'edge-execution-v1' && envelope.key_id !== 'edge-execution-v2') {
    return { valid: false, reason: 'unsupported_authorization_key' };
  }
  if (envelope.dispatch_receipt_id !== expected.dispatchReceiptId) {
    return { valid: false, reason: 'dispatch_receipt_mismatch' };
  }
  if (envelope.policy_hash !== edgeExecutionPolicyHash(expected.policy)) {
    return { valid: false, reason: 'policy_hash_mismatch' };
  }
  const expiresAt = Date.parse(envelope.expires_at);
  if (!Number.isFinite(expiresAt) || expiresAt <= nowMs) {
    return { valid: false, reason: 'authorization_expired' };
  }
  if (envelope.key_id === 'edge-execution-v2') {
    if (
      envelope.request_id !== expected.requestId ||
      envelope.user_id !== expected.userId ||
      envelope.device_id !== expected.deviceId ||
      envelope.skill_id !== expected.skillId ||
      envelope.tool !== expected.tool ||
      envelope.operation !== expected.operation ||
      envelope.input_hash !== edgeExecutionInputHash(expected.input ?? {})
    ) {
      return { valid: false, reason: 'authorization_binding_mismatch' };
    }
  }
  const expectedSignature = createHmac('sha256', secret)
    .update(
      edgeExecutionSignatureBase(
        envelope.key_id === 'edge-execution-v2'
          ? {
              key_id: envelope.key_id,
              nonce: envelope.nonce,
              issued_at: envelope.issued_at,
              expires_at: envelope.expires_at,
              dispatch_receipt_id: envelope.dispatch_receipt_id,
              policy_hash: envelope.policy_hash,
              request_id: envelope.request_id,
              user_id: envelope.user_id,
              device_id: envelope.device_id,
              skill_id: envelope.skill_id,
              tool: envelope.tool,
              operation: envelope.operation,
              input_hash: envelope.input_hash,
              approved: envelope.approved
            }
          : {
              key_id: envelope.key_id,
              nonce: envelope.nonce,
              issued_at: envelope.issued_at,
              expires_at: envelope.expires_at,
              dispatch_receipt_id: envelope.dispatch_receipt_id,
              policy_hash: envelope.policy_hash,
              approved: envelope.approved
            }
      )
    )
    .digest();
  const actualSignature = Buffer.from(envelope.signature, 'hex');
  if (actualSignature.length !== expectedSignature.length || !timingSafeEqual(actualSignature, expectedSignature)) {
    return { valid: false, reason: 'invalid_authorization_signature' };
  }
  if (!envelope.approved) {
    return { valid: false, reason: 'authorization_not_approved' };
  }
  return { valid: true };
}
