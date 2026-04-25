import { createCipheriv, createDecipheriv, createHash, randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

import type { OAuthStateRecord } from './types.js';

interface OAuthStateSnapshot {
  version: 1;
  records: Array<[string, OAuthStateRecord]>;
}

interface EncryptedOAuthStateSnapshot {
  version: 2;
  alg: 'aes-256-gcm';
  nonce: string;
  ciphertext: string;
  tag: string;
}

export class OAuthStateStore {
  private readonly records: Map<string, OAuthStateRecord>;
  private readonly filePath?: string;
  private readonly encryptionKey: Buffer;

  constructor(filePath?: string, encryptionSecret?: string) {
    this.filePath = filePath;
    this.encryptionKey = createHash('sha256').update(encryptionSecret ?? 'brevio-auth-oauth-state').digest();
    this.records = this.loadSnapshot();
  }

  mode(): 'in_memory' | 'local_file_snapshot' {
    return this.filePath ? 'local_file_snapshot' : 'in_memory';
  }

  snapshotPath(): string | undefined {
    return this.filePath;
  }

  size(): number {
    return this.records.size;
  }

  get(state: string): OAuthStateRecord | null {
    const record = this.records.get(state);
    return record ? { ...record } : null;
  }

  put(state: string, record: OAuthStateRecord): void {
    this.records.set(state, { ...record });
    this.persist();
  }

  consume(service: string, state: string, nowMs: number): OAuthStateRecord | null {
    const record = this.records.get(state);
    if (!record) {
      return null;
    }
    if (record.service !== service) {
      return null;
    }
    if (record.expiresAtMs <= nowMs) {
      this.records.delete(state);
      this.persist();
      return null;
    }
    this.records.delete(state);
    this.persist();
    return { ...record };
  }

  expire(nowMs: number): void {
    let changed = false;
    for (const [state, record] of this.records.entries()) {
      if (record.expiresAtMs <= nowMs) {
        this.records.delete(state);
        changed = true;
      }
    }
    if (changed) {
      this.persist();
    }
  }

  private loadSnapshot(): Map<string, OAuthStateRecord> {
    if (!this.filePath || !existsSync(this.filePath)) {
      return new Map();
    }

    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<OAuthStateSnapshot & EncryptedOAuthStateSnapshot>;
      if (!parsed || typeof parsed !== 'object') {
        throw new Error('snapshot must be a JSON object');
      }
      const records = this.recordsFromSnapshot(parsed);
      const out = new Map<string, OAuthStateRecord>();
      for (const entry of records) {
        if (!Array.isArray(entry) || entry.length !== 2 || typeof entry[0] !== 'string' || !entry[0].trim()) {
          throw new Error('snapshot record key is invalid');
        }
        const record = entry[1];
        if (
          !record ||
          typeof record !== 'object' ||
          typeof record.service !== 'string' ||
          !record.service.trim() ||
          typeof record.userId !== 'string' ||
          !record.userId.trim() ||
          (record.completionRedirectUri !== undefined &&
            (typeof record.completionRedirectUri !== 'string' || !record.completionRedirectUri.trim())) ||
          typeof record.codeVerifier !== 'string' ||
          !record.codeVerifier.trim() ||
          typeof record.createdAtMs !== 'number' ||
          !Number.isFinite(record.createdAtMs) ||
          typeof record.expiresAtMs !== 'number' ||
          !Number.isFinite(record.expiresAtMs)
        ) {
          throw new Error('snapshot oauth state record is invalid');
        }
        out.set(entry[0], { ...record });
      }
      return out;
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`oauth state snapshot is corrupt at ${this.filePath}: ${detail}`);
    }
  }

  private recordsFromSnapshot(
    parsed: Partial<OAuthStateSnapshot & EncryptedOAuthStateSnapshot>
  ): Array<[string, OAuthStateRecord]> {
    if (parsed.version === 2) {
      if (parsed.alg !== 'aes-256-gcm' || typeof parsed.nonce !== 'string' || typeof parsed.ciphertext !== 'string' || typeof parsed.tag !== 'string') {
        throw new Error('encrypted snapshot is invalid');
      }
      const decipher = createDecipheriv(
        'aes-256-gcm',
        this.encryptionKey,
        Buffer.from(parsed.nonce, 'base64url')
      );
      decipher.setAuthTag(Buffer.from(parsed.tag, 'base64url'));
      const decrypted = Buffer.concat([
        decipher.update(Buffer.from(parsed.ciphertext, 'base64url')),
        decipher.final()
      ]).toString('utf8');
      const snapshot = JSON.parse(decrypted) as OAuthStateSnapshot;
      if (!Array.isArray(snapshot.records)) {
        throw new Error('encrypted snapshot records must be an array');
      }
      return snapshot.records;
    }

    if (parsed.version !== undefined && parsed.version !== 1) {
      throw new Error(`unsupported snapshot version: ${String(parsed.version)}`);
    }
    if ('records' in parsed && !Array.isArray(parsed.records)) {
      throw new Error('snapshot records must be an array');
    }
    return parsed.records ?? [];
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }

    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.${process.pid}.tmp`;
    const snapshot: OAuthStateSnapshot = {
      version: 1,
      records: Array.from(this.records.entries())
    };
    const nonce = randomBytes(12);
    const cipher = createCipheriv('aes-256-gcm', this.encryptionKey, nonce);
    const ciphertext = Buffer.concat([
      cipher.update(JSON.stringify(snapshot), 'utf8'),
      cipher.final()
    ]);
    const encrypted: EncryptedOAuthStateSnapshot = {
      version: 2,
      alg: 'aes-256-gcm',
      nonce: nonce.toString('base64url'),
      ciphertext: ciphertext.toString('base64url'),
      tag: cipher.getAuthTag().toString('base64url')
    };
    writeFileSync(tmpPath, JSON.stringify(encrypted, null, 2), 'utf8');
    renameSync(tmpPath, this.filePath);
  }
}
