// Postgres-backed TokenStore. Same contract as InMemoryTokenStore from
// Phase 2A. Tokens are encrypted at-rest with the same KEK / AAD scheme;
// ciphertext is stored as base64 text in the access_token_ciphertext /
// refresh_token_ciphertext columns. Phase 2 chose text + base64 over bytea
// for migration simplicity and easier debugging; a future phase can switch
// to bytea via a Drizzle customType without changing the wire contract.

import { and, eq } from 'drizzle-orm';

import { type CryptoConfig, decryptToken, encryptToken } from '../../security/token-crypto.js';
import {
  type StoredTokenInput,
  type StoredTokenView,
  type TokenStore
} from '../../security/oauth/token-store.js';
import { type DrizzleClient } from '../client.js';
import { oauth_tokens } from '../schema.js';

function bufToB64(buf: Buffer): string {
  return buf.toString('base64');
}

function b64ToBuf(text: string): Buffer {
  return Buffer.from(text, 'base64');
}

export class PostgresTokenStore implements TokenStore {
  private readonly db: DrizzleClient;
  private readonly crypto: CryptoConfig;

  constructor(db: DrizzleClient, crypto: CryptoConfig) {
    this.db = db;
    this.crypto = crypto;
  }

  async save(input: StoredTokenInput): Promise<void> {
    const access = encryptToken(this.crypto, input.access_token, input.user_id, input.provider);
    const refresh = input.refresh_token
      ? encryptToken(this.crypto, input.refresh_token, input.user_id, input.provider)
      : null;

    const values: typeof oauth_tokens.$inferInsert = {
      user_id: input.user_id,
      provider: input.provider,
      scopes: [...input.scopes],
      access_token_ciphertext: bufToB64(access.ciphertext),
      refresh_token_ciphertext: refresh ? bufToB64(refresh.ciphertext) : null,
      expires_at: input.expires_at ?? null,
      key_version: access.key_version,
      needs_reauth: false
    };

    await this.db
      .insert(oauth_tokens)
      .values(values)
      .onConflictDoUpdate({
        target: [oauth_tokens.user_id, oauth_tokens.provider],
        set: {
          scopes: values.scopes,
          access_token_ciphertext: values.access_token_ciphertext,
          refresh_token_ciphertext: values.refresh_token_ciphertext,
          expires_at: values.expires_at,
          obtained_at: new Date(),
          last_refreshed_at: null,
          needs_reauth: false,
          key_version: values.key_version
        }
      });
  }

  async loadAccessToken(userId: string, provider: string): Promise<string | null> {
    const rows = await this.db
      .select({
        access_token_ciphertext: oauth_tokens.access_token_ciphertext,
        key_version: oauth_tokens.key_version
      })
      .from(oauth_tokens)
      .where(and(eq(oauth_tokens.user_id, userId), eq(oauth_tokens.provider, provider)))
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return decryptToken(this.crypto, b64ToBuf(r.access_token_ciphertext), r.key_version, userId, provider);
  }

  async loadRefreshToken(userId: string, provider: string): Promise<string | null> {
    const rows = await this.db
      .select({
        refresh_token_ciphertext: oauth_tokens.refresh_token_ciphertext,
        key_version: oauth_tokens.key_version
      })
      .from(oauth_tokens)
      .where(and(eq(oauth_tokens.user_id, userId), eq(oauth_tokens.provider, provider)))
      .limit(1);
    const r = rows[0];
    if (!r?.refresh_token_ciphertext) return null;
    return decryptToken(this.crypto, b64ToBuf(r.refresh_token_ciphertext), r.key_version, userId, provider);
  }

  async delete(userId: string, provider: string): Promise<void> {
    await this.db
      .delete(oauth_tokens)
      .where(and(eq(oauth_tokens.user_id, userId), eq(oauth_tokens.provider, provider)));
  }

  async list(userId: string): Promise<StoredTokenView[]> {
    const rows = await this.db
      .select({
        provider: oauth_tokens.provider,
        scopes: oauth_tokens.scopes,
        expires_at: oauth_tokens.expires_at,
        obtained_at: oauth_tokens.obtained_at,
        needs_reauth: oauth_tokens.needs_reauth,
        key_version: oauth_tokens.key_version
      })
      .from(oauth_tokens)
      .where(eq(oauth_tokens.user_id, userId));
    return rows.map((r) => ({
      provider: r.provider,
      scopes: r.scopes,
      expires_at: r.expires_at,
      obtained_at: r.obtained_at,
      needs_reauth: r.needs_reauth,
      key_version: r.key_version
    }));
  }

  async has(userId: string, provider: string): Promise<boolean> {
    const rows = await this.db
      .select({ provider: oauth_tokens.provider })
      .from(oauth_tokens)
      .where(and(eq(oauth_tokens.user_id, userId), eq(oauth_tokens.provider, provider)))
      .limit(1);
    return rows.length > 0;
  }

  async markNeedsReauth(userId: string, provider: string): Promise<void> {
    await this.db
      .update(oauth_tokens)
      .set({ needs_reauth: true })
      .where(and(eq(oauth_tokens.user_id, userId), eq(oauth_tokens.provider, provider)));
  }
}
