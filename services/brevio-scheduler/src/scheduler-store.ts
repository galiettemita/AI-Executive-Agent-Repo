import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export type JobStatus = 'active' | 'paused' | 'disabled';

export interface ScheduledJob {
  id: string;
  user_id: string;
  skill_id: string;
  schedule: string;
  timezone: string;
  status: JobStatus;
  payload: Record<string, unknown>;
  last_run_at?: string;
  next_run_at: string;
  created_at: string;
  updated_at: string;
}

export interface TriggerEvent {
  id: string;
  user_id: string;
  skill_id: string;
  payload: Record<string, unknown>;
  status: 'queued' | 'dispatched' | 'failed';
  created_at: string;
}

interface SchedulerSnapshot {
  version: 1;
  jobs: Array<[string, ScheduledJob]>;
  triggers: TriggerEvent[];
}

function cloneJob(job: ScheduledJob): ScheduledJob {
  return {
    ...job,
    payload: { ...job.payload }
  };
}

function cloneTrigger(trigger: TriggerEvent): TriggerEvent {
  return {
    ...trigger,
    payload: { ...trigger.payload }
  };
}

export class SchedulerStore {
  private readonly jobs: Map<string, ScheduledJob>;
  private readonly triggers: TriggerEvent[];
  private readonly filePath?: string;

  constructor(filePath?: string) {
    this.filePath = filePath;
    const snapshot = this.loadSnapshot();
    this.jobs = snapshot.jobs;
    this.triggers = snapshot.triggers;
  }

  mode(): 'in_memory' | 'local_file_snapshot' {
    return this.filePath ? 'local_file_snapshot' : 'in_memory';
  }

  snapshotPath(): string | undefined {
    return this.filePath;
  }

  stats(): { jobs: number; triggers: number; queuedTriggers: number } {
    return {
      jobs: this.jobs.size,
      triggers: this.triggers.length,
      queuedTriggers: this.triggers.filter((trigger) => trigger.status === 'queued').length
    };
  }

  jobCount(): number {
    return this.jobs.size;
  }

  listJobs(): ScheduledJob[] {
    return Array.from(this.jobs.values()).map((job) => cloneJob(job));
  }

  listTriggers(): TriggerEvent[] {
    return this.triggers.map((trigger) => cloneTrigger(trigger));
  }

  getJob(id: string): ScheduledJob | null {
    const job = this.jobs.get(id);
    return job ? cloneJob(job) : null;
  }

  saveJob(job: ScheduledJob): ScheduledJob {
    this.jobs.set(job.id, cloneJob(job));
    this.persist();
    return cloneJob(job);
  }

  appendTrigger(trigger: TriggerEvent, maxTriggers: number): TriggerEvent {
    this.triggers.unshift(cloneTrigger(trigger));
    if (this.triggers.length > maxTriggers) {
      this.triggers.length = maxTriggers;
    }
    this.persist();
    return cloneTrigger(trigger);
  }

  private loadSnapshot(): { jobs: Map<string, ScheduledJob>; triggers: TriggerEvent[] } {
    if (!this.filePath || !existsSync(this.filePath)) {
      return {
        jobs: new Map(),
        triggers: []
      };
    }

    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<SchedulerSnapshot>;
      if (!parsed || typeof parsed !== 'object') {
        throw new Error('snapshot must be a JSON object');
      }
      if ('version' in parsed && parsed.version !== 1) {
        throw new Error(`unsupported snapshot version: ${String(parsed.version)}`);
      }
      if ('jobs' in parsed && !Array.isArray(parsed.jobs)) {
        throw new Error('snapshot jobs must be an array');
      }
      if ('triggers' in parsed && !Array.isArray(parsed.triggers)) {
        throw new Error('snapshot triggers must be an array');
      }

      const jobs = new Map<string, ScheduledJob>();
      for (const entry of parsed.jobs ?? []) {
        if (!Array.isArray(entry) || entry.length !== 2 || typeof entry[0] !== 'string' || !entry[0].trim()) {
          throw new Error('snapshot job entry is invalid');
        }
        jobs.set(entry[0], this.validateJob(entry[1]));
      }

      const triggers = (parsed.triggers ?? []).map((trigger) => this.validateTrigger(trigger));
      return { jobs, triggers };
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`scheduler state snapshot is corrupt at ${this.filePath}: ${detail}`);
    }
  }

  private validateJob(value: unknown): ScheduledJob {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      throw new Error('snapshot job payload is invalid');
    }
    const job = value as Partial<ScheduledJob>;
    if (
      typeof job.id !== 'string' ||
      !job.id.trim() ||
      typeof job.user_id !== 'string' ||
      !job.user_id.trim() ||
      typeof job.skill_id !== 'string' ||
      !job.skill_id.trim() ||
      typeof job.schedule !== 'string' ||
      !job.schedule.trim() ||
      typeof job.timezone !== 'string' ||
      !job.timezone.trim() ||
      (job.status !== 'active' && job.status !== 'paused' && job.status !== 'disabled') ||
      !job.payload ||
      typeof job.payload !== 'object' ||
      Array.isArray(job.payload) ||
      typeof job.next_run_at !== 'string' ||
      !job.next_run_at.trim() ||
      typeof job.created_at !== 'string' ||
      !job.created_at.trim() ||
      typeof job.updated_at !== 'string' ||
      !job.updated_at.trim()
    ) {
      throw new Error('snapshot job payload is invalid');
    }
    return cloneJob(job as ScheduledJob);
  }

  private validateTrigger(value: unknown): TriggerEvent {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      throw new Error('snapshot trigger payload is invalid');
    }
    const trigger = value as Partial<TriggerEvent>;
    if (
      typeof trigger.id !== 'string' ||
      !trigger.id.trim() ||
      typeof trigger.user_id !== 'string' ||
      !trigger.user_id.trim() ||
      typeof trigger.skill_id !== 'string' ||
      !trigger.skill_id.trim() ||
      (trigger.status !== 'queued' && trigger.status !== 'dispatched' && trigger.status !== 'failed') ||
      !trigger.payload ||
      typeof trigger.payload !== 'object' ||
      Array.isArray(trigger.payload) ||
      typeof trigger.created_at !== 'string' ||
      !trigger.created_at.trim()
    ) {
      throw new Error('snapshot trigger payload is invalid');
    }
    return cloneTrigger(trigger as TriggerEvent);
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }

    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.${process.pid}.tmp`;
    const snapshot: SchedulerSnapshot = {
      version: 1,
      jobs: Array.from(this.jobs.entries()),
      triggers: this.triggers.map((trigger) => cloneTrigger(trigger))
    };
    writeFileSync(tmpPath, JSON.stringify(snapshot, null, 2), 'utf8');
    renameSync(tmpPath, this.filePath);
  }
}
