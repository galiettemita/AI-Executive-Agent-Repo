import { createHmac, timingSafeEqual } from 'node:crypto';

function normalizeSignature(signatureHeader: string): string {
  const trimmed = signatureHeader.trim();
  if (trimmed.startsWith('sha256=')) {
    return trimmed.slice('sha256='.length);
  }
  return trimmed;
}

export function verifyWhatsAppSignature(rawBody: Buffer, signatureHeader: string | undefined, sharedSecret: string): boolean {
  if (!signatureHeader || sharedSecret.trim() === '') {
    return false;
  }

  const expected = createHmac('sha256', sharedSecret).update(rawBody).digest('hex');
  const actual = normalizeSignature(signatureHeader);
  if (expected.length !== actual.length) {
    return false;
  }

  return timingSafeEqual(Buffer.from(expected, 'utf8'), Buffer.from(actual, 'utf8'));
}

export function verifyAPIKey(provided: string | undefined, expected: string, environment: string): boolean {
  if (expected.trim() === '') {
    return environment !== 'production';
  }
  if (!provided) {
    return false;
  }
  if (provided.length !== expected.length) {
    return false;
  }
  return timingSafeEqual(Buffer.from(provided, 'utf8'), Buffer.from(expected, 'utf8'));
}
