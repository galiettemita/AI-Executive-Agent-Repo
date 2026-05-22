import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import { loadBrainConfig } from './config.js';
import type { BrainConfig, RequestContext } from './types.js';

interface BrainRuntime {
  config: BrainConfig;
  server: http.Server;
  startedAtMs: number;
  close(): Promise<void>;
}

function getHeader(req: http.IncomingMessage, name: string): string | undefined {
  const value = req.headers[name.toLowerCase()];
  if (typeof value === 'string') {
    return value;
  }
  if (Array.isArray(value) && value.length > 0) {
    return value[0];
  }
  return undefined;
}

function requestContext(req: http.IncomingMessage): RequestContext {
  return {
    traceId: getHeader(req, 'x-trace-id') ?? randomUUID(),
    spanId: getHeader(req, 'x-span-id') ?? randomUUID(),
    requestId: getHeader(req, 'x-request-id') ?? randomUUID(),
    userId: getHeader(req, 'x-user-id')
  };
}

function logEvent(
  config: BrainConfig,
  ctx: RequestContext | undefined,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: config.serviceName,
      env: config.environment,
      trace_id: ctx?.traceId,
      span_id: ctx?.spanId,
      request_id: ctx?.requestId,
      user_id: ctx?.userId,
      event,
      severity,
      attrs
    }) + '\n'
  );
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

export function createBrainRuntime(config: BrainConfig = loadBrainConfig()): BrainRuntime {
  const startedAtMs = Date.now();

  const server = http.createServer((req, res) => {
    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const path = (req.url ?? '/').split('?')[0] ?? '/';

    if (method === 'GET' && path === '/health') {
      sendJSON(res, 200, {
        status: 'ok',
        service: config.serviceName,
        version: config.version,
        uptime_ms: Date.now() - startedAtMs
      });
      return;
    }

    sendJSON(res, 404, { error: 'not_found', request_id: ctx.requestId });
  });

  const runtime: BrainRuntime = {
    config,
    server,
    startedAtMs,
    close: async () => {
      await new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
      });
    }
  };

  server.on('listening', () => {
    logEvent(config, undefined, 'brain.server.listening', 'INFO', { port: config.port });
  });

  return runtime;
}

export async function main(): Promise<void> {
  const runtime = createBrainRuntime();
  runtime.server.listen(runtime.config.port);

  const shutdown = async (signal: string): Promise<void> => {
    logEvent(runtime.config, undefined, 'brain.server.shutting_down', 'INFO', { signal });
    try {
      await runtime.close();
    } catch (err) {
      logEvent(runtime.config, undefined, 'brain.server.shutdown_error', 'ERROR', {
        error: err instanceof Error ? err.message : String(err)
      });
      process.exitCode = 1;
    }
  };

  process.once('SIGTERM', () => {
    void shutdown('SIGTERM');
  });
  process.once('SIGINT', () => {
    void shutdown('SIGINT');
  });
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  void main();
}
