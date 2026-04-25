import http from 'node:http';
import { randomUUID } from 'node:crypto';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { URL } from 'node:url';
import WebSocket, { WebSocketServer } from 'ws';
import {
  edgeExecutionPolicyHash,
  parseCapabilityInventory,
  resolveCapabilityInventory,
  signEdgeExecutionAuthorization,
} from '@brevio/shared';
import type { EdgeExecutionAuthorizationEnvelope, EdgeExecutionPolicy } from '@brevio/shared';
import {
  bindExecuteRequest,
  buildSessionSummaries,
  deriveSymmetricKey,
  parseRelayAuthMode,
  protectQueuedInput,
  pseudonymize,
  recoverQueuedInput,
  verifyRelayToken,
} from './security.js';
import type { BoundExecuteRequest, ProtectedInputEnvelope, RelayAuthMode, RelayTokenClaims } from './security.js';
import { ExecutionStore } from './execution-store.js';
import type { ExecutionRecord as StoredExecutionRecord } from './execution-store.js';
import { RelayStateStore } from './relay-state-store.js';
import { reportExecutionLifecycle } from './workflow-runtime.js';
import { buildToolKey, getToolDescriptor, isRegisteredOperation } from '../../brevio-brain/src/catalog.js';

type SkillStatus =
  | 'SUCCESS'
  | 'PARTIAL'
  | 'FAILED'
  | 'TIMEOUT'
  | 'NEEDS_CONSENT'
  | 'NOT_EXECUTED'
  | 'SIMULATED';

interface RelayConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  relayPath: string;
  maxQueueAgeMs: number;
  maxQueuePerDevice: number;
  maxBodyBytes: number;
  maxWsPayloadBytes: number;
  dispatchLeaseMs: number;
  resultDeadlineMs: number;
  authMode: RelayAuthMode;
  tokenSecret?: string;
  executionAuthSecret: string;
  queueEncryptionKey: Buffer;
  logSalt: string;
  temporalWorkerBaseUrl?: string;
  temporalWorkerTimeoutMs: number;
  capabilityInventoryJson?: string;
  executionStateFilePath: string;
  relayStateFilePath: string;
  workflowReportRetryBaseMs: number;
  workflowReportMaxAttempts: number;
}

interface ExecuteSkillMessage {
  type: 'execute_skill';
  protocol_version: '2026.edge.v2';
  request_id: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
  skill_id: string;
  tool: string;
  operation: string;
  policy?: EdgeExecutionPolicy;
  authorization: EdgeExecutionAuthorizationEnvelope;
  dispatch_receipt_id: string;
  result_deadline_at: string;
  input: Record<string, unknown>;
  queued_at: string;
}

interface RegisterMessage {
  type: 'register';
  protocol_version?: string;
  user_id: string;
  device_id: string;
  device_name?: string;
  client_cert_fingerprint?: string;
  agent_version?: string;
  os_version?: string;
  supported_skills?: string[];
}

interface HeartbeatMessage {
  type: 'heartbeat';
  ts: string;
}

interface ExecuteAckMessage {
  type: 'execute_ack';
  request_id: string;
  dispatch_receipt_id: string;
}

interface SkillResultMessage {
  type: 'skill_result';
  request_id: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
  skill_id: string;
  status: SkillStatus;
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
  latency_ms: number;
  dispatch_receipt_id: string;
  result_receipt_id: string;
}

type AgentInboundMessage = RegisterMessage | HeartbeatMessage | ExecuteAckMessage | SkillResultMessage;

interface OutboundAck {
  type: 'ack' | 'error' | 'result_ack';
  message: string;
  request_id?: string;
  result_receipt_id?: string;
}

interface QueuedExecution {
  requestId: string;
  userId: string;
  deviceId: string;
  skillId: string;
  tool: string;
  operation: string;
  policy?: EdgeExecutionPolicy;
  runId?: string;
  taskId?: string;
  stepId?: string;
  attempt?: number;
  protectedInput: ProtectedInputEnvelope;
  queuedAt: number;
}

interface AgentSession {
  socket: WebSocket;
  userId: string;
  deviceId: string;
  deviceName: string;
  protocolVersion: string;
  certFingerprint: string;
  connectedAt: number;
  lastSeenAt: number;
  supportedSkills: Set<string>;
  authBound: boolean;
  allowedSkills: Set<string>;
  tokenClaims: RelayTokenClaims | null;
}

const config = loadConfig(process.env);
const startedAt = Date.now();
const sessions = new Map<string, AgentSession>();
const scheduledWorkflowReports = new Map<string, NodeJS.Timeout>();
const executionStore = new ExecutionStore(config.executionStateFilePath);
const relayState = new RelayStateStore(config.relayStateFilePath);
const dispatchRecoveryHandle = setInterval(
  () => reclaimStaleDispatches(),
  Math.min(config.dispatchLeaseMs, 30_000)
);
dispatchRecoveryHandle.unref();

class RelayHttpError extends Error {
  statusCode: number;
  code: string;

  constructor(statusCode: number, code: string, message: string) {
    super(message);
    this.statusCode = statusCode;
    this.code = code;
  }
}

function extractRequestToken(req: http.IncomingMessage, requestUrl?: URL): string | undefined {
  const authorization = normalizeHeader(req.headers.authorization);
  if (authorization?.toLowerCase().startsWith('bearer ')) {
    return authorization.slice(7).trim();
  }
  return (
    normalizeHeader(req.headers['x-edge-token']) ??
    normalizeHeader(req.headers['x-edge-agent-token']) ??
    requestUrl?.searchParams.get('token')?.trim()
  ) || undefined;
}

function authorizeHttpRequest(req: http.IncomingMessage, allowedRoles: readonly RelayTokenClaims['role'][]): RelayTokenClaims | null {
  const token = extractRequestToken(req);
  if (!token) {
    if (config.authMode === 'required') {
      throw new RelayHttpError(401, 'unauthorized', 'relay token is required');
    }
    return null;
  }
  if (!config.tokenSecret) {
    throw new RelayHttpError(401, 'unauthorized', 'relay token verification is unavailable');
  }
  let claims: RelayTokenClaims;
  try {
    claims = verifyRelayToken(config.tokenSecret, token);
  } catch (error) {
    throw new RelayHttpError(401, 'unauthorized', error instanceof Error ? error.message : 'invalid relay token');
  }
  if (!allowedRoles.includes(claims.role)) {
    throw new RelayHttpError(403, 'forbidden', 'relay token does not have the required role');
  }
  return claims;
}

function authorizeWebSocket(req: http.IncomingMessage, requestUrl: URL): RelayTokenClaims | null {
  const token = extractRequestToken(req, requestUrl);
  if (!token) {
    if (config.authMode === 'required') {
      throw new RelayHttpError(401, 'unauthorized', 'relay token is required for edge session establishment');
    }
    return null;
  }
  if (!config.tokenSecret) {
    throw new RelayHttpError(401, 'unauthorized', 'relay token verification is unavailable');
  }
  let claims: RelayTokenClaims;
  try {
    claims = verifyRelayToken(config.tokenSecret, token);
  } catch (error) {
    throw new RelayHttpError(401, 'unauthorized', error instanceof Error ? error.message : 'invalid relay token');
  }
  if (claims.role !== 'device') {
    throw new RelayHttpError(403, 'forbidden', 'edge session tokens must use the device role');
  }
  return claims;
}

function resolveBoundIdentity(requested: string | string[] | null | undefined, fallback: string | undefined, field: 'user_id' | 'device_id'): string {
  const requestedValue = typeof requested === 'string' ? requested.trim() : undefined;
  if (fallback && requestedValue && requestedValue !== fallback) {
    throw new RelayHttpError(403, 'forbidden', `${field} does not match relay token`);
  }
  const value = fallback ?? requestedValue;
  if (!value) {
    throw new RelayHttpError(401, 'unauthorized', `${field} is required for relay session establishment`);
  }
  return value;
}

function queueContext(execution: Pick<QueuedExecution, 'requestId' | 'userId' | 'deviceId' | 'skillId'>): string {
  return `${execution.requestId}:${execution.userId}:${execution.deviceId}:${execution.skillId}`;
}

function toIdentityRefs(userId: string, deviceId: string): Record<string, string> {
  return {
    user_ref: pseudonymize(userId, config.logSalt),
    device_ref: pseudonymize(deviceId, config.logSalt),
  };
}

function capabilityResolutionFor(
  identity: { tenantId?: string; workspaceId?: string; userId: string; deviceId: string }
): ReturnType<typeof resolveCapabilityInventory> {
  return resolveCapabilityInventory(
    parseCapabilityInventory(config.capabilityInventoryJson),
    {
      tenantId: identity.tenantId,
      workspaceId: identity.workspaceId,
      userId: identity.userId,
      deviceId: identity.deviceId
    }
  );
}

function intersectSkills(...groups: Array<Iterable<string> | undefined>): string[] {
  const populated = groups
    .map((group) => (group ? [...new Set(Array.from(group).map((value) => value.trim()).filter((value) => value.length > 0))] : []))
    .filter((group) => group.length > 0);
  if (populated.length === 0) {
    return [];
  }
  const [first, ...rest] = populated;
  return first.filter((skill) => rest.every((group) => group.includes(skill)));
}

function resolveToolContract(request: BoundExecuteRequest): { tool: string; operation: string } {
  const descriptor = getToolDescriptor(request.skillId);
  if (!descriptor) {
    throw new RelayHttpError(400, 'invalid_request', `skill ${request.skillId} is not registered`);
  }
  const requestedOperation =
    request.operation ??
    (typeof request.input.action === 'string' && request.input.action.trim() !== '' ? request.input.action.trim() : undefined) ??
    (descriptor.operations.length === 1 ? descriptor.operations[0] : undefined);
  if (!requestedOperation || !isRegisteredOperation(request.skillId, requestedOperation)) {
    throw new RelayHttpError(400, 'invalid_request', `operation is required and must be registered for ${request.skillId}`);
  }
  const expectedTool = buildToolKey(request.skillId, requestedOperation);
  if (request.tool && request.tool !== expectedTool) {
    throw new RelayHttpError(400, 'invalid_request', `tool must match ${expectedTool}`);
  }
  return {
    tool: expectedTool,
    operation: requestedOperation
  };
}

function requiresPolicyGate(
  skillId: string,
  operation: string,
  policy: EdgeExecutionPolicy | undefined
): boolean {
  const descriptor = getToolDescriptor(skillId);
  return Boolean(
    descriptor?.requires_confirmation ||
      descriptor?.write_operations.includes(operation) ||
      policy?.consent_requirement === 'required' ||
      policy?.human_review === 'required' ||
      policy?.recipient_verification === 'required'
  );
}

function assertPolicySatisfied(skillId: string, operation: string, policy: EdgeExecutionPolicy | undefined): void {
  if (!requiresPolicyGate(skillId, operation, policy)) {
    return;
  }
  if (!policy) {
    throw new RelayHttpError(403, 'forbidden', `policy is required before ${skillId}.${operation} can run locally`);
  }
  if (policy.consent_requirement === 'required' && !policy.consent_record) {
    throw new RelayHttpError(403, 'forbidden', `consent_record is required before ${skillId}.${operation} can run locally`);
  }
  if (policy.human_review === 'required' && !policy.human_review_record) {
    throw new RelayHttpError(403, 'forbidden', `human_review_record is required before ${skillId}.${operation} can run locally`);
  }
  if (policy.recipient_verification === 'required') {
    throw new RelayHttpError(403, 'forbidden', `recipient_verification must be completed before ${skillId}.${operation} can run locally`);
  }
}

function parseAuthorizedExecuteRequest(body: unknown, principal: RelayTokenClaims | null): BoundExecuteRequest {
  try {
    return bindExecuteRequest(isRecord(body) ? body : {}, principal);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'invalid execute request';
    if (message.includes('does not match relay token') || message.includes('not permitted by relay token')) {
      throw new RelayHttpError(403, 'forbidden', message);
    }
    throw new RelayHttpError(400, 'invalid_request', message);
  }
}

const server = http.createServer((req, res) => {
  if (!req.url) {
    writeJson(res, 400, { error: 'invalid_request', message: 'request url is required' });
    return;
  }
  const requestUrl = new URL(req.url, 'http://localhost');

  if (req.method === 'GET' && requestUrl.pathname === '/health') {
    writeJson(res, 200, healthPayload(false));
    return;
  }

  if (req.method === 'GET' && requestUrl.pathname === '/.well-known/agent-card.json') {
    writeJson(res, 200, agentCardPayload());
    return;
  }

  if (req.method === 'GET' && requestUrl.pathname === '/health/deep') {
    writeJson(res, 200, healthPayload(true));
    return;
  }

  if (req.method === 'GET' && requestUrl.pathname === '/v1/edge/sessions') {
    try {
      authorizeHttpRequest(req, ['admin']);
      const connectedSessions = relayState.connectedSessions();
      writeJson(res, 200, {
        auth_mode: config.authMode,
        sessions: buildSessionSummaries(connectedSessions, config.logSalt),
      });
    } catch (error) {
      const statusCode = error instanceof RelayHttpError ? error.statusCode : 401;
      const code = error instanceof RelayHttpError ? error.code : 'unauthorized';
      const message = error instanceof Error ? error.message : 'unauthorized';
      writeJson(res, statusCode, { error: code, message });
    }
    return;
  }

  if (req.method === 'GET' && (requestUrl.pathname === '/v1/edge/requests' || requestUrl.pathname === '/api/v1/edge/requests')) {
    try {
      authorizeHttpRequest(req, ['admin', 'operator']);
      const limitRaw = Number(requestUrl.searchParams.get('limit') ?? '50');
      const limit = Number.isFinite(limitRaw) && limitRaw > 0 ? Math.floor(limitRaw) : 50;
      writeJson(res, 200, {
        requests: executionStore.list(limit).map((record) => serializeExecutionRecord(record))
      });
    } catch (error) {
      const statusCode = error instanceof RelayHttpError ? error.statusCode : 401;
      const code = error instanceof RelayHttpError ? error.code : 'unauthorized';
      const message = error instanceof Error ? error.message : 'unauthorized';
      writeJson(res, statusCode, { error: code, message });
    }
    return;
  }

  const requestPathMatch = requestUrl.pathname.match(/^\/(?:api\/)?v1\/edge\/requests\/([^/]+)$/);
  if (req.method === 'GET' && requestPathMatch) {
    const requestId = requestPathMatch[1];
    if (!requestId) {
      writeJson(res, 400, { error: 'invalid_request', message: 'request id is required' });
      return;
    }
    try {
      authorizeHttpRequest(req, ['admin', 'operator']);
      const record = executionStore.get(requestId);
      if (!record) {
        writeJson(res, 404, { error: 'not_found', request_id: requestId });
        return;
      }
      writeJson(res, 200, serializeExecutionRecord(record));
    } catch (error) {
      const statusCode = error instanceof RelayHttpError ? error.statusCode : 401;
      const code = error instanceof RelayHttpError ? error.code : 'unauthorized';
      const message = error instanceof Error ? error.message : 'unauthorized';
      writeJson(res, statusCode, { error: code, message });
    }
    return;
  }

  if (req.method === 'POST' && (requestUrl.pathname === '/v1/edge/execute' || requestUrl.pathname === '/api/v1/edge/execute')) {
    readJsonBody(req, config.maxBodyBytes)
      .then((body) => parseAuthorizedExecuteRequest(body, authorizeHttpRequest(req, ['admin', 'operator', 'device'])))
      .then((executeRequest) => {
        const requestId = randomUUID();
        const nowMs = Date.now();
        const toolContract = resolveToolContract(executeRequest);
        assertPolicySatisfied(executeRequest.skillId, toolContract.operation, executeRequest.policy);
        const execution: QueuedExecution = {
          requestId,
          userId: executeRequest.userId,
          deviceId: executeRequest.deviceId,
          skillId: executeRequest.skillId,
          tool: toolContract.tool,
          operation: toolContract.operation,
          policy: executeRequest.policy,
          runId: executeRequest.runId,
          taskId: executeRequest.taskId,
          stepId: executeRequest.stepId,
          attempt: executeRequest.attempt,
          protectedInput: protectQueuedInput(executeRequest.input, config.queueEncryptionKey, queueContext({
            requestId,
            userId: executeRequest.userId,
            deviceId: executeRequest.deviceId,
            skillId: executeRequest.skillId,
          })),
          queuedAt: nowMs,
        };
        const capabilityResolution = capabilityResolutionFor({
          tenantId: executeRequest.tenantId,
          workspaceId: executeRequest.workspaceId,
          userId: executeRequest.userId,
          deviceId: executeRequest.deviceId
        });
        if (capabilityResolution.source !== 'none' && !capabilityResolution.enabledSkills.includes(executeRequest.skillId)) {
          executionStore.create(
            {
              requestId,
              userId: executeRequest.userId,
              deviceId: executeRequest.deviceId,
              skillId: executeRequest.skillId,
              tool: toolContract.tool,
              operation: toolContract.operation,
              policy: executeRequest.policy,
              runId: executeRequest.runId,
              taskId: executeRequest.taskId,
              stepId: executeRequest.stepId,
              attempt: executeRequest.attempt,
              protectedInput: execution.protectedInput
            },
            'WAITING_FOR_AGENT',
            nowMs
          );
          const rejected = executionStore.markRejected(
            requestId,
            'CAPABILITY_NOT_ENABLED',
            `skill ${executeRequest.skillId} is not enabled for this user/device capability inventory`,
            nowMs
          );
          reportWorkflowRecord(rejected);
          writeJson(res, 403, {
            status: 'rejected',
            request_id: requestId,
            error: 'capability_not_enabled'
          });
          return;
        }

        const key = sessionKey(executeRequest.userId, executeRequest.deviceId);
        const session = sessions.get(key);
        if (session && session.socket.readyState === WebSocket.OPEN && session.supportedSkills.size > 0) {
          executionStore.create(
            {
              requestId,
              userId: executeRequest.userId,
              deviceId: executeRequest.deviceId,
              skillId: executeRequest.skillId,
              tool: toolContract.tool,
              operation: toolContract.operation,
              policy: executeRequest.policy,
              runId: executeRequest.runId,
              taskId: executeRequest.taskId,
              stepId: executeRequest.stepId,
              attempt: executeRequest.attempt,
              protectedInput: execution.protectedInput
            },
            'WAITING_FOR_AGENT',
            nowMs
          );
          if (!session.supportedSkills.has(executeRequest.skillId)) {
            const rejected = executionStore.markRejected(
              requestId,
              'SKILL_NOT_SUPPORTED_BY_SESSION',
              `Connected device does not advertise support for ${executeRequest.skillId}`,
              nowMs
            );
            reportWorkflowRecord(rejected);
            writeJson(res, 409, {
              status: 'rejected',
              request_id: requestId,
              error: 'skill_not_supported_by_session'
            });
            return;
          }
          dispatchExecution(session, execution);
          writeJson(res, 200, {
            status: 'sent',
            request_id: requestId,
          });
          return;
        }

        executionStore.create(
          {
            requestId,
            userId: executeRequest.userId,
            deviceId: executeRequest.deviceId,
            skillId: executeRequest.skillId,
            tool: toolContract.tool,
            operation: toolContract.operation,
            policy: executeRequest.policy,
            runId: executeRequest.runId,
            taskId: executeRequest.taskId,
            stepId: executeRequest.stepId,
            attempt: executeRequest.attempt,
            protectedInput: execution.protectedInput
          },
          session && session.socket.readyState === WebSocket.OPEN ? 'WAITING_FOR_AGENT' : 'QUEUED',
          nowMs
        );
        enqueueExecution(key, execution);
        writeJson(res, 202, {
          status: 'queued',
          request_id: requestId,
          message: 'I need your Mac to be online to do that.',
        });
      })
      .catch((error: unknown) => {
        const statusCode =
          error instanceof RelayHttpError ? error.statusCode : error instanceof Error && error.message === 'payload_too_large' ? 413 : 400;
        const code =
          error instanceof RelayHttpError ? error.code : error instanceof Error && error.message === 'payload_too_large' ? 'payload_too_large' : 'invalid_request';
        const message = error instanceof Error ? error.message : 'invalid execute request';
        writeJson(res, statusCode, { error: code, message });
      });
    return;
  }

  writeJson(res, 404, { error: 'not_found', service: config.serviceName });
});

const wss = new WebSocketServer({ noServer: true, maxPayload: config.maxWsPayloadBytes });

server.on('upgrade', (req, socket, head) => {
  const reqUrl = req.url ?? '/';
  const parsed = new URL(reqUrl, 'http://localhost');
  if (parsed.pathname !== config.relayPath) {
    socket.destroy();
    return;
  }

  wss.handleUpgrade(req, socket, head, (ws) => {
    wss.emit('connection', ws, req);
  });
});

wss.on('connection', (socket, req) => {
  const requestUrl = new URL(req.url ?? '/', 'http://localhost');
  let tokenClaims: RelayTokenClaims | null = null;
  try {
    tokenClaims = authorizeWebSocket(req, requestUrl);
  } catch (error) {
    sendMessage(socket, {
      type: 'error',
      message: error instanceof Error ? error.message : 'relay session rejected',
    });
    socket.close(1008, 'unauthorized');
    return;
  }

  let userId: string;
  let deviceId: string;
  try {
    userId = resolveBoundIdentity(requestUrl.searchParams.get('user_id') ?? req.headers['x-user-id'], tokenClaims?.user_id, 'user_id');
    deviceId = resolveBoundIdentity(requestUrl.searchParams.get('device_id') ?? req.headers['x-device-id'], tokenClaims?.device_id, 'device_id');
  } catch (error) {
    sendMessage(socket, {
      type: 'error',
      message: error instanceof Error ? error.message : 'relay session rejected',
    });
    socket.close(1008, 'unauthorized');
    return;
  }

  const key = sessionKey(userId, deviceId);
  const session: AgentSession = {
    socket,
    userId: userId.trim(),
    deviceId: deviceId.trim(),
    deviceName: requestUrl.searchParams.get('device_name') ?? deviceId,
    protocolVersion: '2026.edge.v1',
    certFingerprint: tokenClaims?.cert_fingerprint ?? normalizeHeader(req.headers['x-client-cert-fingerprint']) ?? 'unknown',
    connectedAt: Date.now(),
    lastSeenAt: Date.now(),
    supportedSkills: new Set<string>(),
    authBound: Boolean(tokenClaims),
    allowedSkills: new Set(tokenClaims?.allowed_skills ?? []),
    tokenClaims,
  };

  sessions.set(key, session);
  relayState.upsertSession(key, {
    userId: session.userId,
    deviceId: session.deviceId,
    deviceName: session.deviceName,
    certFingerprint: session.certFingerprint,
    connectedAt: session.connectedAt,
    lastSeenAt: session.lastSeenAt,
    supportedSkills: [],
    authBound: session.authBound,
    allowedSkills: Array.from(session.allowedSkills),
    connected: true
  });
  logEvent('edge_agent_connected', {
    ...toIdentityRefs(session.userId, session.deviceId),
    auth_bound: session.authBound,
  });

  sendMessage(socket, {
    type: 'ack',
    message: 'relay connection established',
  });

  socket.on('message', (payload) => {
    let message: AgentInboundMessage | null;
    try {
      message = parseAgentInboundMessage(payload.toString());
    } catch (error) {
      sendMessage(socket, {
        type: 'error',
        message: error instanceof Error ? error.message : 'invalid relay payload'
      });
      return;
    }
    if (!message) {
      sendMessage(socket, {
        type: 'error',
        message: 'invalid relay payload',
      });
      return;
    }

    session.lastSeenAt = Date.now();
    relayState.touchSession(key, session.lastSeenAt);

    if (message.type === 'register') {
      if (session.authBound) {
        if (message.user_id !== session.userId || message.device_id !== session.deviceId) {
          sendMessage(socket, {
            type: 'error',
            message: 'registration identity does not match relay token',
          });
          socket.close(1008, 'identity_mismatch');
          return;
        }
        if (session.tokenClaims?.cert_fingerprint && message.client_cert_fingerprint && message.client_cert_fingerprint !== session.tokenClaims.cert_fingerprint) {
          sendMessage(socket, {
            type: 'error',
            message: 'client certificate fingerprint does not match relay token',
          });
          socket.close(1008, 'attestation_mismatch');
          return;
        }
      }
      session.deviceName = message.device_name?.trim() || session.deviceName;
      session.protocolVersion = optionalString(message.protocol_version) ?? session.protocolVersion;
      session.certFingerprint = message.client_cert_fingerprint?.trim() || session.certFingerprint;
      const supportedSkills = (message.supported_skills ?? []).filter((skill) => skill.trim() !== '');
      const capabilityResolution = capabilityResolutionFor({
        userId: session.userId,
        deviceId: session.deviceId
      });
      const effectiveSupportedSkills =
        capabilityResolution.source === 'none'
          ? (session.allowedSkills.size > 0 ? intersectSkills(supportedSkills, session.allowedSkills) : supportedSkills)
          : intersectSkills(
              supportedSkills,
              capabilityResolution.enabledSkills,
              session.allowedSkills.size > 0 ? session.allowedSkills : undefined
            );
      session.supportedSkills = new Set(effectiveSupportedSkills);
      relayState.upsertSession(key, {
        userId: session.userId,
        deviceId: session.deviceId,
        deviceName: session.deviceName,
        certFingerprint: session.certFingerprint,
        connectedAt: session.connectedAt,
        lastSeenAt: session.lastSeenAt,
        supportedSkills: Array.from(session.supportedSkills),
        authBound: session.authBound,
        allowedSkills: Array.from(session.allowedSkills),
        connected: true
      });

      sendMessage(socket, {
        type: 'ack',
        message: 'registration accepted',
      });
      flushQueue(key);
      return;
    }

    if (message.type === 'heartbeat') {
      sendMessage(socket, {
        type: 'ack',
        message: 'heartbeat accepted',
      });
      return;
    }

    if (message.type === 'execute_ack') {
      const acknowledged = executionStore.markAcknowledged(
        message.request_id,
        key,
        message.dispatch_receipt_id,
        Date.now(),
        Date.now() + config.resultDeadlineMs
      );
      if (!acknowledged || acknowledged.dispatchReceiptId !== message.dispatch_receipt_id || acknowledged.dispatchedSessionKey !== key) {
        sendMessage(socket, {
          type: 'error',
          request_id: message.request_id,
          message: 'execute ack rejected'
        });
        return;
      }
      sendMessage(socket, {
        type: 'ack',
        request_id: message.request_id,
        message: 'execute ack accepted'
      });
      return;
    }

    if (message.type === 'skill_result') {
      const result = executionStore.applyResult(
        {
          requestId: message.request_id,
          runId: message.run_id,
          taskId: message.task_id,
          stepId: message.step_id,
          attempt: message.attempt,
          skillId: message.skill_id,
          status: message.status,
          data: message.data,
          error: message.error,
          latencyMs: message.latency_ms,
          sessionKey: key,
          dispatchReceiptId: message.dispatch_receipt_id,
          resultReceiptId: message.result_receipt_id
        },
        Date.now()
      );
      if (
        result.outcome === 'unknown_request' ||
        result.outcome === 'skill_mismatch' ||
        result.outcome === 'ref_mismatch' ||
        result.outcome === 'provenance_mismatch'
      ) {
        sendMessage(socket, {
          type: 'error',
          request_id: message.request_id,
          message: `skill result rejected: ${result.outcome}`
        });
        return;
      }
      sendMessage(socket, {
        type: 'result_ack',
        request_id: message.request_id,
        result_receipt_id: message.result_receipt_id,
        message: result.outcome === 'duplicate' ? 'skill result already recorded' : 'skill result accepted'
      });
      if (result.outcome === 'applied' || result.outcome === 'duplicate') {
        reportWorkflowRecord(result.record ?? null);
      }
      logEvent('edge_skill_result_received', {
        ...toIdentityRefs(session.userId, session.deviceId),
        request_id: message.request_id,
        skill_id: message.skill_id,
        status: message.status,
        latency_ms: message.latency_ms,
      });
      return;
    }
  });

  socket.on('close', () => {
    const existing = sessions.get(key);
    if (existing?.socket === socket) {
      sessions.delete(key);
    }
    relayState.disconnectSession(key, session.lastSeenAt);
    logEvent('edge_agent_disconnected', {
      ...toIdentityRefs(session.userId, session.deviceId),
    });
  });

  socket.on('error', (error) => {
    logEvent('edge_agent_socket_error', {
      ...toIdentityRefs(session.userId, session.deviceId),
      error: error.message,
    });
  });
});

server.listen(config.port, () => {
  for (const record of executionStore.pendingWorkflowReports(Date.now())) {
    scheduleWorkflowReport(record, 0);
  }
  reclaimStaleDispatches();
  logEvent('service_started', {
    service: config.serviceName,
    environment: config.environment,
    port: config.port,
    relay_path: config.relayPath,
    auth_mode: config.authMode,
    queue_encrypted: true,
    workflow_report_pending: executionStore.stats().pendingWorkflowReports,
  });
});

function shutdown(signal: string): void {
  logEvent('shutdown_start', { signal });

  for (const timer of scheduledWorkflowReports.values()) {
    clearTimeout(timer);
  }
  scheduledWorkflowReports.clear();
  clearInterval(dispatchRecoveryHandle);

  for (const session of sessions.values()) {
    session.socket.close(1001, 'server_shutdown');
  }

  server.close(() => {
    logEvent('shutdown_complete', {});
    process.exit(0);
  });

  setTimeout(() => {
    logEvent('shutdown_timeout', {});
    process.exit(1);
  }, 30_000).unref();
}

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));

function loadConfig(env: NodeJS.ProcessEnv): RelayConfig {
  const environment = env.BREVIO_ENV?.trim() || 'local';
  const tokenSecret = env.EDGE_RELAY_TOKEN_SECRET?.trim();
  const authMode = parseRelayAuthMode(env.EDGE_AUTH_MODE, environment, Boolean(tokenSecret));
  const executionAuthSecret = env.EDGE_EXECUTION_AUTH_SECRET?.trim() || (environment === 'local' ? 'local-edge-execution-auth' : undefined);
  if (environment !== 'local' && !tokenSecret) {
    throw new Error('EDGE_RELAY_TOKEN_SECRET is required outside local environments');
  }
  if (authMode === 'required' && !tokenSecret) {
    throw new Error('EDGE_RELAY_TOKEN_SECRET is required when EDGE_AUTH_MODE resolves to required');
  }
  if (!executionAuthSecret) {
    throw new Error('EDGE_EXECUTION_AUTH_SECRET is required outside local environments');
  }
  return {
    serviceName: 'brevio-edge-relay',
    version: env.SERVICE_VERSION?.trim() || '0.1.0',
    environment,
    port: parseIntWithDefault(env.PORT, 8086),
    relayPath: env.EDGE_RELAY_PATH?.trim() || '/ws/edge',
    maxQueueAgeMs: parseIntWithDefault(env.EDGE_MAX_QUEUE_AGE_MS, 4 * 60 * 60 * 1000),
    maxQueuePerDevice: parseIntWithDefault(env.EDGE_MAX_QUEUE_PER_DEVICE, 100),
    maxBodyBytes: parseIntWithDefault(env.EDGE_MAX_BODY_BYTES, 2 * 1024 * 1024),
    maxWsPayloadBytes: parseIntWithDefault(env.EDGE_MAX_WS_PAYLOAD_BYTES, 2 * 1024 * 1024),
    dispatchLeaseMs: parseIntWithDefault(env.EDGE_DISPATCH_LEASE_MS, 30_000),
    resultDeadlineMs: parseIntWithDefault(env.EDGE_RESULT_DEADLINE_MS, 5 * 60 * 1000),
    authMode,
    tokenSecret,
    executionAuthSecret,
    queueEncryptionKey: deriveSymmetricKey(env.EDGE_QUEUE_ENCRYPTION_KEY?.trim(), tokenSecret ?? `${environment}:${env.SERVICE_VERSION ?? '0.1.0'}`),
    logSalt: env.EDGE_RELAY_LOG_SALT?.trim() || tokenSecret || `${environment}:edge-relay`,
    temporalWorkerBaseUrl: env.BREVIO_TEMPORAL_WORKER_BASE_URL?.trim() || undefined,
    temporalWorkerTimeoutMs: parseIntWithDefault(env.BREVIO_TEMPORAL_WORKER_TIMEOUT_MS, 1500),
    capabilityInventoryJson: env.BREVIO_CAPABILITY_INVENTORY_JSON?.trim() || undefined,
    executionStateFilePath:
      env.EDGE_EXECUTION_STATE_FILE?.trim() ||
      path.join(process.cwd(), '.runtime', 'edge-relay-execution-state.json'),
    relayStateFilePath:
      env.EDGE_RELAY_STATE_FILE?.trim() ||
      path.join(process.cwd(), '.runtime', 'edge-relay-state.json'),
    workflowReportRetryBaseMs: parseIntWithDefault(env.EDGE_WORKFLOW_REPORT_RETRY_BASE_MS, 1000),
    workflowReportMaxAttempts: parseIntWithDefault(env.EDGE_WORKFLOW_REPORT_MAX_ATTEMPTS, 5),
  };
}

function parseIntWithDefault(raw: string | undefined, fallback: number): number {
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) {
    return fallback;
  }
  return Math.floor(value);
}

function normalizeHeader(header: string | string[] | undefined): string | null {
  if (typeof header === 'string' && header.trim() !== '') {
    return header.trim();
  }
  if (Array.isArray(header) && header.length > 0) {
    const first = header[0];
    if (first && first.trim() !== '') {
      return first.trim();
    }
  }
  return null;
}

function sessionKey(userId: string, deviceId: string): string {
  return `${userId}:${deviceId}`;
}

function healthPayload(deep: boolean): Record<string, unknown> {
  const queuedCount = relayState.queuedCount(Date.now(), config.maxQueueAgeMs);
  const executionStats = executionStore.stats();
  const checks: Record<string, unknown> = {
    process: 'ok',
    connected_agents: relayState.connectedSessionCount(),
    auth_mode: config.authMode,
  };

  if (deep) {
    checks.queued_executions = queuedCount;
    checks.max_queue_age_ms = config.maxQueueAgeMs;
    checks.max_queue_per_device = config.maxQueuePerDevice;
    checks.max_body_bytes = config.maxBodyBytes;
    checks.max_ws_payload_bytes = config.maxWsPayloadBytes;
    checks.dispatch_lease_ms = config.dispatchLeaseMs;
    checks.result_deadline_ms = config.resultDeadlineMs;
    checks.queue_encrypted = true;
    checks.queue_backend = relayState.mode();
    checks.relay_state_file_path = relayState.snapshotPath();
    checks.relay_state_file_exists = relayState.snapshotPath() ? existsSync(relayState.snapshotPath()!) : false;
    checks.sessions_redacted = true;
    checks.tracked_session_records = relayState.trackedSessionCount();
    checks.execution_records = executionStats.total;
    checks.active_requests = executionStats.active;
    checks.terminal_requests = executionStats.terminal;
    checks.workflow_report_pending = executionStats.pendingWorkflowReports;
    checks.execution_store_mode = executionStore.mode();
    checks.execution_state_file_path = executionStore.snapshotPath();
    checks.execution_state_file_exists = executionStore.snapshotPath() ? existsSync(executionStore.snapshotPath()!) : false;
    checks.durable_execution = relayState.mode() !== 'in_memory' && executionStore.mode() !== 'in_memory';
  }

  return {
    status: 'healthy',
    service: config.serviceName,
    version: config.version,
    uptime_ms: Date.now() - startedAt,
    checks,
  };
}

function enqueueExecution(key: string, execution: QueuedExecution): void {
  relayState.enqueueExecution(key, execution, config.maxQueuePerDevice, config.maxQueueAgeMs, Date.now());

  logEvent('edge_execution_queued', {
    request_id: execution.requestId,
    ...toIdentityRefs(execution.userId, execution.deviceId),
    skill_id: execution.skillId,
  });
}

function workflowReportDelayMs(attempts: number): number {
  return Math.min(config.workflowReportRetryBaseMs * Math.max(1, 2 ** Math.max(0, attempts - 1)), 30_000);
}

function scheduleWorkflowReport(record: StoredExecutionRecord | null, delayMs = 0): void {
  if (!record?.workflowReport || !record.runId || !record.stepId) {
    return;
  }
  const existing = scheduledWorkflowReports.get(record.requestId);
  if (existing) {
    clearTimeout(existing);
  }
  const timer = setTimeout(() => {
    scheduledWorkflowReports.delete(record.requestId);
    const latest = executionStore.get(record.requestId);
    if (!latest?.workflowReport || latest.workflowReport.status === 'DELEGATED' || latest.workflowReport.status === 'FAILED') {
      return;
    }
    void reportExecutionLifecycle(latest, config).then((result) => {
      const nextRetryAt = result.delegated
        ? undefined
        : Date.now() + workflowReportDelayMs((latest.workflowReport?.attempts ?? 0) + 1);
      const updated = executionStore.markWorkflowReportOutcome(
        latest.requestId,
        result.delegated,
        Date.now(),
        result.warning,
        nextRetryAt,
        config.workflowReportMaxAttempts
      );
      if (result.warning) {
        logEvent('edge_workflow_report_deferred', {
          request_id: latest.requestId,
          run_id: latest.runId,
          step_id: latest.stepId,
          warning: result.warning,
          attempts: updated?.workflowReport?.attempts,
          workflow_report_status: updated?.workflowReport?.status
        });
      }
      if (updated?.workflowReport?.status === 'RETRYING') {
        scheduleWorkflowReport(updated, workflowReportDelayMs(updated.workflowReport.attempts));
      }
      if (updated?.workflowReport?.status === 'FAILED') {
        logEvent('edge_workflow_report_failed', {
          request_id: latest.requestId,
          run_id: latest.runId,
          step_id: latest.stepId,
          warning: updated.workflowReport.warning,
          attempts: updated.workflowReport.attempts
        });
      }
    });
  }, delayMs);
  timer.unref();
  scheduledWorkflowReports.set(record.requestId, timer);
}

function reportWorkflowRecord(record: StoredExecutionRecord | null): void {
  if (
    !record?.workflowReport ||
    record.workflowReport.status === 'DELEGATED' ||
    record.workflowReport.status === 'FAILED'
  ) {
    return;
  }
  scheduleWorkflowReport(record, 0);
}

function pruneExpiredFromQueue(key: string): void {
  relayState.queueFor(key, Date.now(), config.maxQueueAgeMs);
}

function reclaimStaleDispatches(): void {
  const nowMs = Date.now();
  for (const record of executionStore.collectExpiredDispatches(nowMs)) {
    const key = sessionKey(record.userId, record.deviceId);
    const session = sessions.get(key);
    const canRetry =
      record.protectedInput &&
      record.status === 'SENT' &&
      session &&
      session.socket.readyState === WebSocket.OPEN &&
      session.supportedSkills.has(record.skillId);

    if (canRetry) {
      executionStore.markQueued(record.requestId, nowMs, 'WAITING_FOR_AGENT');
      enqueueExecution(key, {
        requestId: record.requestId,
        userId: record.userId,
        deviceId: record.deviceId,
        skillId: record.skillId,
        tool: record.tool ?? buildToolKey(record.skillId, record.operation ?? 'execute'),
        operation: record.operation ?? 'execute',
        policy: record.policy,
        runId: record.runId,
        taskId: record.taskId,
        stepId: record.stepId,
        attempt: record.attempt,
        protectedInput: record.protectedInput,
        queuedAt: record.queuedAt ?? record.createdAt
      });
      flushQueue(key);
      continue;
    }

    const failed = executionStore.markFailedTerminal(
      record.requestId,
      'FAILED',
      'STALE_DISPATCH',
      'edge dispatch expired before acknowledgement or result delivery',
      nowMs
    );
    reportWorkflowRecord(failed);
  }
}

function flushQueue(key: string): void {
  const session = sessions.get(key);
  if (!session || session.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const queue = relayState.takeQueue(key, Date.now(), config.maxQueueAgeMs);

  if (queue.length === 0) {
    return;
  }

  for (const execution of queue) {
    if (!session.supportedSkills.has(execution.skillId)) {
      const rejected = executionStore.markRejected(
        execution.requestId,
        'SKILL_NOT_SUPPORTED_BY_SESSION',
        `Connected device does not advertise support for ${execution.skillId}`,
        Date.now()
      );
      reportWorkflowRecord(rejected);
      logEvent('edge_execution_rejected_unsupported_skill', {
        request_id: execution.requestId,
        ...toIdentityRefs(execution.userId, execution.deviceId),
        skill_id: execution.skillId,
      });
      continue;
    }
    try {
      dispatchExecution(session, execution);
    } catch (error) {
      const failed = executionStore.markFailedTerminal(
        execution.requestId,
        'FAILED',
        'QUEUE_DECRYPT_FAILED',
        error instanceof Error ? error.message : 'queue decrypt failed',
        Date.now()
      );
      reportWorkflowRecord(failed);
      logEvent('edge_execution_drop_failed_decrypt', {
        request_id: execution.requestId,
        ...toIdentityRefs(execution.userId, execution.deviceId),
        skill_id: execution.skillId,
        error: error instanceof Error ? error.message : 'queue decrypt failed',
      });
    }
  }
}

function dispatchExecution(session: AgentSession, execution: QueuedExecution): void {
  const nowMs = Date.now();
  const input = recoverQueuedInput(execution.protectedInput, config.queueEncryptionKey, queueContext(execution));
  const dispatchReceiptId = randomUUID();
  const authorization = signEdgeExecutionAuthorization(config.executionAuthSecret, {
    key_id: 'edge-execution-v1',
    nonce: randomUUID(),
    issued_at: new Date(nowMs).toISOString(),
    expires_at: new Date(nowMs + config.resultDeadlineMs).toISOString(),
    dispatch_receipt_id: dispatchReceiptId,
    policy_hash: edgeExecutionPolicyHash(execution.policy),
    approved: true
  });
  executionStore.markSent(execution.requestId, {
    nowMs,
    sessionKey: sessionKey(execution.userId, execution.deviceId),
    dispatchReceiptId,
    dispatchLeaseExpiresAt: nowMs + config.dispatchLeaseMs,
    resultDeadlineAt: nowMs + config.resultDeadlineMs
  });
  const payload: ExecuteSkillMessage = {
    type: 'execute_skill',
    protocol_version: '2026.edge.v2',
    request_id: execution.requestId,
    run_id: execution.runId,
    task_id: execution.taskId,
    step_id: execution.stepId,
    attempt: execution.attempt,
    skill_id: execution.skillId,
    tool: execution.tool,
    operation: execution.operation,
    policy: execution.policy,
    authorization,
    dispatch_receipt_id: dispatchReceiptId,
    result_deadline_at: new Date(nowMs + config.resultDeadlineMs).toISOString(),
    input,
    queued_at: new Date(execution.queuedAt).toISOString(),
  };

  sendMessage(session.socket, payload);
  logEvent('edge_execution_dispatched', {
    request_id: execution.requestId,
    ...toIdentityRefs(execution.userId, execution.deviceId),
    skill_id: execution.skillId,
  });
}

function sendMessage(socket: WebSocket, message: OutboundAck | ExecuteSkillMessage): void {
  if (socket.readyState !== WebSocket.OPEN) {
    return;
  }
  socket.send(JSON.stringify(message));
}

function parseAgentInboundMessage(raw: string): AgentInboundMessage | null {
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return null;
  }

  if (!isRecord(decoded) || typeof decoded.type !== 'string') {
    return null;
  }

  if (decoded.type === 'register') {
    const userId = ensureNonEmptyString(decoded.user_id, 'user_id');
    const deviceId = ensureNonEmptyString(decoded.device_id, 'device_id');
    const supportedSkills = Array.isArray(decoded.supported_skills)
      ? decoded.supported_skills.filter((value): value is string => typeof value === 'string')
      : [];

    return {
      type: 'register',
      protocol_version: optionalString(decoded.protocol_version),
      user_id: userId,
      device_id: deviceId,
      device_name: optionalString(decoded.device_name),
      client_cert_fingerprint: optionalString(decoded.client_cert_fingerprint),
      agent_version: optionalString(decoded.agent_version),
      os_version: optionalString(decoded.os_version),
      supported_skills: supportedSkills,
    };
  }

  if (decoded.type === 'heartbeat') {
    return {
      type: 'heartbeat',
      ts: optionalString(decoded.ts) || new Date().toISOString(),
    };
  }

  if (decoded.type === 'execute_ack') {
    return {
      type: 'execute_ack',
      request_id: ensureNonEmptyString(decoded.request_id, 'request_id'),
      dispatch_receipt_id: ensureNonEmptyString(decoded.dispatch_receipt_id, 'dispatch_receipt_id')
    };
  }

  if (decoded.type === 'skill_result') {
    const requestId = ensureNonEmptyString(decoded.request_id, 'request_id');
    const skillId = ensureNonEmptyString(decoded.skill_id, 'skill_id');
    const statusRaw = decoded.status;
    if (!isSkillStatus(statusRaw)) {
      throw new Error('status must be a supported skill execution status');
    }

    const latency = Number(decoded.latency_ms);
    if (!Number.isFinite(latency) || latency < 0) {
      throw new Error('latency_ms must be a non-negative number');
    }

    const error = isRecord(decoded.error)
      ? {
          code: optionalString(decoded.error.code) || 'UNKNOWN',
          message: optionalString(decoded.error.message) || 'unknown error',
        }
      : undefined;

    return {
      type: 'skill_result',
      request_id: requestId,
      run_id: optionalString(decoded.run_id),
      task_id: optionalString(decoded.task_id),
      step_id: optionalString(decoded.step_id),
      attempt: typeof decoded.attempt === 'number' && Number.isInteger(decoded.attempt) && decoded.attempt > 0 ? decoded.attempt : undefined,
      skill_id: skillId,
      status: statusRaw,
      data: isRecord(decoded.data) ? decoded.data : undefined,
      error,
      latency_ms: Math.floor(latency),
      dispatch_receipt_id: ensureNonEmptyString(decoded.dispatch_receipt_id, 'dispatch_receipt_id'),
      result_receipt_id: ensureNonEmptyString(decoded.result_receipt_id, 'result_receipt_id'),
    };
  }

  return null;
}

function optionalString(value: unknown): string | undefined {
  if (typeof value === 'string' && value.trim() !== '') {
    return value.trim();
  }
  return undefined;
}

function ensureNonEmptyString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${field} must be a non-empty string`);
  }
  return value.trim();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isSkillStatus(value: unknown): value is SkillStatus {
  return (
    value === 'SUCCESS' ||
    value === 'PARTIAL' ||
    value === 'FAILED' ||
    value === 'TIMEOUT' ||
    value === 'NEEDS_CONSENT' ||
    value === 'NOT_EXECUTED' ||
    value === 'SIMULATED'
  );
}

function readJsonBody(req: http.IncomingMessage, maxBytes: number): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let bytes = 0;
    req.on('data', (chunk) => {
      const data = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
      bytes += data.byteLength;
      if (bytes > maxBytes) {
        reject(new Error('payload_too_large'));
        req.destroy();
        return;
      }
      chunks.push(data);
    });
    req.on('error', reject);
    req.on('end', () => {
      const payload = Buffer.concat(chunks).toString('utf8').trim();
      if (payload === '') {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(payload));
      } catch {
        reject(new Error('request body must be valid JSON'));
      }
    });
  });
}

function writeJson(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

function logEvent(event: string, attrs: Record<string, unknown>): void {
  process.stdout.write(
    `${JSON.stringify({
      ts: new Date().toISOString(),
      service: config.serviceName,
      env: config.environment,
      event,
      attrs,
    })}\n`,
  );
}

function agentCardPayload(): Record<string, unknown> {
  return {
    agent_id: config.serviceName,
    name: 'Brevio Edge Relay',
    description: 'Relay for capability-aware local edge skill execution with request tracking and result correlation.',
    version: config.version,
    protocol_version: '2026.a2a.v1',
    default_endpoint: `http://localhost:${config.port}/v1/edge/execute`,
    capabilities: [
      {
        id: 'edge.local.execute',
        name: 'Local Edge Skill Dispatch',
        description: 'Dispatches local device-bound skills and tracks lifecycle state until completion.',
        version: '1.0.0',
        input_modes: ['application/json'],
        output_modes: ['application/json'],
        async: true
      }
    ],
    supports: {
      task_lifecycle: true,
      task_query: true,
      artifact_updates: false,
      push_callbacks: true,
      capability_inventory: true
    }
  };
}

function serializeExecutionRecord(record: StoredExecutionRecord): Record<string, unknown> {
  return {
    request_id: record.requestId,
    run_id: record.runId,
    task_id: record.taskId,
    step_id: record.stepId,
    attempt: record.attempt,
    user_ref: pseudonymize(record.userId, config.logSalt),
    device_ref: pseudonymize(record.deviceId, config.logSalt),
    skill_id: record.skillId,
    tool: record.tool,
    operation: record.operation,
    status: record.status,
    created_at: new Date(record.createdAt).toISOString(),
    updated_at: new Date(record.updatedAt).toISOString(),
    queued_at: record.queuedAt ? new Date(record.queuedAt).toISOString() : undefined,
    dispatched_at: record.dispatchedAt ? new Date(record.dispatchedAt).toISOString() : undefined,
    delivery_ack_at: record.deliveryAckAt ? new Date(record.deliveryAckAt).toISOString() : undefined,
    completed_at: record.completedAt ? new Date(record.completedAt).toISOString() : undefined,
    dispatch_lease_expires_at: record.dispatchLeaseExpiresAt ? new Date(record.dispatchLeaseExpiresAt).toISOString() : undefined,
    result_deadline_at: record.resultDeadlineAt ? new Date(record.resultDeadlineAt).toISOString() : undefined,
    result: record.result
      ? {
          status: record.result.status,
          data: record.result.data,
          error: record.result.error,
          latency_ms: record.result.latencyMs,
          result_receipt_id: record.result.resultReceiptId
        }
      : undefined,
    last_error: record.lastError,
    workflow_report: record.workflowReport
      ? {
          status: record.workflowReport.status,
          attempts: record.workflowReport.attempts,
          updated_at: new Date(record.workflowReport.updatedAt).toISOString(),
          next_retry_at: record.workflowReport.nextRetryAt ? new Date(record.workflowReport.nextRetryAt).toISOString() : undefined,
          warning: record.workflowReport.warning
        }
      : undefined
  };
}
