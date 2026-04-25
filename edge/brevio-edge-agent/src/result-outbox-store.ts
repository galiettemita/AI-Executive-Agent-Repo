import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export interface PersistedSkillResultMessage {
  type: 'skill_result';
  request_id: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
  skill_id: string;
  status: string;
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
  latency_ms: number;
  dispatch_receipt_id: string;
  result_receipt_id: string;
}

export interface OutboxRecord {
  requestId: string;
  resultReceiptId: string;
  queuedAt: number;
  sentAt?: number;
  result: PersistedSkillResultMessage;
}

interface Snapshot {
  version: 1;
  records: OutboxRecord[];
}

function cloneRecord(record: OutboxRecord): OutboxRecord {
  return {
    requestId: record.requestId,
    resultReceiptId: record.resultReceiptId,
    queuedAt: record.queuedAt,
    sentAt: record.sentAt,
    result: {
      ...record.result,
      data: record.result.data ? { ...record.result.data } : undefined,
      error: record.result.error ? { ...record.result.error } : undefined
    }
  };
}

export class ResultOutboxStore {
  private readonly filePath?: string;
  private records: OutboxRecord[];

  constructor(filePath?: string) {
    this.filePath = filePath ? path.resolve(filePath) : undefined;
    this.records = this.loadSnapshot();
  }

  mode(): 'in_memory' | 'local_file_snapshot' {
    return this.filePath ? 'local_file_snapshot' : 'in_memory';
  }

  snapshotPath(): string | undefined {
    return this.filePath;
  }

  size(nowMs: number, maxQueueAgeMs: number): number {
    return this.pending(nowMs, maxQueueAgeMs).length;
  }

  enqueue(record: OutboxRecord): void {
    this.records = this.records.filter((existing) => existing.resultReceiptId !== record.resultReceiptId);
    this.records.push(cloneRecord(record));
    this.persist();
  }

  pending(nowMs: number, maxQueueAgeMs: number): OutboxRecord[] {
    const filtered = this.records.filter((record) => nowMs - record.queuedAt <= maxQueueAgeMs);
    if (filtered.length !== this.records.length) {
      this.records = filtered;
      this.persist();
    }
    return filtered.map((record) => cloneRecord(record)).sort((left, right) => left.queuedAt - right.queuedAt);
  }

  markSent(resultReceiptId: string, nowMs: number): OutboxRecord | null {
    const record = this.records.find((entry) => entry.resultReceiptId === resultReceiptId);
    if (!record) {
      return null;
    }
    record.sentAt = nowMs;
    this.persist();
    return cloneRecord(record);
  }

  markAcked(resultReceiptId: string): OutboxRecord | null {
    const index = this.records.findIndex((entry) => entry.resultReceiptId === resultReceiptId);
    if (index === -1) {
      return null;
    }
    const [record] = this.records.splice(index, 1);
    this.persist();
    return cloneRecord(record);
  }

  private loadSnapshot(): OutboxRecord[] {
    if (!this.filePath || !existsSync(this.filePath)) {
      return [];
    }
    try {
      const parsed = JSON.parse(readFileSync(this.filePath, 'utf8')) as Partial<Snapshot>;
      if (parsed.version !== 1 || !Array.isArray(parsed.records)) {
        throw new Error('invalid snapshot');
      }
      return parsed.records.map((record) => cloneRecord(record));
    } catch (error) {
      throw new Error(
        `edge result outbox snapshot is corrupt at ${this.filePath}: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }
    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.tmp`;
    const snapshot: Snapshot = {
      version: 1,
      records: this.records.map((record) => cloneRecord(record))
    };
    writeFileSync(tmpPath, JSON.stringify(snapshot, null, 2), 'utf8');
    renameSync(tmpPath, this.filePath);
  }
}
