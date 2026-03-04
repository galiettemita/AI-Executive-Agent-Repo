import { randomUUID } from 'node:crypto';

import { logJSON } from './logger.js';
import { installSignalHandlers, startAuthService } from './server.js';

async function main(): Promise<void> {
  const runtime = await startAuthService();
  installSignalHandlers(runtime);

  logJSON(
    'service_started',
    'INFO',
    runtime.config.serviceName,
    runtime.config.environment,
    {
      traceId: randomUUID(),
      spanId: randomUUID(),
      correlationId: randomUUID()
    },
    {
      port: runtime.config.port,
      map_path: runtime.config.mapPath,
      oauth_services: runtime.serviceMap.oauth_services.length,
      api_key_services: runtime.serviceMap.api_key_services.length,
      no_auth_services: runtime.serviceMap.no_auth_services.length
    }
  );
}

void main().catch((err) => {
  process.stderr.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      event: 'service_start_failed',
      severity: 'ERROR',
      error: err instanceof Error ? err.message : String(err)
    }) + '\n'
  );
  process.exit(1);
});
