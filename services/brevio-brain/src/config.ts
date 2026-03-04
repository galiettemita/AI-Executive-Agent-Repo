import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import type { BrainConfig, DisambiguationRule } from './types.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

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

function resolveDisambiguationPath(inputPath: string | undefined): string {
  if (inputPath && inputPath.trim() !== '') {
    return inputPath;
  }

  const candidates = [
    path.resolve(process.cwd(), 'config', 'skill-disambiguation.yaml'),
    path.resolve(process.cwd(), '..', '..', 'config', 'skill-disambiguation.yaml'),
    path.resolve(__dirname, '..', '..', '..', 'config', 'skill-disambiguation.yaml')
  ];

  for (const candidate of candidates) {
    try {
      readFileSync(candidate);
      return candidate;
    } catch {
      // keep scanning
    }
  }

  throw new Error('unable to resolve skill-disambiguation.yaml path; set BREVIO_DISAMBIGUATION_CONFIG_PATH');
}

function parseValue(raw: string): string | string[] {
  const trimmed = raw.trim();
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    const inner = trimmed.slice(1, -1).trim();
    if (inner === '') {
      return [];
    }
    return inner
      .split(',')
      .map((entry) => entry.trim())
      .map((entry) => {
        if ((entry.startsWith('"') && entry.endsWith('"')) || (entry.startsWith("'") && entry.endsWith("'"))) {
          return entry.slice(1, -1);
        }
        return entry;
      })
      .filter((entry) => entry.length > 0);
  }

  if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1);
  }

  return trimmed;
}

function parseDisambiguation(raw: string): DisambiguationRule[] {
  const lines = raw.split('\n');
  const rules: DisambiguationRule[] = [];
  let current: DisambiguationRule | null = null;

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed === '' || trimmed.startsWith('#') || trimmed === 'rules:' || trimmed.startsWith('version:')) {
      continue;
    }

    if (trimmed.startsWith('- group:')) {
      if (current) {
        rules.push(current);
      }
      const group = trimmed.slice('- group:'.length).trim();
      current = {
        group,
        values: {}
      };
      continue;
    }

    if (!current) {
      continue;
    }

    const idx = trimmed.indexOf(':');
    if (idx <= 0) {
      continue;
    }
    const key = trimmed.slice(0, idx).trim();
    const valueRaw = trimmed.slice(idx + 1).trim();
    current.values[key] = parseValue(valueRaw);
  }

  if (current) {
    rules.push(current);
  }

  return rules;
}

export function loadBrainConfig(): BrainConfig {
  return {
    serviceName: 'brevio-brain',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment: process.env.NODE_ENV ?? 'development',
    port: parsePositiveInt(process.env.PORT, 8081, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS'),
    disambiguationConfigPath: resolveDisambiguationPath(process.env.BREVIO_DISAMBIGUATION_CONFIG_PATH)
  };
}

export function loadDisambiguationRules(pathToConfig: string): DisambiguationRule[] {
  const raw = readFileSync(pathToConfig, 'utf8');
  const rules = parseDisambiguation(raw);
  if (rules.length < 11) {
    throw new Error(`expected at least 11 disambiguation rules, got ${rules.length}`);
  }
  return rules;
}
