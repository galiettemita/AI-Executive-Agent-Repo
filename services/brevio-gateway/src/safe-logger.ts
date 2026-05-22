// Sensitive-field redaction wrapper. Use this instead of process.stdout.write for any
// log line that includes user-supplied data, request bodies, or token-bearing fields.
//
// Forbidden in any log line:
//   - access tokens, refresh tokens, KEK, session signing key, OAuth client secrets
//   - the raw OAuth `code` parameter
//   - request bodies for /oauth/callback or /me/consent without redaction
//   - user PII beyond user_id

const REDACTED_KEYS = new Set([
  'access_token',
  'refresh_token',
  'access_token_ciphertext',
  'refresh_token_ciphertext',
  'token',
  'authorization',
  'cookie',
  'set-cookie',
  'kek',
  'session_signing_key',
  'oauth_client_secret',
  'client_secret',
  'code',
  'code_verifier',
  'state',
  'dek_encrypted',
  'access_token_encrypted',
  'refresh_token_encrypted',
  'password'
]);

const TOKEN_SHAPED = /\b(eyJ[A-Za-z0-9_-]{20,}|ya29\.[A-Za-z0-9_-]{20,}|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{30,}|sk-[A-Za-z0-9_-]{20,})\b/g;

function redactValue(value: unknown, depth = 0): unknown {
  if (depth > 8) return '<truncated>';
  if (value === null || value === undefined) return value;
  if (typeof value === 'string') {
    return value.replace(TOKEN_SHAPED, '<redacted-token>');
  }
  if (Array.isArray(value)) {
    return value.map((item) => redactValue(item, depth + 1));
  }
  if (typeof value === 'object') {
    const obj = value as Record<string, unknown>;
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(obj)) {
      if (REDACTED_KEYS.has(k.toLowerCase())) {
        out[k] = '<redacted>';
        continue;
      }
      out[k] = redactValue(v, depth + 1);
    }
    return out;
  }
  return value;
}

export function redact(payload: unknown): unknown {
  return redactValue(payload);
}

export interface SafeLogEvent {
  service: string;
  environment: string;
  trace_id?: string;
  span_id?: string;
  request_id?: string;
  user_id?: string;
  event: string;
  severity: 'INFO' | 'WARN' | 'ERROR';
  attrs?: Record<string, unknown>;
}

export function safeLog(event: SafeLogEvent, write: (line: string) => void = (line) => process.stdout.write(line)): void {
  const line = JSON.stringify({
    ts: new Date().toISOString(),
    service: event.service,
    env: event.environment,
    trace_id: event.trace_id,
    span_id: event.span_id,
    request_id: event.request_id,
    user_id: event.user_id,
    event: event.event,
    severity: event.severity,
    attrs: event.attrs ? redact(event.attrs) : undefined
  }) + '\n';
  write(line);
}

export function containsTokenShape(text: string): boolean {
  TOKEN_SHAPED.lastIndex = 0;
  return TOKEN_SHAPED.test(text);
}
