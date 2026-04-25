import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';
import WebSocket from 'ws';
import type { AnyEdgeExecutionAuthorizationEnvelope } from '@brevio/shared';
import { verifyEdgeExecutionAuthorization } from '@brevio/shared';
import { buildToolKey, getToolDescriptor, isRegisteredOperation } from '../../../services/brevio-brain/src/catalog.js';
import { executeImplementedLocalSkill, resolveSupportedLocalSkills, supportsLocalOperation } from './local-skills.js';
import { ResultOutboxStore } from './result-outbox-store.js';

type SkillStatus =
  | 'SUCCESS'
  | 'PARTIAL'
  | 'FAILED'
  | 'TIMEOUT'
  | 'NEEDS_CONSENT'
  | 'NOT_EXECUTED'
  | 'SIMULATED';

interface AgentConfig {
  serviceName: string;
  serviceVersion: string;
  environment: string;
  relayUrl: string;
  relayAuthToken?: string;
  userId: string;
  deviceId: string;
  deviceName: string;
  clientCertFingerprint: string;
  osVersion: string;
  heartbeatMs: number;
  reconnectBaseMs: number;
  reconnectMaxMs: number;
  maxReconnectAttempts: number;
  healthPort: number;
  maxQueueAgeMs: number;
  executionAuthSecret: string;
  resultOutboxFilePath: string;
  supportedSkills: string[];
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
  policy?: {
    consent_requirement?: 'none' | 'recommended' | 'required';
    consent_record?: string;
    human_review?: 'none' | 'recommended' | 'required';
    human_review_record?: string;
    recipient_verification?: 'not_applicable' | 'required' | 'verified';
  };
  authorization: AnyEdgeExecutionAuthorizationEnvelope;
  dispatch_receipt_id: string;
  result_deadline_at: string;
  input: Record<string, unknown>;
  queued_at: string;
}

interface RegisterMessage {
  type: 'register';
  protocol_version: '2026.edge.v2';
  user_id: string;
  device_id: string;
  device_name: string;
  client_cert_fingerprint: string;
  agent_version: string;
  os_version: string;
  supported_skills: string[];
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

interface ResultAckMessage {
  type: 'result_ack';
  request_id: string;
  result_receipt_id: string;
  message?: string;
}

interface AgentState {
  connected: boolean;
  reconnectAttempt: number;
  lastConnectedAt: number | null;
  lastDisconnectedAt: number | null;
  lastHeartbeatAt: number | null;
}

const config = loadConfig(process.env);
const state: AgentState = {
  connected: false,
  reconnectAttempt: 0,
  lastConnectedAt: null,
  lastDisconnectedAt: null,
  lastHeartbeatAt: null,
};
const outbox = new ResultOutboxStore(config.resultOutboxFilePath);

let socket: WebSocket | null = null;
let heartbeatHandle: NodeJS.Timeout | null = null;
let shouldRun = true;
const startedAt = Date.now();

const healthServer = http.createServer((req, res) => {
  if (!req.url) {
    writeJson(res, 400, { error: 'invalid_request' });
    return;
  }

  if (req.method === 'GET' && req.url === '/health') {
    writeJson(res, 200, healthPayload(false));
    return;
  }

  if (req.method === 'GET' && req.url === '/health/deep') {
    writeJson(res, 200, healthPayload(true));
    return;
  }

  writeJson(res, 404, { error: 'not_found' });
});

healthServer.listen(config.healthPort, () => {
  logEvent('edge_agent_health_server_started', {
    port: config.healthPort,
  });
  void connectWithRetry();
});

process.on('SIGTERM', () => {
  void shutdown('SIGTERM');
});
process.on('SIGINT', () => {
  void shutdown('SIGINT');
});

async function connectWithRetry(): Promise<void> {
  while (shouldRun) {
    try {
      await connectOnce();
      return;
    } catch (error) {
      state.reconnectAttempt += 1;
      const message = error instanceof Error ? error.message : 'unknown error';
      logEvent('edge_agent_connect_failed', {
        attempt: state.reconnectAttempt,
        error: message,
      });

      if (state.reconnectAttempt >= config.maxReconnectAttempts) {
        logEvent('edge_agent_connect_give_up', {
          max_attempts: config.maxReconnectAttempts,
        });
        return;
      }

      const backoffMs = Math.min(
        config.reconnectBaseMs * 2 ** Math.max(0, state.reconnectAttempt - 1),
        config.reconnectMaxMs,
      );
      await delay(backoffMs);
    }
  }
}

function connectOnce(): Promise<void> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(buildRelayConnectionUrl(), {
      headers: {
        'x-client-cert-fingerprint': config.clientCertFingerprint,
        ...(config.relayAuthToken ? { authorization: `Bearer ${config.relayAuthToken}` } : {}),
      },
    });

    socket = ws;

    const openHandler = (): void => {
      state.connected = true;
      state.reconnectAttempt = 0;
      state.lastConnectedAt = Date.now();
      sendRegister(ws);
      flushQueuedResults(ws);
      startHeartbeat(ws);

      logEvent('edge_agent_connected', {
        relay_url: config.relayUrl,
        device_id: config.deviceId,
      });
      resolve();
    };

    ws.once('open', openHandler);

    ws.once('error', (error) => {
      ws.off('open', openHandler);
      reject(error);
    });

    ws.on('message', (payload) => {
      handleRelayMessage(ws, payload.toString());
    });

    ws.on('close', () => {
      state.connected = false;
      state.lastDisconnectedAt = Date.now();
      stopHeartbeat();
      if (shouldRun) {
        void connectWithRetry();
      }
    });
  });
}

function buildRelayConnectionUrl(): string {
  const url = new URL(config.relayUrl);
  url.searchParams.set('user_id', config.userId);
  url.searchParams.set('device_id', config.deviceId);
  url.searchParams.set('device_name', config.deviceName);
  return url.toString();
}

function sendRegister(ws: WebSocket): void {
  const payload: RegisterMessage = {
    type: 'register',
    protocol_version: '2026.edge.v2',
    user_id: config.userId,
    device_id: config.deviceId,
    device_name: config.deviceName,
    client_cert_fingerprint: config.clientCertFingerprint,
    agent_version: config.serviceVersion,
    os_version: config.osVersion,
    supported_skills: config.supportedSkills,
  };
  sendMessage(ws, payload);
}

function startHeartbeat(ws: WebSocket): void {
  stopHeartbeat();
  heartbeatHandle = setInterval(() => {
    const payload: HeartbeatMessage = {
      type: 'heartbeat',
      ts: new Date().toISOString(),
    };
    sendMessage(ws, payload);
    state.lastHeartbeatAt = Date.now();
  }, config.heartbeatMs);
}

function stopHeartbeat(): void {
  if (heartbeatHandle) {
    clearInterval(heartbeatHandle);
    heartbeatHandle = null;
  }
}

function handleRelayMessage(ws: WebSocket, raw: string): void {
  const decoded = parseJson(raw);
  if (!decoded || typeof decoded.type !== 'string') {
    logEvent('edge_agent_invalid_message', { payload: raw });
    return;
  }

  if (decoded.type === 'ack') {
    logEvent('edge_agent_ack', { message: optionalString(decoded.message) ?? 'ack' });
    return;
  }

  if (decoded.type === 'result_ack') {
    const message = parseResultAckMessage(decoded);
    if (!message) {
      logEvent('edge_agent_invalid_result_ack', { payload: raw });
      return;
    }
    outbox.markAcked(message.result_receipt_id);
    logEvent('edge_agent_result_acked', {
      request_id: message.request_id,
      result_receipt_id: message.result_receipt_id
    });
    return;
  }

  if (decoded.type === 'error') {
    logEvent('edge_agent_error_message', { message: optionalString(decoded.message) ?? 'unknown' });
    return;
  }

  if (decoded.type !== 'execute_skill') {
    logEvent('edge_agent_unsupported_message', { type: decoded.type });
    return;
  }

  const message = parseExecuteMessage(decoded);
  if (!message) {
    logEvent('edge_agent_invalid_execute_message', { payload: raw });
    return;
  }

  sendMessage(ws, {
    type: 'execute_ack',
    request_id: message.request_id,
    dispatch_receipt_id: message.dispatch_receipt_id
  });

  void executeSkillLocally(message)
    .then((result) => {
      enqueueResult(result);
      flushQueuedResults(ws);
    })
    .catch((error) => {
      const failure: SkillResultMessage = {
        type: 'skill_result',
        request_id: message.request_id,
        run_id: message.run_id,
        task_id: message.task_id,
        step_id: message.step_id,
        attempt: message.attempt,
        skill_id: message.skill_id,
        status: 'FAILED',
        error: {
          code: 'LOCAL_EXECUTION_ERROR',
          message: error instanceof Error ? error.message : 'local execution failed',
        },
        latency_ms: 0,
        dispatch_receipt_id: message.dispatch_receipt_id,
        result_receipt_id: randomUUID(),
      };
      enqueueResult(failure);
      flushQueuedResults(ws);
    });
}

function parseExecuteMessage(value: Record<string, unknown>): ExecuteSkillMessage | null {
  const requestId = optionalString(value.request_id);
  const skillId = optionalString(value.skill_id);
  const tool = optionalString(value.tool);
  const operation = optionalString(value.operation);
  const dispatchReceiptId = optionalString(value.dispatch_receipt_id);
  const resultDeadlineAt = optionalString(value.result_deadline_at);
  if (!requestId || !skillId || !tool || !operation || !dispatchReceiptId || !resultDeadlineAt) {
    return null;
  }

  const input = isRecord(value.input) ? value.input : {};
  const descriptor = getToolDescriptor(skillId);
  if (!descriptor || !isRegisteredOperation(skillId, operation) || tool !== buildToolKey(skillId, operation)) {
    return null;
  }
  if (!config.supportedSkills.includes(skillId.trim().toLowerCase())) {
    return null;
  }
  if (!supportsLocalOperation(skillId, operation)) {
    return null;
  }
  const authorization = isRecord(value.authorization)
    ? ({
        ...value.authorization,
        key_id: value.authorization.key_id,
        nonce: optionalString(value.authorization.nonce),
        issued_at: optionalString(value.authorization.issued_at),
        expires_at: optionalString(value.authorization.expires_at),
        dispatch_receipt_id: optionalString(value.authorization.dispatch_receipt_id),
        policy_hash: optionalString(value.authorization.policy_hash),
        approved: value.authorization.approved,
        signature: optionalString(value.authorization.signature),
        request_id: optionalString(value.authorization.request_id),
        user_id: optionalString(value.authorization.user_id),
        device_id: optionalString(value.authorization.device_id),
        skill_id: optionalString(value.authorization.skill_id),
        tool: optionalString(value.authorization.tool),
        operation: optionalString(value.authorization.operation),
        input_hash: optionalString(value.authorization.input_hash)
      } as ExecuteSkillMessage['authorization'])
    : undefined;
  if (
    !authorization ||
    !authorization.nonce ||
    !authorization.issued_at ||
    !authorization.expires_at ||
    !authorization.dispatch_receipt_id ||
    !authorization.policy_hash ||
    typeof authorization.approved !== 'boolean' ||
    !authorization.signature
  ) {
    return null;
  }
  if (
    authorization.key_id !== 'edge-execution-v1' &&
    !(
      authorization.key_id === 'edge-execution-v2' &&
      authorization.request_id &&
      authorization.user_id &&
      authorization.device_id &&
      authorization.skill_id &&
      authorization.tool &&
      authorization.operation &&
      authorization.input_hash
    )
  ) {
    return null;
  }
  const policy = isRecord(value.policy)
    ? {
        consent_requirement:
          value.policy.consent_requirement === 'none' ||
          value.policy.consent_requirement === 'recommended' ||
          value.policy.consent_requirement === 'required'
            ? value.policy.consent_requirement
            : undefined,
        consent_record: optionalString(value.policy.consent_record),
        human_review:
          value.policy.human_review === 'none' ||
          value.policy.human_review === 'recommended' ||
          value.policy.human_review === 'required'
            ? value.policy.human_review
            : undefined,
        human_review_record: optionalString(value.policy.human_review_record),
        recipient_verification:
          value.policy.recipient_verification === 'not_applicable' ||
          value.policy.recipient_verification === 'required' ||
          value.policy.recipient_verification === 'verified'
            ? value.policy.recipient_verification
            : undefined
      }
    : undefined;
  const authCheck = verifyEdgeExecutionAuthorization(
    config.executionAuthSecret,
    authorization,
    {
      dispatchReceiptId,
      policy,
      requestId,
      userId: config.userId,
      deviceId: config.deviceId,
      skillId,
      tool,
      operation,
      input
    }
  );
  if (!authCheck.valid) {
    return null;
  }

  return {
    type: 'execute_skill',
    protocol_version: '2026.edge.v2',
    request_id: requestId,
    run_id: optionalString(value.run_id),
    task_id: optionalString(value.task_id),
    step_id: optionalString(value.step_id),
    attempt: typeof value.attempt === 'number' && Number.isInteger(value.attempt) && value.attempt > 0 ? value.attempt : undefined,
    skill_id: skillId,
    tool,
    operation,
    policy,
    authorization,
    dispatch_receipt_id: dispatchReceiptId,
    result_deadline_at: resultDeadlineAt,
    input,
    queued_at: optionalString(value.queued_at) ?? new Date().toISOString(),
  };
}

function parseResultAckMessage(value: Record<string, unknown>): ResultAckMessage | null {
  const requestId = optionalString(value.request_id);
  const resultReceiptId = optionalString(value.result_receipt_id);
  if (!requestId || !resultReceiptId) {
    return null;
  }
  return {
    type: 'result_ack',
    request_id: requestId,
    result_receipt_id: resultReceiptId,
    message: optionalString(value.message)
  };
}

async function executeSkillLocally(message: ExecuteSkillMessage): Promise<SkillResultMessage> {
  const started = Date.now();

  const normalizedSkill = message.skill_id.trim().toLowerCase();
  const resultReceiptId = randomUUID();

  if (!config.supportedSkills.includes(normalizedSkill)) {
    return {
      type: 'skill_result',
      request_id: message.request_id,
      run_id: message.run_id,
      task_id: message.task_id,
      step_id: message.step_id,
      attempt: message.attempt,
      skill_id: message.skill_id,
      status: 'FAILED',
      error: {
        code: 'SKILL_NOT_SUPPORTED_LOCALLY',
        message: `Local edge agent does not support skill ${message.skill_id}`,
      },
      latency_ms: Date.now() - started,
      dispatch_receipt_id: message.dispatch_receipt_id,
      result_receipt_id: resultReceiptId,
    };
  }

  const descriptor = getToolDescriptor(message.skill_id);
  if (!descriptor || !isRegisteredOperation(message.skill_id, message.operation) || message.tool !== buildToolKey(message.skill_id, message.operation)) {
    return {
      type: 'skill_result',
      request_id: message.request_id,
      run_id: message.run_id,
      task_id: message.task_id,
      step_id: message.step_id,
      attempt: message.attempt,
      skill_id: message.skill_id,
      status: 'FAILED',
      error: {
        code: 'UNSUPPORTED_OPERATION',
        message: `Local edge agent rejected invalid tool contract ${message.tool}`
      },
      latency_ms: Date.now() - started,
      dispatch_receipt_id: message.dispatch_receipt_id,
      result_receipt_id: resultReceiptId
    };
  }

  const execution = executeImplementedLocalSkill(normalizedSkill, message.operation, message.input);
  if (!execution) {
    return {
      type: 'skill_result',
      request_id: message.request_id,
      run_id: message.run_id,
      task_id: message.task_id,
      step_id: message.step_id,
      attempt: message.attempt,
      skill_id: message.skill_id,
      status: 'FAILED',
      error: {
        code: 'SKILL_NOT_IMPLEMENTED_LOCALLY',
        message: `Local edge agent is configured for ${message.skill_id}.${message.operation}, but no implementation is registered`
      },
      latency_ms: Date.now() - started,
      dispatch_receipt_id: message.dispatch_receipt_id,
      result_receipt_id: resultReceiptId
    };
  }

  return {
    type: 'skill_result',
    request_id: message.request_id,
    run_id: message.run_id,
    task_id: message.task_id,
    step_id: message.step_id,
    attempt: message.attempt,
    skill_id: message.skill_id,
    status: execution.status,
    data:
      execution.status === 'SUCCESS'
        ? {
            ...(execution.data ?? {}),
            execution_receipt: {
              executor: 'brevio-edge-agent',
              mode: 'local',
              issued_at: new Date().toISOString(),
              receipt_id: resultReceiptId
            }
          }
        : execution.data,
    latency_ms: Date.now() - started,
    dispatch_receipt_id: message.dispatch_receipt_id,
    result_receipt_id: resultReceiptId,
  };
}

function enqueueResult(result: SkillResultMessage): void {
  outbox.enqueue({
    requestId: result.request_id,
    resultReceiptId: result.result_receipt_id,
    queuedAt: Date.now(),
    result
  });
  logEvent('edge_agent_result_queued', {
    request_id: result.request_id,
    skill_id: result.skill_id,
    queue_size: outbox.size(Date.now(), config.maxQueueAgeMs),
  });
}

function flushQueuedResults(ws: WebSocket): void {
  const pending = outbox.pending(Date.now(), config.maxQueueAgeMs);
  if (pending.length === 0 || ws.readyState !== WebSocket.OPEN) {
    return;
  }

  for (const next of pending) {
    sendMessage(ws, next.result);
    outbox.markSent(next.resultReceiptId, Date.now());
  }
}

function sendMessage(ws: WebSocket, payload: RegisterMessage | HeartbeatMessage | SkillResultMessage | ExecuteAckMessage): void {
  if (ws.readyState !== WebSocket.OPEN) {
    return;
  }
  ws.send(JSON.stringify(payload));
}

function parseJson(raw: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(raw);
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function optionalString(value: unknown): string | undefined {
  if (typeof value === 'string' && value.trim() !== '') {
    return value.trim();
  }
  return undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function loadConfig(env: NodeJS.ProcessEnv): AgentConfig {
  const hostname = os.hostname();
  const supportedSkills = resolveSupportedLocalSkills(env.EDGE_SUPPORTED_SKILLS);
  const environment = env.BREVIO_ENV?.trim() || 'local';
  const executionAuthSecret = env.EDGE_EXECUTION_AUTH_SECRET?.trim() || (environment === 'local' ? 'local-edge-execution-auth' : undefined);
  if (!executionAuthSecret) {
    throw new Error('EDGE_EXECUTION_AUTH_SECRET is required outside local environments');
  }

  return {
    serviceName: 'brevio-edge-agent',
    serviceVersion: env.SERVICE_VERSION?.trim() || '0.1.0',
    environment,
    relayUrl: env.EDGE_RELAY_URL?.trim() || 'ws://127.0.0.1:8086/ws/edge',
    relayAuthToken: env.EDGE_RELAY_AUTH_TOKEN?.trim() || undefined,
    userId: env.EDGE_USER_ID?.trim() || 'local-user',
    deviceId: env.EDGE_DEVICE_ID?.trim() || hostname,
    deviceName: env.EDGE_DEVICE_NAME?.trim() || hostname,
    clientCertFingerprint: env.EDGE_CLIENT_CERT_FINGERPRINT?.trim() || 'dev-client-cert-fingerprint',
    osVersion: env.EDGE_OS_VERSION?.trim() || os.release(),
    heartbeatMs: parseIntWithDefault(env.EDGE_HEARTBEAT_MS, 15_000),
    reconnectBaseMs: parseIntWithDefault(env.EDGE_RECONNECT_BASE_MS, 1_000),
    reconnectMaxMs: parseIntWithDefault(env.EDGE_RECONNECT_MAX_MS, 30_000),
    maxReconnectAttempts: parseIntWithDefault(env.EDGE_MAX_RECONNECT_ATTEMPTS, 100),
    healthPort: parseIntWithDefault(env.EDGE_HEALTH_PORT, 18090),
    maxQueueAgeMs: parseIntWithDefault(env.EDGE_MAX_QUEUE_AGE_MS, 4 * 60 * 60 * 1000),
    executionAuthSecret,
    resultOutboxFilePath:
      env.EDGE_RESULT_OUTBOX_FILE?.trim() ||
      path.join(process.cwd(), '.runtime', 'edge-agent-result-outbox.json'),
    supportedSkills,
  };
}

function parseIntWithDefault(raw: string | undefined, fallback: number): number {
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) {
    return fallback;
  }
  return Math.floor(value);
}

function healthPayload(deep: boolean): Record<string, unknown> {
  const checks: Record<string, unknown> = {
    process: 'ok',
    connected: state.connected,
  };

  if (deep) {
    checks.queue_size = outbox.size(Date.now(), config.maxQueueAgeMs);
    checks.last_connected_at = state.lastConnectedAt ? new Date(state.lastConnectedAt).toISOString() : null;
    checks.last_disconnected_at = state.lastDisconnectedAt ? new Date(state.lastDisconnectedAt).toISOString() : null;
    checks.last_heartbeat_at = state.lastHeartbeatAt ? new Date(state.lastHeartbeatAt).toISOString() : null;
    checks.max_queue_age_ms = config.maxQueueAgeMs;
    checks.outbox_mode = outbox.mode();
    checks.outbox_path = outbox.snapshotPath();
    checks.supported_skills = config.supportedSkills;
  }

  return {
    status: 'healthy',
    service: config.serviceName,
    version: config.serviceVersion,
    environment: config.environment,
    uptime_ms: Date.now() - startedAt,
    checks,
  };
}

async function shutdown(signal: string): Promise<void> {
  shouldRun = false;
  logEvent('edge_agent_shutdown_start', { signal });

  stopHeartbeat();
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.close(1001, 'shutdown');
  }

  await new Promise<void>((resolve) => {
    healthServer.close(() => resolve());
  });

  logEvent('edge_agent_shutdown_complete', {});
  process.exit(0);
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
