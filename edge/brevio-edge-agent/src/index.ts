import http from 'node:http';
import os from 'node:os';
import { setTimeout as delay } from 'node:timers/promises';
import WebSocket from 'ws';

type SkillStatus = 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT';

interface AgentConfig {
  serviceName: string;
  serviceVersion: string;
  environment: string;
  relayUrl: string;
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
  supportedSkills: string[];
}

interface ExecuteSkillMessage {
  type: 'execute_skill';
  request_id: string;
  skill_id: string;
  input: Record<string, unknown>;
  queued_at: string;
}

interface RegisterMessage {
  type: 'register';
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

interface SkillResultMessage {
  type: 'skill_result';
  request_id: string;
  skill_id: string;
  status: SkillStatus;
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
  latency_ms: number;
}

interface PendingResult {
  queuedAt: number;
  result: SkillResultMessage;
}

interface AgentState {
  connected: boolean;
  reconnectAttempt: number;
  lastConnectedAt: number | null;
  lastDisconnectedAt: number | null;
  lastHeartbeatAt: number | null;
  queue: PendingResult[];
}

const config = loadConfig(process.env);
const state: AgentState = {
  connected: false,
  reconnectAttempt: 0,
  lastConnectedAt: null,
  lastDisconnectedAt: null,
  lastHeartbeatAt: null,
  queue: [],
};

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

  void executeSkillLocally(message)
    .then((result) => {
      if (ws.readyState === WebSocket.OPEN) {
        sendMessage(ws, result);
        return;
      }
      queueResult(result);
    })
    .catch((error) => {
      const failure: SkillResultMessage = {
        type: 'skill_result',
        request_id: message.request_id,
        skill_id: message.skill_id,
        status: 'FAILED',
        error: {
          code: 'LOCAL_EXECUTION_ERROR',
          message: error instanceof Error ? error.message : 'local execution failed',
        },
        latency_ms: 0,
      };

      if (ws.readyState === WebSocket.OPEN) {
        sendMessage(ws, failure);
        return;
      }
      queueResult(failure);
    });
}

function parseExecuteMessage(value: Record<string, unknown>): ExecuteSkillMessage | null {
  const requestId = optionalString(value.request_id);
  const skillId = optionalString(value.skill_id);
  if (!requestId || !skillId) {
    return null;
  }

  const input = isRecord(value.input) ? value.input : {};

  return {
    type: 'execute_skill',
    request_id: requestId,
    skill_id: skillId,
    input,
    queued_at: optionalString(value.queued_at) ?? new Date().toISOString(),
  };
}

async function executeSkillLocally(message: ExecuteSkillMessage): Promise<SkillResultMessage> {
  const started = Date.now();

  const normalizedSkill = message.skill_id.trim().toLowerCase();

  if (!config.supportedSkills.includes(normalizedSkill)) {
    return {
      type: 'skill_result',
      request_id: message.request_id,
      skill_id: message.skill_id,
      status: 'FAILED',
      error: {
        code: 'SKILL_NOT_SUPPORTED_LOCALLY',
        message: `Local edge agent does not support skill ${message.skill_id}`,
      },
      latency_ms: Date.now() - started,
    };
  }

  if (normalizedSkill === 'voice-wake-say') {
    const text = optionalString(message.input.text) ?? 'done';
    return {
      type: 'skill_result',
      request_id: message.request_id,
      skill_id: message.skill_id,
      status: 'SUCCESS',
      data: {
        spoken_text: text,
        transport: 'local_say',
      },
      latency_ms: Date.now() - started,
    };
  }

  if (normalizedSkill === 'apple-remind-me') {
    const title = optionalString(message.input.title) ?? 'Reminder from Brevio';
    return {
      type: 'skill_result',
      request_id: message.request_id,
      skill_id: message.skill_id,
      status: 'SUCCESS',
      data: {
        reminder_title: title,
        created: true,
      },
      latency_ms: Date.now() - started,
    };
  }

  return {
    type: 'skill_result',
    request_id: message.request_id,
    skill_id: message.skill_id,
    status: 'SUCCESS',
    data: {
      executed_locally: true,
      skill_id: message.skill_id,
    },
    latency_ms: Date.now() - started,
  };
}

function queueResult(result: SkillResultMessage): void {
  pruneExpiredQueue();
  state.queue.push({ queuedAt: Date.now(), result });
  logEvent('edge_agent_result_queued', {
    request_id: result.request_id,
    skill_id: result.skill_id,
    queue_size: state.queue.length,
  });
}

function flushQueuedResults(ws: WebSocket): void {
  pruneExpiredQueue();
  if (state.queue.length === 0 || ws.readyState !== WebSocket.OPEN) {
    return;
  }

  while (state.queue.length > 0) {
    const next = state.queue.shift();
    if (!next) {
      break;
    }
    sendMessage(ws, next.result);
  }
}

function pruneExpiredQueue(): void {
  const now = Date.now();
  state.queue = state.queue.filter((entry) => now - entry.queuedAt <= config.maxQueueAgeMs);
}

function sendMessage(ws: WebSocket, payload: RegisterMessage | HeartbeatMessage | SkillResultMessage): void {
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
  const supportedSkills = (env.EDGE_SUPPORTED_SKILLS ?? 'voice-wake-say,apple-remind-me,apple-notes-skill')
    .split(',')
    .map((skill) => skill.trim().toLowerCase())
    .filter((skill) => skill !== '');

  return {
    serviceName: 'brevio-edge-agent',
    serviceVersion: env.SERVICE_VERSION?.trim() || '0.1.0',
    environment: env.BREVIO_ENV?.trim() || 'local',
    relayUrl: env.EDGE_RELAY_URL?.trim() || 'ws://127.0.0.1:8086/ws/edge',
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
    checks.queue_size = state.queue.length;
    checks.last_connected_at = state.lastConnectedAt ? new Date(state.lastConnectedAt).toISOString() : null;
    checks.last_disconnected_at = state.lastDisconnectedAt ? new Date(state.lastDisconnectedAt).toISOString() : null;
    checks.last_heartbeat_at = state.lastHeartbeatAt ? new Date(state.lastHeartbeatAt).toISOString() : null;
    checks.max_queue_age_ms = config.maxQueueAgeMs;
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
