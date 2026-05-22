import type { BrainConfig } from './types.js';

function parsePositiveInt(raw: string | undefined, fallback: number, field: string): number {
  if (!raw || raw.trim() === '') {
    return fallback;
  }
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid ${field}: expected positive integer`);
  }
  return parsed;
}

export function loadBrainConfig(): BrainConfig {
  return {
    serviceName: 'brevio-brain',
    version: process.env.SERVICE_VERSION ?? process.env.npm_package_version ?? '0.1.0',
    environment: process.env.NODE_ENV ?? 'development',
    port: parsePositiveInt(process.env.PORT, 8081, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(
      process.env.BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS,
      30000,
      'BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS'
    )
  };
}
