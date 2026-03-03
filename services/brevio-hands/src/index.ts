import http from 'node:http';
import { randomUUID } from 'node:crypto';

import { getSkillAdapter, SkillRegistry } from './skills/index.js';

const serviceName = 'brevio-hands';
const version = process.env.SERVICE_VERSION ?? '0.1.0';
const start = Date.now();
const port = Number(process.env.PORT ?? 8080);
const ACTIVE_SKILLS = Object.keys(SkillRegistry).sort();

function healthPayload(deep: boolean): string {
  const checks: Record<string, string> = {
    process: 'ok'
  };

  if (deep) {
    checks.db = process.env.DATABASE_URL ? 'configured' : 'not_configured';
    checks.redis = process.env.REDIS_URL ? 'configured' : 'not_configured';
    checks.temporal = process.env.TEMPORAL_HOST ? 'configured' : 'not_configured';
    checks.skill_registry = ACTIVE_SKILLS.length > 0 ? 'loaded' : 'empty';
  }

  return JSON.stringify({
    status: 'healthy',
    service: serviceName,
    version,
    uptime_ms: Date.now() - start,
    checks,
    skill_count: ACTIVE_SKILLS.length
  });
}

type ExecuteRequest = {
  skill_id: string;
  user_id?: string;
  input?: Record<string, unknown>;
};

async function readJSONBody(req: http.IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    if (typeof chunk === 'string') {
      chunks.push(Buffer.from(chunk));
      continue;
    }
    chunks.push(chunk);
  }
  if (chunks.length === 0) {
    return {};
  }
  const body = Buffer.concat(chunks).toString('utf8');
  return JSON.parse(body);
}

const server = http.createServer(async (req, res) => {
  if (!req.url) {
    res.writeHead(400).end();
    return;
  }

  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(healthPayload(false));
    return;
  }

  if (req.method === 'GET' && req.url === '/health/deep') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(healthPayload(true));
    return;
  }

  if (req.method === 'GET' && req.url === '/v1/hands/skills') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(
      JSON.stringify({
        total: ACTIVE_SKILLS.length,
        skills: ACTIVE_SKILLS
      })
    );
    return;
  }

  if (req.method === 'POST' && req.url === '/v1/hands/execute') {
    try {
      const parsed = (await readJSONBody(req)) as ExecuteRequest;
      const skillId = String(parsed.skill_id ?? '').trim();
      if (skillId.length === 0) {
        res.writeHead(400, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'skill_id is required' }));
        return;
      }
      const adapter = getSkillAdapter(skillId);
      if (!adapter) {
        res.writeHead(404, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'skill_not_found', skill_id: skillId }));
        return;
      }
      const result = await adapter.execute(parsed.input ?? {}, {
        userId: String(parsed.user_id ?? randomUUID()),
        oauthTokens: new Map(),
        userProfile: {
          id: String(parsed.user_id ?? 'anonymous'),
          timezone: 'UTC',
          locale: 'en-US'
        },
        logger: {
          info() {},
          warn() {},
          error() {}
        },
        tracer: {
          startSpan() {
            return {};
          }
        },
        cache: {
          async get() {
            return null;
          },
          async set() {}
        },
        config: {}
      });
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(result));
      return;
    } catch (error) {
      res.writeHead(400, { 'content-type': 'application/json' });
      res.end(
        JSON.stringify({
          error: 'invalid_request',
          message: error instanceof Error ? error.message : 'invalid request payload'
        })
      );
      return;
    }
  }

  res.writeHead(404, { 'content-type': 'application/json' });
  res.end(JSON.stringify({ error: 'not_found', service: serviceName }));
});

server.listen(port, () => {
  process.stdout.write(JSON.stringify({ event: 'service_started', service: serviceName, port }) + '\n');
});

function shutdown(signal: string): void {
  process.stdout.write(JSON.stringify({ event: 'shutdown_start', service: serviceName, signal }) + '\n');
  server.close(() => {
    process.stdout.write(JSON.stringify({ event: 'shutdown_complete', service: serviceName }) + '\n');
    process.exit(0);
  });

  setTimeout(() => {
    process.stdout.write(JSON.stringify({ event: 'shutdown_timeout', service: serviceName }) + '\n');
    process.exit(1);
  }, 30_000).unref();
}

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));
