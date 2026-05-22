import { type CryptoConfig, decryptToken, encryptToken } from './crypto.js';

export interface StoredTokenInput {
  user_id: string;
  provider: string;
  scopes: string[];
  access_token: string;
  refresh_token?: string;
  expires_at?: Date;
}

export interface StoredTokenView {
  provider: string;
  scopes: string[];
  expires_at: Date | null;
  obtained_at: Date;
  needs_reauth: boolean;
  key_version: number;
}

export interface TokenStore {
  save(input: StoredTokenInput): Promise<void>;
  loadAccessToken(userId: string, provider: string): Promise<string | null>;
  loadRefreshToken(userId: string, provider: string): Promise<string | null>;
  delete(userId: string, provider: string): Promise<void>;
  list(userId: string): Promise<StoredTokenView[]>;
  has(userId: string, provider: string): Promise<boolean>;
  markNeedsReauth(userId: string, provider: string): Promise<void>;
}

interface InternalRow {
  user_id: string;
  provider: string;
  scopes: string[];
  access_token_ciphertext: Buffer;
  refresh_token_ciphertext: Buffer | null;
  expires_at: Date | null;
  obtained_at: Date;
  last_refreshed_at: Date | null;
  needs_reauth: boolean;
  key_version: number;
}

export class InMemoryTokenStore implements TokenStore {
  private readonly rows = new Map<string, InternalRow>();
  private readonly crypto: CryptoConfig;

  constructor(crypto: CryptoConfig) {
    this.crypto = crypto;
  }

  private key(userId: string, provider: string): string {
    return `${userId}::${provider}`;
  }

  async save(input: StoredTokenInput): Promise<void> {
    const access = encryptToken(this.crypto, input.access_token, input.user_id, input.provider);
    const refresh = input.refresh_token
      ? encryptToken(this.crypto, input.refresh_token, input.user_id, input.provider)
      : null;

    this.rows.set(this.key(input.user_id, input.provider), {
      user_id: input.user_id,
      provider: input.provider,
      scopes: [...input.scopes],
      access_token_ciphertext: access.ciphertext,
      refresh_token_ciphertext: refresh?.ciphertext ?? null,
      expires_at: input.expires_at ?? null,
      obtained_at: new Date(),
      last_refreshed_at: null,
      needs_reauth: false,
      key_version: access.key_version
    });
  }

  async loadAccessToken(userId: string, provider: string): Promise<string | null> {
    const row = this.rows.get(this.key(userId, provider));
    if (!row) return null;
    return decryptToken(this.crypto, row.access_token_ciphertext, row.key_version, userId, provider);
  }

  async loadRefreshToken(userId: string, provider: string): Promise<string | null> {
    const row = this.rows.get(this.key(userId, provider));
    if (!row?.refresh_token_ciphertext) return null;
    return decryptToken(this.crypto, row.refresh_token_ciphertext, row.key_version, userId, provider);
  }

  async delete(userId: string, provider: string): Promise<void> {
    this.rows.delete(this.key(userId, provider));
  }

  async list(userId: string): Promise<StoredTokenView[]> {
    const out: StoredTokenView[] = [];
    for (const row of this.rows.values()) {
      if (row.user_id === userId) {
        out.push({
          provider: row.provider,
          scopes: row.scopes,
          expires_at: row.expires_at,
          obtained_at: row.obtained_at,
          needs_reauth: row.needs_reauth,
          key_version: row.key_version
        });
      }
    }
    return out;
  }

  async has(userId: string, provider: string): Promise<boolean> {
    return this.rows.has(this.key(userId, provider));
  }

  async markNeedsReauth(userId: string, provider: string): Promise<void> {
    const row = this.rows.get(this.key(userId, provider));
    if (row) row.needs_reauth = true;
  }
}
