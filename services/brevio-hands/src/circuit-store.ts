import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export type CircuitBreakerState = 'CLOSED' | 'HALF_OPEN' | 'OPEN';

export interface CircuitStateEntry {
  state: CircuitBreakerState;
  failureCount: number;
  openedAtMs?: number;
  halfOpenRemaining: number;
  updatedAtMs: number;
}

interface CircuitSnapshotRecord extends CircuitStateEntry {
  skillId: string;
}

interface CircuitSnapshot {
  version: 1;
  circuits: CircuitSnapshotRecord[];
}

function cloneEntry(entry: CircuitStateEntry): CircuitStateEntry {
  return {
    state: entry.state,
    failureCount: entry.failureCount,
    openedAtMs: entry.openedAtMs,
    halfOpenRemaining: entry.halfOpenRemaining,
    updatedAtMs: entry.updatedAtMs
  };
}

export class CircuitStore {
  private readonly stateFilePath?: string;
  private readonly circuits: Map<string, CircuitStateEntry>;

  constructor(stateFilePath?: string) {
    this.stateFilePath = stateFilePath ? path.resolve(stateFilePath) : undefined;
    this.circuits = this.loadSnapshot();
  }

  mode(): 'memory' | 'local_file_snapshot' {
    return this.stateFilePath ? 'local_file_snapshot' : 'memory';
  }

  snapshotPath(): string | undefined {
    return this.stateFilePath;
  }

  size(): number {
    return this.circuits.size;
  }

  entries(): Array<[string, CircuitStateEntry]> {
    return Array.from(this.circuits.entries(), ([skillId, entry]) => [skillId, cloneEntry(entry)]);
  }

  get(skillId: string, halfOpenMaxCalls: number, nowMs = Date.now()): CircuitStateEntry {
    return cloneEntry(this.ensure(skillId, halfOpenMaxCalls, nowMs));
  }

  update(
    skillId: string,
    halfOpenMaxCalls: number,
    mutate: (entry: CircuitStateEntry) => void,
    nowMs = Date.now()
  ): CircuitStateEntry {
    const entry = this.ensure(skillId, halfOpenMaxCalls, nowMs);
    mutate(entry);
    this.persist();
    return cloneEntry(entry);
  }

  private ensure(skillId: string, halfOpenMaxCalls: number, nowMs: number): CircuitStateEntry {
    const existing = this.circuits.get(skillId);
    if (existing) {
      return existing;
    }

    const created: CircuitStateEntry = {
      state: 'CLOSED',
      failureCount: 0,
      halfOpenRemaining: halfOpenMaxCalls,
      updatedAtMs: nowMs
    };
    this.circuits.set(skillId, created);
    this.persist();
    return created;
  }

  private loadSnapshot(): Map<string, CircuitStateEntry> {
    if (!this.stateFilePath || !existsSync(this.stateFilePath)) {
      return new Map();
    }

    try {
      const raw = readFileSync(this.stateFilePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<CircuitSnapshot>;
      if (parsed.version !== 1 || !Array.isArray(parsed.circuits)) {
        throw new Error('invalid snapshot envelope');
      }

      const circuits = new Map<string, CircuitStateEntry>();
      for (const record of parsed.circuits) {
        this.assertRecord(record);
        circuits.set(record.skillId, {
          state: record.state,
          failureCount: record.failureCount,
          openedAtMs: record.openedAtMs,
          halfOpenRemaining: record.halfOpenRemaining,
          updatedAtMs: record.updatedAtMs
        });
      }
      return circuits;
    } catch (error) {
      throw new Error(
        `circuit state snapshot is corrupt at ${this.stateFilePath}: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }

  private persist(): void {
    if (!this.stateFilePath) {
      return;
    }

    mkdirSync(path.dirname(this.stateFilePath), { recursive: true });
    const tmpPath = `${this.stateFilePath}.tmp`;
    const snapshot: CircuitSnapshot = {
      version: 1,
      circuits: Array.from(this.circuits.entries(), ([skillId, entry]) => ({
        skillId,
        state: entry.state,
        failureCount: entry.failureCount,
        openedAtMs: entry.openedAtMs,
        halfOpenRemaining: entry.halfOpenRemaining,
        updatedAtMs: entry.updatedAtMs
      }))
    };
    writeFileSync(tmpPath, JSON.stringify(snapshot, null, 2), 'utf8');
    renameSync(tmpPath, this.stateFilePath);
  }

  private assertRecord(record: unknown): asserts record is CircuitSnapshotRecord {
    if (!record || typeof record !== 'object') {
      throw new Error('record must be an object');
    }

    const value = record as Partial<CircuitSnapshotRecord>;
    if (typeof value.skillId !== 'string' || value.skillId.trim() === '') {
      throw new Error('skillId must be a non-empty string');
    }
    if (value.state !== 'CLOSED' && value.state !== 'HALF_OPEN' && value.state !== 'OPEN') {
      throw new Error(`invalid circuit state for ${value.skillId}`);
    }
    if (!Number.isInteger(value.failureCount) || value.failureCount < 0) {
      throw new Error(`invalid failureCount for ${value.skillId}`);
    }
    if (!Number.isInteger(value.halfOpenRemaining) || value.halfOpenRemaining < 0) {
      throw new Error(`invalid halfOpenRemaining for ${value.skillId}`);
    }
    if (!Number.isInteger(value.updatedAtMs) || value.updatedAtMs <= 0) {
      throw new Error(`invalid updatedAtMs for ${value.skillId}`);
    }
    if (value.openedAtMs !== undefined && (!Number.isInteger(value.openedAtMs) || value.openedAtMs <= 0)) {
      throw new Error(`invalid openedAtMs for ${value.skillId}`);
    }
  }
}
