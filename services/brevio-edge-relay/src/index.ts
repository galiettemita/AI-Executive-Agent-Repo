import http from 'node:http';
import { randomUUID } from 'node:crypto';
import { URL } from 'node:url';
import WebSocket, { WebSocketServer } from 'ws';

type SkillStatus = 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT';

interface RelayConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  relayPath: string;
  maxQueueAgeMs: number;
  maxQueuePerDevice: number;
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

type AgentInboundMessage = RegisterMessage | HeartbeatMessage | SkillResultMessage;

interface OutboundAck {
  type: 'ack' | 'error';
  message: string;
  request_id?: string;
}

interface QueuedExecution {
  requestId: string;
  userId: string;
  deviceId: string;
  skillId: string;
  input: Record<string, unknown>;
  queuedAt: number;
}

interface AgentSession {
  socket: WebSocket;
  userId: string;
  deviceId: string;
  deviceName: string;
  certFingerprint: string;
  connectedAt: number;
  lastSeenAt: number;
  supportedSkills: Set<string>;
}

interface ExecuteRequest {
  userId: string;
  deviceId: string;
  skillId: string;
  input: Record<string, unknown>;
}

const config = loadConfig(process.env);
const startedAt = Date.now();
const sessions = new Map<string, AgentSession>();
const offlineQueues = new Map<string, QueuedExecution[]>();

const server = http.createServer((req, res) => {
  if (!req.url) {
    writeJson(res, 400, { error: 'invalid_request', message: 'request url is required' });
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

  if (req.method === 'GET' && req.url === '/v1/edge/sessions') {
    writeJson(res, 200, {
      sessions: Array.from(sessions.values()).map((session) => ({
        user_id: session.userId,
        device_id: session.deviceId,
        device_name: session.deviceName,
        connected_at: new Date(session.connectedAt).toISOString(),
        last_seen_at: new Date(session.lastSeenAt).toISOString(),
        supported_skills: Array.from(session.supportedSkills).sort(),
      })),
    });
    return;
  }

  if (req.method === 'POST' && req.url === '/v1/edge/execute') {
    readJsonBody(req)
      .then((body) => parseExecuteRequest(body))
      .then((executeRequest) => {
        const requestId = randomUUID();
        const execution: QueuedExecution = {
          requestId,
          userId: executeRequest.userId,
          deviceId: executeRequest.deviceId,
          skillId: executeRequest.skillId,
          input: executeRequest.input,
          queuedAt: Date.now(),
        };

        const key = sessionKey(executeRequest.userId, executeRequest.deviceId);
        const session = sessions.get(key);
        if (session && session.socket.readyState === WebSocket.OPEN) {
          dispatchExecution(session, execution);
          writeJson(res, 200, {
            status: 'dispatched',
            request_id: requestId,
          });
          return;
        }

        enqueueExecution(key, execution);
        writeJson(res, 202, {
          status: 'queued',
          request_id: requestId,
          message: 'I need your Mac to be online to do that.',
        });
      })
      .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : 'invalid execute request';
        writeJson(res, 400, { error: 'invalid_request', message });
      });
    return;
  }

  writeJson(res, 404, { error: 'not_found', service: config.serviceName });
});

const wss = new WebSocketServer({ noServer: true });

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
  const userId = requestUrl.searchParams.get('user_id') ?? req.headers['x-user-id'];
  const deviceId = requestUrl.searchParams.get('device_id') ?? req.headers['x-device-id'];

  if (typeof userId !== 'string' || typeof deviceId !== 'string' || userId.trim() === '' || deviceId.trim() === '') {
    sendMessage(socket, {
      type: 'error',
      message: 'user_id and device_id are required for relay session establishment',
    });
    socket.close(1008, 'missing_identity');
    return;
  }

  const key = sessionKey(userId, deviceId);
  const session: AgentSession = {
    socket,
    userId: userId.trim(),
    deviceId: deviceId.trim(),
    deviceName: requestUrl.searchParams.get('device_name') ?? deviceId,
    certFingerprint: normalizeHeader(req.headers['x-client-cert-fingerprint']) ?? 'unknown',
    connectedAt: Date.now(),
    lastSeenAt: Date.now(),
    supportedSkills: new Set<string>(),
  };

  sessions.set(key, session);
  logEvent('edge_agent_connected', {
    user_id: session.userId,
    device_id: session.deviceId,
  });

  sendMessage(socket, {
    type: 'ack',
    message: 'relay connection established',
  });
  flushQueue(key);

  socket.on('message', (payload) => {
    const message = parseAgentInboundMessage(payload.toString());
    if (!message) {
      sendMessage(socket, {
        type: 'error',
        message: 'invalid relay payload',
      });
      return;
    }

    session.lastSeenAt = Date.now();

    if (message.type === 'register') {
      session.deviceName = message.device_name?.trim() || session.deviceName;
      session.certFingerprint = message.client_cert_fingerprint?.trim() || session.certFingerprint;
      session.supportedSkills = new Set((message.supported_skills ?? []).filter((skill) => skill.trim() !== ''));

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

    if (message.type === 'skill_result') {
      logEvent('edge_skill_result_received', {
        user_id: session.userId,
        device_id: session.deviceId,
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
    logEvent('edge_agent_disconnected', {
      user_id: session.userId,
      device_id: session.deviceId,
    });
  });

  socket.on('error', (error) => {
    logEvent('edge_agent_socket_error', {
      user_id: session.userId,
      device_id: session.deviceId,
      error: error.message,
    });
  });
});

server.listen(config.port, () => {
  logEvent('service_started', {
    service: config.serviceName,
    environment: config.environment,
    port: config.port,
    relay_path: config.relayPath,
  });
});

function shutdown(signal: string): void {
  logEvent('shutdown_start', { signal });

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
  return {
    serviceName: 'brevio-edge-relay',
    version: env.SERVICE_VERSION?.trim() || '0.1.0',
    environment: env.BREVIO_ENV?.trim() || 'local',
    port: parseIntWithDefault(env.PORT, 8086),
    relayPath: env.EDGE_RELAY_PATH?.trim() || '/ws/edge',
    maxQueueAgeMs: parseIntWithDefault(env.EDGE_MAX_QUEUE_AGE_MS, 4 * 60 * 60 * 1000),
    maxQueuePerDevice: parseIntWithDefault(env.EDGE_MAX_QUEUE_PER_DEVICE, 100),
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
  const queuedCount = Array.from(offlineQueues.values()).reduce((acc, queue) => acc + queue.length, 0);
  const checks: Record<string, unknown> = {
    process: 'ok',
    connected_agents: sessions.size,
  };

  if (deep) {
    checks.queued_executions = queuedCount;
    checks.max_queue_age_ms = config.maxQueueAgeMs;
    checks.max_queue_per_device = config.maxQueuePerDevice;
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
  pruneExpiredFromQueue(key);
  const queue = offlineQueues.get(key) ?? [];
  queue.push(execution);
  while (queue.length > config.maxQueuePerDevice) {
    queue.shift();
  }
  offlineQueues.set(key, queue);

  logEvent('edge_execution_queued', {
    request_id: execution.requestId,
    user_id: execution.userId,
    device_id: execution.deviceId,
    skill_id: execution.skillId,
  });
}

function pruneExpiredFromQueue(key: string): void {
  const queue = offlineQueues.get(key);
  if (!queue || queue.length === 0) {
    return;
  }

  const now = Date.now();
  const filtered = queue.filter((item) => now-item.queuedAt <= config.maxQueueAgeMs);
  if (filtered.length === 0) {
    offlineQueues.delete(key);
    return;
  }
  offlineQueues.set(key, filtered);
}

function flushQueue(key: string): void {
  pruneExpiredFromQueue(key);
  const queue = offlineQueues.get(key);
  const session = sessions.get(key);

  if (!queue || queue.length === 0 || !session || session.socket.readyState !== WebSocket.OPEN) {
    return;
  }

  while (queue.length > 0) {
    const execution = queue.shift();
    if (!execution) {
      break;
    }
    dispatchExecution(session, execution);
  }

  offlineQueues.delete(key);
}

function dispatchExecution(session: AgentSession, execution: QueuedExecution): void {
  const payload: ExecuteSkillMessage = {
    type: 'execute_skill',
    request_id: execution.requestId,
    skill_id: execution.skillId,
    input: execution.input,
    queued_at: new Date(execution.queuedAt).toISOString(),
  };

  sendMessage(session.socket, payload);
  logEvent('edge_execution_dispatched', {
    request_id: execution.requestId,
    user_id: execution.userId,
    device_id: execution.deviceId,
    skill_id: execution.skillId,
  });
}

function sendMessage(socket: WebSocket, message: OutboundAck | ExecuteSkillMessage): void {
  if (socket.readyState !== WebSocket.OPEN) {
    return;
  }
  socket.send(JSON.stringify(message));
}

function parseExecuteRequest(value: unknown): ExecuteRequest {
  if (!isRecord(value)) {
    throw new Error('request body must be an object');
  }

  const userId = ensureNonEmptyString(value.user_id, 'user_id');
  const deviceId = ensureNonEmptyString(value.device_id, 'device_id');
  const skillId = ensureNonEmptyString(value.skill_id, 'skill_id');

  const inputRaw = value.input;
  const input = isRecord(inputRaw) ? inputRaw : {};

  return {
    userId,
    deviceId,
    skillId,
    input,
  };
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

  if (decoded.type === 'skill_result') {
    const requestId = ensureNonEmptyString(decoded.request_id, 'request_id');
    const skillId = ensureNonEmptyString(decoded.skill_id, 'skill_id');
    const statusRaw = decoded.status;
    if (!isSkillStatus(statusRaw)) {
      throw new Error('status must be one of SUCCESS/PARTIAL/FAILED/TIMEOUT');
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
      skill_id: skillId,
      status: statusRaw,
      data: isRecord(decoded.data) ? decoded.data : undefined,
      error,
      latency_ms: Math.floor(latency),
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
  return value === 'SUCCESS' || value === 'PARTIAL' || value === 'FAILED' || value === 'TIMEOUT';
}

function readJsonBody(req: http.IncomingMessage): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk) => {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
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
