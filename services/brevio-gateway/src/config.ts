import type { GatewayConfig } from './types.js';

function parsePositiveInt(raw: string | undefined, fallback: number, field: string): number {
  if (!raw || raw.trim() === '') {
    return fallback;
  }
  const value = Number(raw);
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`invalid ${field}: expected positive integer`);
  }
  return value;
}

export function loadGatewayConfig(): GatewayConfig {
  return {
    serviceName: 'brevio-gateway',
    version: process.env.SERVICE_VERSION ?? '0.1.0',
    environment: process.env.NODE_ENV ?? 'development',
    port: parsePositiveInt(process.env.PORT, 8080, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(
      process.env.BREVIO_GATEWAY_SHUTDOWN_TIMEOUT_MS,
      30000,
      'BREVIO_GATEWAY_SHUTDOWN_TIMEOUT_MS'
    )
  };
}
