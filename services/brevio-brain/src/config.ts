import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  loadBrevioEnvironment,
  resolveAccessTokenVerificationKey,
  resolveCallerContextVerificationKey
} from '../../../packages/shared/src/security.js';

import type { BrainConfig, DisambiguationRuleConfig, DisambiguationRules, PlannerProvider } from './types.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const REQUIRED_GROUPS = [
  'apple-notes',
  'notion',
  'spotify',
  'flight-tracking',
  'healthkit',
  'apple-mail',
  'email-send',
  'expense-tracking',
  'package-tracking',
  'places-location',
  'youtube',
  'speech-transcription',
  'speech-synthesis',
  'speech-conversation',
  'image-perception',
  'document-perception',
  'video-perception',
  'camera-perception',
  'media-generation',
  'local-media'
] as const;

const REQUIRED_FIELDS: Record<(typeof REQUIRED_GROUPS)[number], string[]> = {
  'apple-notes': ['canonical'],
  notion: ['canonical', 'fallback'],
  spotify: ['cloud', 'local_mac', 'terminal', 'analytics'],
  'flight-tracking': ['track', 'find', 'free_tier'],
  healthkit: ['canonical'],
  'apple-mail': ['crud', 'search'],
  'email-send': ['by_preference'],
  'expense-tracking': ['canonical'],
  'package-tracking': ['international', 'carriers_17track', 'austrian_post'],
  'places-location': ['navigate', 'near_me', 'find_all', 'simple_nearby'],
  youtube: ['search', 'summarize', 'download'],
  'speech-transcription': ['canonical', 'transcribe'],
  'speech-synthesis': ['canonical', 'tts', 'local_mac'],
  'speech-conversation': ['canonical', 'realtime'],
  'image-perception': ['analyze', 'ocr'],
  'document-perception': ['canonical', 'extract'],
  'video-perception': ['canonical', 'frames'],
  'camera-perception': ['canonical', 'capture'],
  'media-generation': ['canonical', 'generate', 'caption'],
  'local-media': ['search', 'search_photos']
};

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

function parsePlannerProvider(raw: string | undefined): PlannerProvider {
  if (!raw || raw.trim() === '') {
    return 'deterministic';
  }
  const normalized = raw.trim().toLowerCase();
  if (normalized === 'deterministic' || normalized === 'openai_responses') {
    return normalized;
  }
  throw new Error('invalid BREVIO_BRAIN_PLANNER_PROVIDER: expected deterministic or openai_responses');
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

function stripQuotes(value: string): string {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function parseInlineArray(value: string): string[] {
  const inner = value.slice(1, -1).trim();
  if (inner === '') {
    return [];
  }
  return inner
    .split(',')
    .map((entry) => stripQuotes(entry))
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

function parseScalar(value: string): string | string[] {
  const trimmed = value.trim();
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    return parseInlineArray(trimmed);
  }
  return stripQuotes(trimmed);
}

function splitKeyValue(line: string): { key: string; value: string } {
  const idx = line.indexOf(':');
  if (idx <= 0) {
    throw new Error(`invalid config line: ${line}`);
  }
  return {
    key: line.slice(0, idx).trim(),
    value: line.slice(idx + 1).trim()
  };
}

function validateRules(rules: DisambiguationRules): DisambiguationRules {
  const groups = Object.keys(rules).sort();
  const required = [...REQUIRED_GROUPS].sort();
  if (groups.length !== required.length) {
    throw new Error(`disambiguation group count mismatch: expected ${required.length}, got ${groups.length}`);
  }
  for (const group of required) {
    const rule = rules[group];
    if (!rule) {
      throw new Error(`missing disambiguation group ${group}`);
    }
    for (const field of REQUIRED_FIELDS[group]) {
      const value = (rule as unknown as Record<string, unknown>)[field];
      if (!value || (Array.isArray(value) && value.length === 0)) {
        throw new Error(`disambiguation group ${group} missing required field ${field}`);
      }
    }
  }
  return rules;
}

function parseDisambiguation(raw: string): DisambiguationRules {
  const lines = raw.split('\n');
  const rules: DisambiguationRules = {};
  let currentRule: DisambiguationRuleConfig | undefined;
  let currentNestedKey: keyof DisambiguationRuleConfig | undefined;
  let versionSeen = false;

  for (const originalLine of lines) {
    const lineWithoutComment = originalLine.replace(/\s+#.*$/, '');
    if (lineWithoutComment.trim() === '') {
      continue;
    }

    const indent = lineWithoutComment.match(/^ */)?.[0].length ?? 0;
    const trimmed = lineWithoutComment.trim();

    if (indent === 0 && trimmed.startsWith('version:')) {
      const value = splitKeyValue(trimmed).value;
      if (value !== '1') {
        throw new Error(`unsupported disambiguation config version ${value}`);
      }
      versionSeen = true;
      continue;
    }

    if (indent === 0 && trimmed === 'rules:') {
      continue;
    }

    if (indent === 2 && trimmed.startsWith('- ')) {
      const entry = trimmed.slice(2).trim();
      const { key, value } = splitKeyValue(entry);
      if (key !== 'group') {
        throw new Error(`expected group declaration, got ${entry}`);
      }
      const group = stripQuotes(value);
      if (rules[group]) {
        throw new Error(`duplicate disambiguation group ${group}`);
      }
      currentRule = { group };
      rules[group] = currentRule;
      currentNestedKey = undefined;
      continue;
    }

    if (!currentRule) {
      throw new Error(`encountered config before any rule: ${trimmed}`);
    }

    if (indent === 4) {
      const { key, value } = splitKeyValue(trimmed);
      if (value === '') {
        if (key !== 'by_preference') {
          throw new Error(`unsupported nested config block ${key}`);
        }
        currentRule.by_preference = {};
        currentNestedKey = 'by_preference';
        continue;
      }
      currentNestedKey = undefined;
      (currentRule as unknown as Record<string, unknown>)[key] = parseScalar(value);
      continue;
    }

    if (indent === 6 && currentNestedKey === 'by_preference' && currentRule.by_preference) {
      const { key, value } = splitKeyValue(trimmed);
      currentRule.by_preference[key] = stripQuotes(value);
      continue;
    }

    throw new Error(`unsupported disambiguation config indentation: ${trimmed}`);
  }

  if (!versionSeen) {
    throw new Error('missing disambiguation config version');
  }

  return validateRules(rules);
}

export function loadBrainConfig(): BrainConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-brain',
    version: process.env.SERVICE_VERSION ?? process.env.npm_package_version ?? '0.3.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8081, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS'),
    disambiguationConfigPath: resolveDisambiguationPath(process.env.BREVIO_DISAMBIGUATION_CONFIG_PATH),
    plannerProvider: parsePlannerProvider(process.env.BREVIO_BRAIN_PLANNER_PROVIDER),
    plannerModel: process.env.BREVIO_BRAIN_PLANNER_MODEL ?? 'gpt-5.2',
    plannerFallbackModel: process.env.BREVIO_BRAIN_PLANNER_FALLBACK_MODEL ?? 'gpt-5-mini',
    plannerTimeoutMs: parsePositiveInt(process.env.BREVIO_BRAIN_PLANNER_TIMEOUT_MS, 30000, 'BREVIO_BRAIN_PLANNER_TIMEOUT_MS'),
    plannerBaseUrl: process.env.OPENAI_BASE_URL ?? 'https://api.openai.com/v1',
    temporalWorkerBaseUrl: process.env.BREVIO_TEMPORAL_WORKER_BASE_URL?.trim() || undefined,
    temporalWorkerTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_TIMEOUT_MS, 4000, 'BREVIO_TEMPORAL_WORKER_TIMEOUT_MS'),
    accessTokenIssuers: buildAccessTokenIssuerRegistry([
      {
        issuer: process.env.BREVIO_AUTH_ACCESS_ISSUER?.trim() || 'https://auth.brevio.internal',
        verificationKey: resolveAccessTokenVerificationKey(
          process.env.BREVIO_AUTH_ACCESS_PUBLIC_KEY,
          undefined,
          undefined,
          environment,
          'BREVIO_AUTH_ACCESS_PUBLIC_KEY',
          'auth-access'
        ),
        allowedTokenUses: ['user_access', 'admin_access']
      },
      {
        issuer: process.env.BREVIO_GATEWAY_SERVICE_ISSUER?.trim() || 'https://gateway.brevio.internal',
        verificationKey: resolveAccessTokenVerificationKey(
          process.env.BREVIO_GATEWAY_SERVICE_PUBLIC_KEY,
          undefined,
          undefined,
          environment,
          'BREVIO_GATEWAY_SERVICE_PUBLIC_KEY',
          'gateway-service'
        ),
        allowedTokenUses: ['service_access']
      }
    ]),
    serviceAudience: process.env.BREVIO_BRAIN_AUDIENCE?.trim() || 'brevio-brain',
    callerContextIssuers: buildCallerContextIssuerRegistry([
      {
        issuer: process.env.BREVIO_GATEWAY_CALLER_CONTEXT_ISSUER?.trim() || 'https://gateway.brevio.internal/caller-context',
        verificationKey: resolveCallerContextVerificationKey(
          process.env.BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY,
          environment,
          'BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY',
          'gateway-caller-context'
        )
      }
    ]),
    logSalt: process.env.BREVIO_BRAIN_LOG_SALT?.trim() || `brevio-brain:${environment}`
  };
}

export function loadDisambiguationRules(pathToConfig: string): DisambiguationRules {
  const raw = readFileSync(pathToConfig, 'utf8');
  return parseDisambiguation(raw);
}

export { REQUIRED_GROUPS };
