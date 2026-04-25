import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

import type { ProtectedInputEnvelope } from './security.js';

export interface PersistedQueuedExecution {
  requestId: string;
  userId: string;
  deviceId: string;
  skillId: string;
  runId?: string;
  taskId?: string;
  stepId?: string;
  attempt?: number;
  protectedInput: ProtectedInputEnvelope;
  queuedAt: number;
}

export interface PersistedSessionRecord {
  userId: string;
  deviceId: string;
  deviceName: string;
  certFingerprint: string;
  connectedAt: number;
  lastSeenAt: number;
  supportedSkills: string[];
  authBound: boolean;
  allowedSkills: string[];
  connected: boolean;
}

interface QueueSnapshotRecord {
  key: string;
  executions: PersistedQueuedExecution[];
}

interface RelayStateSnapshot {
  version: 1;
  sessions: Array<{ key: string; session: PersistedSessionRecord }>;
  offlineQueues: QueueSnapshotRecord[];
}

function cloneExecution(execution: PersistedQueuedExecution): PersistedQueuedExecution {
  return {
    requestId: execution.requestId,
    userId: execution.userId,
    deviceId: execution.deviceId,
    skillId: execution.skillId,
    runId: execution.runId,
    taskId: execution.taskId,
    stepId: execution.stepId,
    attempt: execution.attempt,
    protectedInput: {
      alg: execution.protectedInput.alg,
      nonce: execution.protectedInput.nonce,
      ciphertext: execution.protectedInput.ciphertext
    },
    queuedAt: execution.queuedAt
  };
}

function cloneSession(session: PersistedSessionRecord): PersistedSessionRecord {
  return {
    userId: session.userId,
    deviceId: session.deviceId,
    deviceName: session.deviceName,
    certFingerprint: session.certFingerprint,
    connectedAt: session.connectedAt,
    lastSeenAt: session.lastSeenAt,
    supportedSkills: [...session.supportedSkills],
    authBound: session.authBound,
    allowedSkills: [...session.allowedSkills],
    connected: session.connected
  };
}

export class RelayStateStore {
  private readonly filePath?: string;
  private readonly sessions: Map<string, PersistedSessionRecord>;
  private readonly offlineQueues: Map<string, PersistedQueuedExecution[]>;

  constructor(filePath?: string) {
    this.filePath = filePath ? path.resolve(filePath) : undefined;
    const snapshot = this.loadSnapshot();
    this.sessions = snapshot.sessions;
    this.offlineQueues = snapshot.offlineQueues;
    this.markLoadedSessionsDisconnected();
  }

  mode(): 'in_memory' | 'local_file_snapshot' {
    return this.filePath ? 'local_file_snapshot' : 'in_memory';
  }

  snapshotPath(): string | undefined {
    return this.filePath;
  }

  connectedSessions(): PersistedSessionRecord[] {
    return Array.from(this.sessions.values())
      .filter((session) => session.connected)
      .sort((left, right) => left.connectedAt - right.connectedAt)
      .map((session) => cloneSession(session));
  }

  connectedSessionCount(): number {
    let count = 0;
    for (const session of this.sessions.values()) {
      if (session.connected) {
        count += 1;
      }
    }
    return count;
  }

  trackedSessionCount(): number {
    return this.sessions.size;
  }

  upsertSession(key: string, session: PersistedSessionRecord): PersistedSessionRecord {
    this.sessions.set(key, cloneSession(session));
    this.persist();
    return cloneSession(session);
  }

  touchSession(key: string, lastSeenAt: number): PersistedSessionRecord | null {
    const session = this.sessions.get(key);
    if (!session) {
      return null;
    }
    session.lastSeenAt = lastSeenAt;
    this.persist();
    return cloneSession(session);
  }

  disconnectSession(key: string, lastSeenAt: number): PersistedSessionRecord | null {
    const session = this.sessions.get(key);
    if (!session) {
      return null;
    }
    session.connected = false;
    if (lastSeenAt > 0) {
      session.lastSeenAt = lastSeenAt;
    }
    this.persist();
    return cloneSession(session);
  }

  enqueueExecution(
    key: string,
    execution: PersistedQueuedExecution,
    maxQueuePerDevice: number,
    maxQueueAgeMs: number,
    nowMs: number
  ): PersistedQueuedExecution[] {
    const queue = this.prunedQueue(key, nowMs, maxQueueAgeMs);
    queue.push(cloneExecution(execution));
    while (queue.length > maxQueuePerDevice) {
      queue.shift();
    }
    this.offlineQueues.set(key, queue);
    this.persist();
    return queue.map((item) => cloneExecution(item));
  }

  queuedCount(nowMs: number, maxQueueAgeMs: number): number {
    let total = 0;
    for (const key of this.offlineQueues.keys()) {
      total += this.prunedQueue(key, nowMs, maxQueueAgeMs).length;
    }
    return total;
  }

  takeQueue(key: string, nowMs: number, maxQueueAgeMs: number): PersistedQueuedExecution[] {
    const queue = this.prunedQueue(key, nowMs, maxQueueAgeMs);
    if (queue.length === 0) {
      this.offlineQueues.delete(key);
      this.persist();
      return [];
    }
    this.offlineQueues.delete(key);
    this.persist();
    return queue.map((item) => cloneExecution(item));
  }

  queueFor(key: string, nowMs: number, maxQueueAgeMs: number): PersistedQueuedExecution[] {
    return this.prunedQueue(key, nowMs, maxQueueAgeMs).map((item) => cloneExecution(item));
  }

  private markLoadedSessionsDisconnected(): void {
    let changed = false;
    for (const session of this.sessions.values()) {
      if (session.connected) {
        session.connected = false;
        changed = true;
      }
    }
    if (changed) {
      this.persist();
    }
  }

  private prunedQueue(key: string, nowMs: number, maxQueueAgeMs: number): PersistedQueuedExecution[] {
    const existing = this.offlineQueues.get(key) ?? [];
    const filtered = existing.filter((item) => nowMs-item.queuedAt <= maxQueueAgeMs);
    if (filtered.length === 0) {
      if (existing.length > 0) {
        this.offlineQueues.delete(key);
        this.persist();
      }
      return [];
    }
    if (filtered.length !== existing.length) {
      this.offlineQueues.set(key, filtered);
      this.persist();
    }
    return filtered;
  }

  private loadSnapshot(): {
    sessions: Map<string, PersistedSessionRecord>;
    offlineQueues: Map<string, PersistedQueuedExecution[]>;
  } {
    if (!this.filePath || !existsSync(this.filePath)) {
      return {
        sessions: new Map(),
        offlineQueues: new Map()
      };
    }

    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<RelayStateSnapshot>;
      if (!parsed || typeof parsed !== 'object') {
        throw new Error('snapshot must be a JSON object');
      }
      if (parsed.version !== 1) {
        throw new Error(`unsupported snapshot version: ${String(parsed.version)}`);
      }
      if (!Array.isArray(parsed.sessions)) {
        throw new Error('sessions must be an array');
      }
      if (!Array.isArray(parsed.offlineQueues)) {
        throw new Error('offlineQueues must be an array');
      }

      const sessions = new Map<string, PersistedSessionRecord>();
      for (const entry of parsed.sessions) {
        if (!entry || typeof entry !== 'object' || typeof entry.key !== 'string') {
          throw new Error('session entry is missing key');
        }
        this.assertSession(entry.session);
        sessions.set(entry.key, cloneSession(entry.session));
      }

      const offlineQueues = new Map<string, PersistedQueuedExecution[]>();
      for (const entry of parsed.offlineQueues) {
        if (!entry || typeof entry !== 'object' || typeof entry.key !== 'string' || !Array.isArray(entry.executions)) {
          throw new Error('offline queue entry is invalid');
        }
        const executions = entry.executions.map((execution) => {
          this.assertExecution(execution);
          return cloneExecution(execution);
        });
        offlineQueues.set(entry.key, executions);
      }

      return { sessions, offlineQueues }
    } catch (error) {
      throw new Error(
        `relay state snapshot is corrupt at ${this.filePath}: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }

    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.tmp`;
    const snapshot: RelayStateSnapshot = {
      version: 1,
      sessions: Array.from(this.sessions.entries(), ([key, session]) => ({
        key,
        session: cloneSession(session)
      })),
      offlineQueues: Array.from(this.offlineQueues.entries(), ([key, executions]) => ({
        key,
        executions: executions.map((execution) => cloneExecution(execution))
      }))
    };
    writeFileSync(tmpPath, JSON.stringify(snapshot, null, 2), 'utf8');
    renameSync(tmpPath, this.filePath);
  }

  private assertSession(value: unknown): asserts value is PersistedSessionRecord {
    if (!value || typeof value !== 'object') {
      throw new Error('session must be an object');
    }
    const session = value as Partial<PersistedSessionRecord>;
    if (typeof session.userId !== 'string' || session.userId.trim() === '') {
      throw new Error('session.userId must be a non-empty string');
    }
    if (typeof session.deviceId !== 'string' || session.deviceId.trim() === '') {
      throw new Error('session.deviceId must be a non-empty string');
    }
    if (typeof session.deviceName !== 'string') {
      throw new Error('session.deviceName must be a string');
    }
    if (typeof session.certFingerprint !== 'string') {
      throw new Error('session.certFingerprint must be a string');
    }
    if (!Number.isInteger(session.connectedAt) || session.connectedAt <= 0) {
      throw new Error('session.connectedAt must be a positive integer');
    }
    if (!Number.isInteger(session.lastSeenAt) || session.lastSeenAt <= 0) {
      throw new Error('session.lastSeenAt must be a positive integer');
    }
    if (!Array.isArray(session.supportedSkills) || !session.supportedSkills.every((skill) => typeof skill === 'string')) {
      throw new Error('session.supportedSkills must be a string array');
    }
    if (typeof session.authBound !== 'boolean') {
      throw new Error('session.authBound must be a boolean');
    }
    if (!Array.isArray(session.allowedSkills) || !session.allowedSkills.every((skill) => typeof skill === 'string')) {
      throw new Error('session.allowedSkills must be a string array');
    }
    if (typeof session.connected !== 'boolean') {
      throw new Error('session.connected must be a boolean');
    }
  }

  private assertExecution(value: unknown): asserts value is PersistedQueuedExecution {
    if (!value || typeof value !== 'object') {
      throw new Error('queue execution must be an object');
    }
    const execution = value as Partial<PersistedQueuedExecution>;
    if (typeof execution.requestId !== 'string' || execution.requestId.trim() === '') {
      throw new Error('execution.requestId must be a non-empty string');
    }
    if (typeof execution.userId !== 'string' || execution.userId.trim() === '') {
      throw new Error('execution.userId must be a non-empty string');
    }
    if (typeof execution.deviceId !== 'string' || execution.deviceId.trim() === '') {
      throw new Error('execution.deviceId must be a non-empty string');
    }
    if (typeof execution.skillId !== 'string' || execution.skillId.trim() === '') {
      throw new Error('execution.skillId must be a non-empty string');
    }
    if (!Number.isInteger(execution.queuedAt) || execution.queuedAt <= 0) {
      throw new Error('execution.queuedAt must be a positive integer');
    }
    if (!execution.protectedInput || typeof execution.protectedInput !== 'object') {
      throw new Error('execution.protectedInput must be an object');
    }
    if (execution.protectedInput.alg !== 'aes-256-gcm') {
      throw new Error('execution.protectedInput.alg must be aes-256-gcm');
    }
    if (typeof execution.protectedInput.nonce !== 'string' || typeof execution.protectedInput.ciphertext !== 'string') {
      throw new Error('execution.protectedInput must include nonce and ciphertext');
    }
    if (execution.attempt !== undefined && (!Number.isInteger(execution.attempt) || execution.attempt <= 0)) {
      throw new Error('execution.attempt must be a positive integer');
    }
  }
}
