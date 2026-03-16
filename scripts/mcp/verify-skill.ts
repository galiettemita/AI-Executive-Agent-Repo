#!/usr/bin/env npx tsx
/**
 * verify-skill.ts — Run live integration verification for a skill.
 * Usage: npx tsx scripts/mcp/verify-skill.ts <skill-id> [--input '{"action":"list"}']
 *
 * Set OAuth tokens via environment variables:
 *   BREVIO_OAUTH_GOOGLE_CALENDAR=ya29.xxx
 *   BREVIO_OAUTH_SLACK=xoxb-xxx
 */

const skillId = process.argv[2];
if (!skillId) {
  console.error('Usage: verify-skill <skill-id> [--input <json>]');
  console.error('Example: npx tsx scripts/mcp/verify-skill.ts google-calendar --input \'{"action":"list"}\'');
  process.exit(1);
}

const inputFlag = process.argv.indexOf('--input');
const rawInput = inputFlag >= 0 ? process.argv[inputFlag + 1] : '{}';
let input: Record<string, unknown>;
try {
  input = JSON.parse(rawInput ?? '{}');
} catch {
  console.error('Invalid --input JSON');
  process.exit(1);
}

let adapter: any;
try {
  const mod = await import(`../../services/hands-runtime/src/skills/${skillId}/index.js`);
  adapter = mod.default;
} catch (e: any) {
  console.error(`Skill not found or failed to load: ${skillId}`);
  console.error(e.message);
  process.exit(1);
}

const envKey = `BREVIO_OAUTH_${skillId.toUpperCase().replace(/-/g, '_')}`;
const oauthToken = process.env[envKey] ?? '';

const ctx = {
  userId: 'verify-cli',
  oauthTokens: new Map(oauthToken ? [[skillId, { accessToken: oauthToken }]] : []),
  userProfile: { id: 'verify-cli', timezone: 'UTC', locale: 'en-US' },
  logger: {
    info: (p: Record<string, unknown>) => console.log('[INFO]', JSON.stringify(p)),
    warn: (p: Record<string, unknown>) => console.warn('[WARN]', JSON.stringify(p)),
    error: (p: Record<string, unknown>) => console.error('[ERROR]', JSON.stringify(p)),
  },
  tracer: { startSpan: () => ({}) },
  cache: { get: async () => null, set: async () => {} },
  config: {},
};

console.log(`\nVerifying skill: ${skillId}`);
console.log(`Input: ${JSON.stringify(input, null, 2)}`);
if (oauthToken) {
  console.log(`OAuth: ${envKey} set (${oauthToken.slice(0, 8)}...)`);
} else {
  console.log(`OAuth: not set (set ${envKey} for authenticated calls)`);
}
console.log('');

const start = performance.now();
try {
  const result = await adapter.execute(input, ctx);
  const elapsed = (performance.now() - start).toFixed(1);
  console.log(`✓ Status: ${result.status} (${elapsed}ms)`);
  if (result.data) {
    console.log(`Output:\n${JSON.stringify(result.data, null, 2)}`);
  }
  if (result.error) {
    console.log(`Error: ${JSON.stringify(result.error)}`);
  }
  process.exit(result.status === 'SUCCESS' ? 0 : 1);
} catch (e: any) {
  const elapsed = (performance.now() - start).toFixed(1);
  console.error(`✗ Error (${elapsed}ms): ${e.message}`);
  process.exit(1);
}
