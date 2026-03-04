import { randomUUID } from 'node:crypto';
import type { IncomingHttpHeaders } from 'node:http';

import type { RequestContext } from './types.js';

function getHeader(headers: IncomingHttpHeaders, key: string): string | undefined {
  const value = headers[key.toLowerCase()];
  if (typeof value === 'string') {
    return value;
  }
  if (Array.isArray(value) && value.length > 0) {
    return value[0];
  }
  return undefined;
}

export function requestContextFromHeaders(headers: IncomingHttpHeaders): RequestContext {
  const traceId = getHeader(headers, 'x-trace-id') ?? randomUUID();
  const spanId = getHeader(headers, 'x-span-id') ?? randomUUID();
  const userId = getHeader(headers, 'x-user-id');
  const correlationId =
    getHeader(headers, 'x-correlation-id') ?? getHeader(headers, 'x-request-id') ?? randomUUID();

  return {
    traceId,
    spanId,
    userId,
    correlationId
  };
}

export function logJSON(
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  service: string,
  env: string,
  context: RequestContext,
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service,
      env,
      trace_id: context.traceId,
      span_id: context.spanId,
      user_id: context.userId,
      correlation_id: context.correlationId,
      event,
      severity,
      attrs
    }) + '\n'
  );
}
