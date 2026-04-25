import { createHash } from 'node:crypto';
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises';
import path from 'node:path';

export type KnowledgeFileName = 'USER.md' | 'SOUL.md' | 'AGENTS.md';

export interface ProfileRecord {
  user_id: string;
  timezone: string;
  locale: string;
  preferences: Record<string, unknown>;
  profile_hash: string;
  created_at: string;
  updated_at: string;
}

interface ProfilePaths {
  profileDir: string;
  profileFile: string;
  knowledgeDir: string;
}

export const KNOWLEDGE_FILES: readonly KnowledgeFileName[] = ['USER.md', 'SOUL.md', 'AGENTS.md'];

function resolveProfilePath(rootDir: string, userId: string): ProfilePaths {
  const profileDir = path.resolve(rootDir, userId);
  return {
    profileDir,
    profileFile: path.join(profileDir, 'profile.json'),
    knowledgeDir: path.join(profileDir, 'knowledge')
  };
}

function stableStringify(value: unknown): string {
  if (value === null || typeof value !== 'object') {
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((entry) => stableStringify(entry)).join(',')}]`;
  }

  const entries = Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left.localeCompare(right));
  return `{${entries
    .map(([key, entryValue]) => `${JSON.stringify(key)}:${stableStringify(entryValue)}`)
    .join(',')}}`;
}

async function readKnowledgeContent(knowledgeDir: string, fileName: KnowledgeFileName): Promise<string> {
  try {
    return await readFile(path.join(knowledgeDir, fileName), 'utf8');
  } catch (error) {
    if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
      return '';
    }
    throw error;
  }
}

function validateProfileRecord(value: unknown, userId: string): ProfileRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('profile file must contain a JSON object');
  }

  const parsed = value as Partial<ProfileRecord>;
  if (typeof parsed.user_id !== 'string' || parsed.user_id.trim() === '' || parsed.user_id !== userId) {
    throw new Error('profile user_id is invalid');
  }
  if (typeof parsed.timezone !== 'string' || parsed.timezone.trim() === '') {
    throw new Error('profile timezone is invalid');
  }
  if (typeof parsed.locale !== 'string' || parsed.locale.trim() === '') {
    throw new Error('profile locale is invalid');
  }
  if (!parsed.preferences || typeof parsed.preferences !== 'object' || Array.isArray(parsed.preferences)) {
    throw new Error('profile preferences must be an object');
  }
  if (typeof parsed.created_at !== 'string' || parsed.created_at.trim() === '') {
    throw new Error('profile created_at is invalid');
  }
  if (typeof parsed.updated_at !== 'string' || parsed.updated_at.trim() === '') {
    throw new Error('profile updated_at is invalid');
  }

  return {
    user_id: parsed.user_id,
    timezone: parsed.timezone,
    locale: parsed.locale,
    preferences: { ...(parsed.preferences as Record<string, unknown>) },
    profile_hash: typeof parsed.profile_hash === 'string' ? parsed.profile_hash : '',
    created_at: parsed.created_at,
    updated_at: parsed.updated_at
  };
}

async function atomicWriteUtf8(filePath: string, content: string): Promise<void> {
  await mkdir(path.dirname(filePath), { recursive: true });
  const tmpPath = `${filePath}.${process.pid}.tmp`;
  await writeFile(tmpPath, content, 'utf8');
  await rename(tmpPath, filePath);
}

export class ProfileStore {
  private readonly rootDir: string;
  private readonly knowledgeFiles: readonly KnowledgeFileName[];
  private readonly userLocks = new Map<string, Promise<void>>();

  constructor(rootDir: string, knowledgeFiles: readonly KnowledgeFileName[] = KNOWLEDGE_FILES) {
    this.rootDir = rootDir;
    this.knowledgeFiles = knowledgeFiles;
  }

  mode(): 'local_file_repository' {
    return 'local_file_repository';
  }

  rootPath(): string {
    return this.rootDir;
  }

  async ensureProfile(userId: string): Promise<ProfileRecord> {
    return await this.withUserLock(userId, async () => {
      const existing = await this.readProfile(userId);
      if (existing) {
        return await this.materializeProfile(userId, existing);
      }

      const now = new Date().toISOString();
      const profile: ProfileRecord = {
        user_id: userId,
        timezone: 'UTC',
        locale: 'en-US',
        preferences: {},
        profile_hash: '',
        created_at: now,
        updated_at: now
      };
      return await this.persistProfile(userId, profile);
    });
  }

  async updatePreferences(
    userId: string,
    update: {
      preferences: Record<string, unknown>;
      timezone?: string;
      locale?: string;
    }
  ): Promise<ProfileRecord> {
    return await this.withUserLock(userId, async () => {
      const current = await this.readOrCreateProfile(userId);
      const next: ProfileRecord = {
        ...current,
        preferences: { ...update.preferences },
        timezone: update.timezone?.trim() ? update.timezone : current.timezone,
        locale: update.locale?.trim() ? update.locale : current.locale,
        updated_at: new Date().toISOString()
      };
      return await this.persistProfile(userId, next);
    });
  }

  async readKnowledge(userId: string, fileName: KnowledgeFileName): Promise<{ profile: ProfileRecord; content: string }> {
    return await this.withUserLock(userId, async () => {
      const profile = await this.readOrCreateProfile(userId);
      const { knowledgeDir } = resolveProfilePath(this.rootDir, userId);
      const content = await readKnowledgeContent(knowledgeDir, fileName);
      return { profile, content };
    });
  }

  async writeKnowledge(userId: string, fileName: KnowledgeFileName, content: string): Promise<ProfileRecord> {
    return await this.withUserLock(userId, async () => {
      const profile = await this.readOrCreateProfile(userId);
      const { knowledgeDir } = resolveProfilePath(this.rootDir, userId);
      const knowledgePath = path.join(knowledgeDir, fileName);
      await atomicWriteUtf8(knowledgePath, content);

      const next: ProfileRecord = {
        ...profile,
        updated_at: new Date().toISOString()
      };
      return await this.persistProfile(userId, next);
    });
  }

  async refreshHash(userId: string): Promise<ProfileRecord> {
    return await this.withUserLock(userId, async () => {
      const current = await this.readOrCreateProfile(userId);
      const refreshed = await this.materializeProfile(userId, current);
      if (refreshed.profile_hash === current.profile_hash) {
        return current;
      }

      const next: ProfileRecord = {
        ...refreshed,
        updated_at: new Date().toISOString()
      };
      return await this.persistProfile(userId, next);
    });
  }

  private async readOrCreateProfile(userId: string): Promise<ProfileRecord> {
    const existing = await this.readProfile(userId);
    if (existing) {
      return await this.materializeProfile(userId, existing);
    }

    const now = new Date().toISOString();
    const profile: ProfileRecord = {
      user_id: userId,
      timezone: 'UTC',
      locale: 'en-US',
      preferences: {},
      profile_hash: '',
      created_at: now,
      updated_at: now
    };
    return await this.persistProfile(userId, profile);
  }

  private async readProfile(userId: string): Promise<ProfileRecord | null> {
    const { profileFile } = resolveProfilePath(this.rootDir, userId);
    try {
      const raw = await readFile(profileFile, 'utf8');
      return validateProfileRecord(JSON.parse(raw), userId);
    } catch (error) {
      if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
        return null;
      }
      throw error;
    }
  }

  private async materializeProfile(userId: string, profile: ProfileRecord): Promise<ProfileRecord> {
    const { knowledgeDir } = resolveProfilePath(this.rootDir, userId);
    const knowledgeByFile: Record<string, string> = {};
    for (const fileName of this.knowledgeFiles) {
      knowledgeByFile[fileName] = await readKnowledgeContent(knowledgeDir, fileName);
    }

    const hash = createHash('sha256')
      .update(
        stableStringify({
          user_id: profile.user_id,
          timezone: profile.timezone,
          locale: profile.locale,
          preferences: profile.preferences,
          knowledge: knowledgeByFile
        })
      )
      .digest('hex');

    return {
      ...profile,
      preferences: { ...profile.preferences },
      profile_hash: hash
    };
  }

  private async persistProfile(userId: string, profile: ProfileRecord): Promise<ProfileRecord> {
    const materialized = await this.materializeProfile(userId, profile);
    const { profileFile } = resolveProfilePath(this.rootDir, userId);
    await atomicWriteUtf8(profileFile, JSON.stringify(materialized, null, 2));
    return materialized;
  }

  private async withUserLock<T>(userId: string, operation: () => Promise<T>): Promise<T> {
    const previous = this.userLocks.get(userId) ?? Promise.resolve();
    let release!: () => void;
    const current = new Promise<void>((resolve) => {
      release = resolve;
    });
    const chain = previous.then(() => current, () => current);
    this.userLocks.set(userId, chain);

    await previous.catch(() => undefined);
    try {
      return await operation();
    } finally {
      release();
      if (this.userLocks.get(userId) === chain) {
        this.userLocks.delete(userId);
      }
    }
  }
}
