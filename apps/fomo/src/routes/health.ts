import type http from 'node:http';

import type { FomoConfig } from '../types.js';

export interface HealthResponse {
  status: 'ok';
  service: string;
  version: string;
  uptime_ms: number;
}

export function buildHealthResponse(config: FomoConfig, startedAtMs: number, now: number = Date.now()): HealthResponse {
  return {
    status: 'ok',
    service: config.serviceName,
    version: config.version,
    uptime_ms: now - startedAtMs
  };
}

export function handleHealth(
  res: http.ServerResponse,
  config: FomoConfig,
  startedAtMs: number
): void {
  const payload = buildHealthResponse(config, startedAtMs);
  res.writeHead(200, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}
