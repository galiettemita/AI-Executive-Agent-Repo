// AES-256-GCM helpers for token encryption at rest.
//
// On-disk layout: nonce(12) || ciphertext || tag(16). AAD = userId || provider || key_version.
// KEK from BREVIO_TOKEN_KEK env (32-byte base64url or hex:). Production refuses to boot
// if missing AND BREVIO_DEV_MODE !== 'true'. Memory wiping is best-effort.
//
// This is a "single-KEK" model. The migration 012 has key_version reserved on
// auth.oauth_tokens for a future rotation flow; this module accepts/emits it as AAD
// so a multi-KEK rotation can be wired later without schema changes.

import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';

export interface CryptoConfig {
  kek: Buffer | undefined;
  devMode: boolean;
}

export interface EncryptedField {
  ciphertext: Buffer;
  key_version: number;
}

const NONCE_BYTES = 12;
const TAG_BYTES = 16;
const KEY_BYTES = 32;
const KEY_VERSION = 1;

export function loadCryptoConfig(): CryptoConfig {
  const raw = process.env.BREVIO_TOKEN_KEK?.trim();
  const devMode = process.env.BREVIO_DEV_MODE === 'true';
  if (!raw) {
    if (devMode) {
      // Dev fallback: deterministic but distinct per process so dev tokens don't survive restart
      return { kek: randomBytes(KEY_BYTES), devMode: true };
    }
    throw new Error(
      'BREVIO_TOKEN_KEK required in production. Set BREVIO_DEV_MODE=true to use a per-process random KEK (dev only — encrypted tokens lost on restart).'
    );
  }
  const decoded = raw.startsWith('hex:')
    ? Buffer.from(raw.slice(4), 'hex')
    : Buffer.from(raw, 'base64');
  if (decoded.length !== KEY_BYTES) {
    throw new Error(`BREVIO_TOKEN_KEK must decode to exactly ${KEY_BYTES} bytes; got ${decoded.length}`);
  }
  return { kek: decoded, devMode };
}

function buildAAD(userId: string, provider: string, keyVersion: number): Buffer {
  return Buffer.from(`${userId}|${provider}|${keyVersion}`, 'utf8');
}

export function encryptToken(
  config: CryptoConfig,
  plaintext: string,
  userId: string,
  provider: string
): EncryptedField {
  if (!config.kek) {
    throw new Error('crypto: KEK not loaded');
  }
  const nonce = randomBytes(NONCE_BYTES);
  const aad = buildAAD(userId, provider, KEY_VERSION);
  const cipher = createCipheriv('aes-256-gcm', config.kek, nonce);
  cipher.setAAD(aad);
  const enc = Buffer.concat([cipher.update(Buffer.from(plaintext, 'utf8')), cipher.final()]);
  const tag = cipher.getAuthTag();
  return {
    ciphertext: Buffer.concat([nonce, enc, tag]),
    key_version: KEY_VERSION
  };
}

export function decryptToken(
  config: CryptoConfig,
  ciphertext: Buffer,
  keyVersion: number,
  userId: string,
  provider: string
): string {
  if (!config.kek) {
    throw new Error('crypto: KEK not loaded');
  }
  if (ciphertext.length < NONCE_BYTES + TAG_BYTES) {
    throw new Error('crypto: ciphertext too short');
  }
  const nonce = ciphertext.subarray(0, NONCE_BYTES);
  const tag = ciphertext.subarray(ciphertext.length - TAG_BYTES);
  const enc = ciphertext.subarray(NONCE_BYTES, ciphertext.length - TAG_BYTES);
  const aad = buildAAD(userId, provider, keyVersion);
  const decipher = createDecipheriv('aes-256-gcm', config.kek, nonce);
  decipher.setAAD(aad);
  decipher.setAuthTag(tag);
  const plain = Buffer.concat([decipher.update(enc), decipher.final()]);
  return plain.toString('utf8');
}

export const CRYPTO_KEY_VERSION = KEY_VERSION;
