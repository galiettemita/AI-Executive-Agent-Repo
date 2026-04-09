import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export type SkillStatus = 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT';
export type ExecutionLifecycleStatus = 'QUEUED' | 'WAITING_FOR_AGENT' | 'DISPATCHED' | 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT' | 'REJECTED';
export type WorkflowReportStatus = 'PENDING' | 'RETRYING' | 'DELEGATED' | 'FAILED';

export interface ExecutionRefs {
  runId?: string;
  taskId?: string;
  stepId?: string;
  attempt?: number;
}

export interface ExecutionCreateInput extends ExecutionRefs {
  requestId: string;
  userId: string;
  deviceId: string;
  skillId: string;
}

export interface ExecutionResultUpdate extends ExecutionRefs {
  requestId: string;
  skillId: string;
  status: SkillStatus;
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
  latencyMs: number;
}

export interface WorkflowReportState {
  status: WorkflowReportStatus;
  attempts: number;
  updatedAt: number;
  nextRetryAt?: number;
  warning?: string;
}

export interface ExecutionRecord extends ExecutionRefs {
  requestId: string;
  userId: string;
  deviceId: string;
  skillId: string;
  status: ExecutionLifecycleStatus;
  createdAt: number;
  updatedAt: number;
  queuedAt?: number;
  dispatchedAt?: number;
  completedAt?: number;
  result?: {
    status: SkillStatus;
    data?: Record<string, unknown>;
    error?: {
      code: string;
      message: string;
    };
    latencyMs: number;
  };
  lastError?: {
    code: string;
    message: string;
  };
  workflowReport?: WorkflowReportState;
}

export interface ApplyResultResponse {
  outcome: 'applied' | 'duplicate' | 'unknown_request' | 'skill_mismatch' | 'ref_mismatch';
  record?: ExecutionRecord;
}

const TERMINAL_STATUSES = new Set<ExecutionLifecycleStatus>(['SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT', 'REJECTED']);

function cloneRecord(record: ExecutionRecord): ExecutionRecord {
  return {
    ...record,
    result: record.result
      ? {
          ...record.result,
          data: record.result.data ? { ...record.result.data } : undefined,
          error: record.result.error ? { ...record.result.error } : undefined
        }
      : undefined,
    lastError: record.lastError ? { ...record.lastError } : undefined,
    workflowReport: record.workflowReport ? { ...record.workflowReport } : undefined
  };
}

function refsMatch(record: ExecutionRecord, update: ExecutionRefs): boolean {
  if (record.runId && update.runId && record.runId !== update.runId) {
    return false;
  }
  if (record.taskId && update.taskId && record.taskId !== update.taskId) {
    return false;
  }
  if (record.stepId && update.stepId && record.stepId !== update.stepId) {
    return false;
  }
  if (record.attempt !== undefined && update.attempt !== undefined && record.attempt !== update.attempt) {
    return false;
  }
  return true;
}

export class ExecutionStore {
  private readonly records: Map<string, ExecutionRecord>;
  private readonly filePath?: string;

  constructor(filePath?: string) {
    this.filePath = filePath;
    this.records = this.loadSnapshot();
  }

  create(input: ExecutionCreateInput, initialStatus: Extract<ExecutionLifecycleStatus, 'QUEUED' | 'WAITING_FOR_AGENT' | 'DISPATCHED'>, nowMs: number): ExecutionRecord {
    const record: ExecutionRecord = {
      requestId: input.requestId,
      userId: input.userId,
      deviceId: input.deviceId,
      skillId: input.skillId,
      runId: input.runId,
      taskId: input.taskId,
      stepId: input.stepId,
      attempt: input.attempt,
      status: initialStatus,
      createdAt: nowMs,
      updatedAt: nowMs,
      queuedAt: initialStatus === 'DISPATCHED' ? undefined : nowMs,
      dispatchedAt: initialStatus === 'DISPATCHED' ? nowMs : undefined
    };
    this.records.set(record.requestId, record);
    this.persist();
    return cloneRecord(record);
  }

  get(requestId: string): ExecutionRecord | null {
    const record = this.records.get(requestId);
    return record ? cloneRecord(record) : null;
  }

  list(limit = 100): ExecutionRecord[] {
    return Array.from(this.records.values())
      .sort((left, right) => right.updatedAt - left.updatedAt)
      .slice(0, Math.max(1, limit))
      .map((record) => cloneRecord(record));
  }

  pendingWorkflowReports(nowMs: number): ExecutionRecord[] {
    return Array.from(this.records.values())
      .filter((record) => {
        const state = record.workflowReport;
        if (!state) {
          return false;
        }
        if (state.status !== 'PENDING' && state.status !== 'RETRYING') {
          return false;
        }
        return state.nextRetryAt === undefined || state.nextRetryAt <= nowMs;
      })
      .sort((left, right) => (left.workflowReport?.nextRetryAt ?? 0) - (right.workflowReport?.nextRetryAt ?? 0))
      .map((record) => cloneRecord(record));
  }

  markQueued(requestId: string, nowMs: number, status: Extract<ExecutionLifecycleStatus, 'QUEUED' | 'WAITING_FOR_AGENT'> = 'QUEUED'): ExecutionRecord | null {
    const record = this.records.get(requestId);
    if (!record || TERMINAL_STATUSES.has(record.status)) {
      return record ? cloneRecord(record) : null;
    }
    record.status = status;
    record.updatedAt = nowMs;
    record.queuedAt = record.queuedAt ?? nowMs;
    this.persist();
    return cloneRecord(record);
  }

  markDispatched(requestId: string, nowMs: number): ExecutionRecord | null {
    const record = this.records.get(requestId);
    if (!record || TERMINAL_STATUSES.has(record.status)) {
      return record ? cloneRecord(record) : null;
    }
    record.status = 'DISPATCHED';
    record.updatedAt = nowMs;
    record.dispatchedAt = nowMs;
    record.queuedAt = record.queuedAt ?? nowMs;
    this.persist();
    return cloneRecord(record);
  }

  markRejected(requestId: string, code: string, message: string, nowMs: number): ExecutionRecord | null {
    const record = this.records.get(requestId);
    if (!record) {
      return null;
    }
    if (TERMINAL_STATUSES.has(record.status)) {
      return cloneRecord(record);
    }
    record.status = 'REJECTED';
    record.updatedAt = nowMs;
    record.completedAt = nowMs;
    record.lastError = { code, message };
    this.markWorkflowReportPendingForRecord(record, nowMs);
    this.persist();
    return cloneRecord(record);
  }

  applyResult(update: ExecutionResultUpdate, nowMs: number): ApplyResultResponse {
    const record = this.records.get(update.requestId);
    if (!record) {
      return { outcome: 'unknown_request' };
    }
    if (record.skillId !== update.skillId) {
      return { outcome: 'skill_mismatch', record: cloneRecord(record) };
    }
    if (!refsMatch(record, update)) {
      return { outcome: 'ref_mismatch', record: cloneRecord(record) };
    }
    if (TERMINAL_STATUSES.has(record.status)) {
      return { outcome: 'duplicate', record: cloneRecord(record) };
    }

    record.status = update.status;
    record.updatedAt = nowMs;
    record.completedAt = nowMs;
    record.result = {
      status: update.status,
      data: update.data,
      error: update.error,
      latencyMs: update.latencyMs
    };
    if (update.error) {
      record.lastError = { ...update.error };
    }
    this.markWorkflowReportPendingForRecord(record, nowMs);
    this.persist();
    return { outcome: 'applied', record: cloneRecord(record) };
  }

  markWorkflowReportOutcome(
    requestId: string,
    delegated: boolean,
    nowMs: number,
    warning: string | undefined,
    nextRetryAt?: number,
    maxAttempts = 5
  ): ExecutionRecord | null {
    const record = this.records.get(requestId);
    if (!record || !record.workflowReport) {
      return record ? cloneRecord(record) : null;
    }
    const attempts = record.workflowReport.attempts + 1;
    if (delegated) {
      record.workflowReport = {
        status: 'DELEGATED',
        attempts,
        updatedAt: nowMs
      };
    } else {
      record.workflowReport = {
        status: attempts >= maxAttempts ? 'FAILED' : 'RETRYING',
        attempts,
        updatedAt: nowMs,
        nextRetryAt: attempts >= maxAttempts ? undefined : nextRetryAt,
        warning
      };
    }
    record.updatedAt = nowMs;
    this.persist();
    return cloneRecord(record);
  }

  stats(): { total: number; active: number; terminal: number; pendingWorkflowReports: number } {
    let terminal = 0;
    let pendingWorkflowReports = 0;
    for (const record of this.records.values()) {
      if (TERMINAL_STATUSES.has(record.status)) {
        terminal += 1;
      }
      if (record.workflowReport && (record.workflowReport.status === 'PENDING' || record.workflowReport.status === 'RETRYING')) {
        pendingWorkflowReports += 1;
      }
    }
    return {
      total: this.records.size,
      active: this.records.size - terminal,
      terminal,
      pendingWorkflowReports
    };
  }

  private markWorkflowReportPendingForRecord(record: ExecutionRecord, nowMs: number): void {
    if (!record.runId || !record.stepId || !TERMINAL_STATUSES.has(record.status)) {
      return;
    }
    record.workflowReport = {
      status: 'PENDING',
      attempts: 0,
      updatedAt: nowMs
    };
  }

  private loadSnapshot(): Map<string, ExecutionRecord> {
    if (!this.filePath || !existsSync(this.filePath)) {
      return new Map();
    }
    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as { records?: ExecutionRecord[] };
      const records = Array.isArray(parsed?.records) ? parsed.records : [];
      return new Map(records.map((record) => [record.requestId, record]));
    } catch {
      return new Map();
    }
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }
    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.tmp`;
    writeFileSync(
      tmpPath,
      JSON.stringify(
        {
          version: 1,
          records: Array.from(this.records.values())
        },
        null,
        2
      ),
      'utf8'
    );
    renameSync(tmpPath, this.filePath);
  }
}
