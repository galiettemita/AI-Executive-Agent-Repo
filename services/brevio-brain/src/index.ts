import { randomUUID } from 'node:crypto';
import http from 'node:http';

import { aggregateResults } from './aggregate.js';
import { classifyIntent } from './classify.js';
import { loadBrainConfig, loadDisambiguationRules } from './config.js';
import { decomposeTask } from './decompose.js';
import { disambiguateSkills } from './disambiguate.js';
import type {
  AggregationRequest,
  BrainConfig,
  DisambiguationRequest,
  IntentClassificationInput,
  RequestContext,
  SkillResult
} from './types.js';

interface BrainRuntime {
  config: BrainConfig;
  startedAtMs: number;
  disambiguationRules: ReturnType<typeof loadDisambiguationRules>;
  server: http.Server;
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
  runtime: BrainRuntime,
  ctx: RequestContext,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: runtime.config.serviceName,
      env: runtime.config.environment,
      trace_id: ctx.traceId,
      span_id: ctx.spanId,
      request_id: ctx.requestId,
      user_id: ctx.userId,
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

async function readRawBody(req: http.IncomingMessage, maxBytes = 2 * 1024 * 1024): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let bytes = 0;
  for await (const chunk of req) {
    const data = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    bytes += data.byteLength;
    if (bytes > maxBytes) {
      throw new Error('payload_too_large');
    }
    chunks.push(data);
  }
  return chunks.length > 0 ? Buffer.concat(chunks) : Buffer.from('{}', 'utf8');
}

function parseObject(rawBody: Buffer): Record<string, unknown> {
  try {
    const parsed = JSON.parse(rawBody.toString('utf8'));
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return {};
  } catch {
    return {};
  }
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const output = value
    .map((item) => asString(item))
    .filter((item): item is string => Boolean(item));
  return output.length > 0 ? output : undefined;
}

function asBool(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') {
    return value;
  }
  return undefined;
}

function extractSkillResults(value: unknown): SkillResult[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const out: SkillResult[] = [];
  for (const item of value) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      continue;
    }
    const raw = item as Record<string, unknown>;
    const skillId = asString(raw.skill_id);
    const status = asString(raw.status);
    if (!skillId || !status) {
      continue;
    }
    if (status !== 'SUCCESS' && status !== 'PARTIAL' && status !== 'FAILED' && status !== 'TIMEOUT') {
      continue;
    }

    const result: SkillResult = {
      skill_id: skillId,
      status,
      data: raw.data && typeof raw.data === 'object' && !Array.isArray(raw.data)
        ? (raw.data as Record<string, unknown>)
        : undefined,
      error: raw.error && typeof raw.error === 'object' && !Array.isArray(raw.error)
        ? {
            code: asString((raw.error as Record<string, unknown>).code) ?? 'UNKNOWN_ERROR',
            message: asString((raw.error as Record<string, unknown>).message) ?? 'unknown error'
          }
        : undefined
    };

    out.push(result);
  }

  return out;
}

function healthPayload(runtime: BrainRuntime, deep: boolean): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    status: 'healthy',
    version: runtime.config.version,
    uptime_ms: Date.now() - runtime.startedAtMs
  };

  if (!deep) {
    return payload;
  }

  return {
    ...payload,
    checks: {
      process: 'ok',
      disambiguation_config: runtime.config.disambiguationConfigPath
    },
    disambiguation: {
      rules_loaded: runtime.disambiguationRules.length,
      groups: runtime.disambiguationRules.map((rule) => rule.group)
    }
  };
}

function buildRuntime(config?: BrainConfig): BrainRuntime {
  const resolvedConfig = config ?? loadBrainConfig();
  const disambiguationRules = loadDisambiguationRules(resolvedConfig.disambiguationConfigPath);
  const startedAtMs = Date.now();

  let runtimeRef: BrainRuntime | undefined;

  const server = http.createServer((req, res) => {
    const runtime = runtimeRef;
    if (!runtime) {
      sendJSON(res, 500, { error: 'runtime_not_ready' });
      return;
    }

    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const path = new URL(req.url ?? '/', 'http://localhost').pathname;

    const onError = (statusCode: number, code: string): void => {
      sendJSON(res, statusCode, { error: code });
      logEvent(runtime, ctx, 'brain.request.error', 'WARN', {
        method,
        path,
        status_code: statusCode,
        code
      });
    };

    if (method === 'GET' && path === '/health') {
      sendJSON(res, 200, healthPayload(runtime, false));
      return;
    }

    if (method === 'GET' && path === '/health/deep') {
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/classify') {
      void (async () => {
        const rawBody = await readRawBody(req);
        const payload = parseObject(rawBody);
        const messageText = asString(payload.message_text);
        if (!messageText) {
          onError(400, 'message_text_required');
          return;
        }

        const output = classifyIntent(payload as IntentClassificationInput);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.classify.complete', 'INFO', {
          intent: output.intent,
          confidence: output.confidence,
          skills: output.skills
        });
      })().catch((err) => {
        onError(500, 'classify_failed');
        logEvent(runtime, ctx, 'brain.classify.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/disambiguate') {
      void (async () => {
        const rawBody = await readRawBody(req);
        const payload = parseObject(rawBody);
        const messageText = asString(payload.message_text);
        if (!messageText) {
          onError(400, 'message_text_required');
          return;
        }

        const disambiguationReq: DisambiguationRequest = {
          message_text: messageText,
          intent: asString(payload.intent),
          candidate_skills: asStringArray(payload.candidate_skills),
          deployment_mode: asString(payload.deployment_mode) as DisambiguationRequest['deployment_mode'],
          user_tier: asString(payload.user_tier) as DisambiguationRequest['user_tier'],
          user_preferences:
            payload.user_preferences && typeof payload.user_preferences === 'object' && !Array.isArray(payload.user_preferences)
              ? (payload.user_preferences as DisambiguationRequest['user_preferences'])
              : undefined
        };

        const output = disambiguateSkills(disambiguationReq, runtime.disambiguationRules);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.disambiguate.complete', 'INFO', {
          group_hits: output.group_hits,
          resolved_skills: output.resolved_skills
        });
      })().catch((err) => {
        onError(500, 'disambiguation_failed');
        logEvent(runtime, ctx, 'brain.disambiguate.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/decompose') {
      void (async () => {
        const rawBody = await readRawBody(req);
        const payload = parseObject(rawBody);
        const requestText = asString(payload.request) ?? asString(payload.message_text) ?? '';
        const skills = asStringArray(payload.skills) ?? [];
        const requires = asBool(payload.requires_decomposition) ?? skills.length > 1;

        const output = decomposeTask(requestText, skills, requires);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.decompose.complete', 'INFO', {
          tasks: output.tasks.length,
          execution_order: output.execution_order
        });
      })().catch((err) => {
        if (err instanceof Error && err.message.startsWith('TASK_GRAPH_INVALID')) {
          sendJSON(res, 422, {
            error: 'TASK_GRAPH_INVALID',
            message: err.message
          });
          return;
        }
        onError(500, 'decompose_failed');
        logEvent(runtime, ctx, 'brain.decompose.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/aggregate') {
      void (async () => {
        const rawBody = await readRawBody(req);
        const payload = parseObject(rawBody);
        const request: AggregationRequest = {
          skill_results: extractSkillResults(payload.skill_results),
          user_profile:
            payload.user_profile && typeof payload.user_profile === 'object' && !Array.isArray(payload.user_profile)
              ? (payload.user_profile as AggregationRequest['user_profile'])
              : undefined,
          channel: asString(payload.channel) as AggregationRequest['channel']
        };

        const output = aggregateResults(request);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.aggregate.complete', 'INFO', {
          suggested_actions: output.suggested_actions.length,
          follow_up_scheduled: output.follow_up_scheduled
        });
      })().catch((err) => {
        onError(500, 'aggregate_failed');
        logEvent(runtime, ctx, 'brain.aggregate.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/process') {
      void (async () => {
        const rawBody = await readRawBody(req);
        const payload = parseObject(rawBody);
        const messageText = asString(payload.message_text);
        if (!messageText) {
          onError(400, 'message_text_required');
          return;
        }

        const classification = classifyIntent(payload as IntentClassificationInput);
        const disambiguated = disambiguateSkills(
          {
            message_text: messageText,
            intent: classification.intent,
            candidate_skills: classification.skills,
            deployment_mode: asString(payload.deployment_mode) as DisambiguationRequest['deployment_mode'],
            user_tier: asString(payload.user_tier) as DisambiguationRequest['user_tier'],
            user_preferences:
              payload.user_preferences && typeof payload.user_preferences === 'object' && !Array.isArray(payload.user_preferences)
                ? (payload.user_preferences as DisambiguationRequest['user_preferences'])
                : undefined
          },
          runtime.disambiguationRules
        );
        const decomposition = decomposeTask(
          messageText,
          disambiguated.resolved_skills,
          classification.requires_decomposition
        );

        const syntheticResults: SkillResult[] = disambiguated.resolved_skills.map((skill) => ({
          skill_id: skill,
          status: 'SUCCESS',
          data: {
            note: 'execution delegated to hands-plane'
          }
        }));

        const aggregation = aggregateResults({
          skill_results: syntheticResults,
          user_profile:
            payload.user_profile && typeof payload.user_profile === 'object' && !Array.isArray(payload.user_profile)
              ? (payload.user_profile as AggregationRequest['user_profile'])
              : undefined,
          channel: asString(payload.channel) as AggregationRequest['channel']
        });

        sendJSON(res, 200, {
          classification,
          disambiguation: disambiguated,
          decomposition,
          aggregation
        });

        logEvent(runtime, ctx, 'brain.process.complete', 'INFO', {
          intent: classification.intent,
          resolved_skills: disambiguated.resolved_skills,
          tasks: decomposition.tasks.length
        });
      })().catch((err) => {
        if (err instanceof Error && err.message.startsWith('TASK_GRAPH_INVALID')) {
          sendJSON(res, 422, {
            error: 'TASK_GRAPH_INVALID',
            message: err.message
          });
          return;
        }
        onError(500, 'process_failed');
        logEvent(runtime, ctx, 'brain.process.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: BrainRuntime = {
    config: resolvedConfig,
    startedAtMs,
    disambiguationRules,
    server,
    async close(): Promise<void> {
      await new Promise<void>((resolve, reject) => {
        server.close((err) => {
          if (err) {
            reject(err);
            return;
          }
          resolve();
        });
      });
    }
  };

  runtimeRef = runtime;
  return runtime;
}

function installSignalHandlers(runtime: BrainRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'brain.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'brain.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'brain.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'brain.shutdown.failed', 'ERROR', {
        message: err instanceof Error ? err.message : String(err)
      });
      process.exit(1);
    }
  };

  process.on('SIGTERM', () => {
    void shutdown('SIGTERM');
  });
  process.on('SIGINT', () => {
    void shutdown('SIGINT');
  });
}

async function main(): Promise<void> {
  const runtime = buildRuntime();

  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(runtime.config.port, () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  installSignalHandlers(runtime);

  const ctx: RequestContext = {
    traceId: randomUUID(),
    spanId: randomUUID(),
    requestId: randomUUID()
  };

  logEvent(runtime, ctx, 'brain.started', 'INFO', {
    port: runtime.config.port,
    disambiguation_rules: runtime.disambiguationRules.length,
    disambiguation_config: runtime.config.disambiguationConfigPath
  });
}

void main().catch((err) => {
  process.stderr.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: 'brevio-brain',
      event: 'brain.start.failed',
      severity: 'ERROR',
      message: err instanceof Error ? err.message : String(err)
    }) + '\n'
  );
  process.exit(1);
});

export { buildRuntime as createBrainRuntime };
