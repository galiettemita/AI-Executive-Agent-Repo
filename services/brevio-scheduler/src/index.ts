import http from 'node:http';

const serviceName = 'brevio-scheduler';
const version = process.env.SERVICE_VERSION ?? '0.1.0';
const start = Date.now();
const port = Number(process.env.PORT ?? 8080);

function healthPayload(deep: boolean): string {
  const checks: Record<string, string> = {
    process: 'ok'
  };

  if (deep) {
    checks.db = process.env.DATABASE_URL ? 'configured' : 'not_configured';
    checks.redis = process.env.REDIS_URL ? 'configured' : 'not_configured';
    checks.temporal = process.env.TEMPORAL_HOST ? 'configured' : 'not_configured';
  }

  return JSON.stringify({
    status: 'healthy',
    service: serviceName,
    version,
    uptime_ms: Date.now() - start,
    checks
  });
}

const server = http.createServer((req, res) => {
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
