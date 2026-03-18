/**
 * Brevio k6 Smoke Test
 * Usage: k6 run tests/load/k6-smoke.js
 * Run after every deployment to verify basic functionality.
 */
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '1m',
  thresholds: {
    'http_req_duration': ['p(99)<2000'],
    'http_req_failed':   ['rate<0.01'],
  },
};

const GATEWAY_URL = (__ENV.GATEWAY_URL || 'http://localhost:18080').replace(/\/$/, '');
const BRAIN_URL   = (__ENV.BRAIN_URL   || 'http://localhost:18081').replace(/\/$/, '');

export function setup() {
  for (const base of [GATEWAY_URL, BRAIN_URL]) {
    const res = http.get(`${base}/healthz/ready`, { timeout: '5s' });
    if (res.status !== 200) console.error(`Not ready: ${base} (${res.status})`);
  }
}

export default function () {
  check(http.get(`${GATEWAY_URL}/health`), { 'gateway health 200': r => r.status === 200 });
  check(http.get(`${BRAIN_URL}/health/deep`), {
    'brain deep 200/503': r => r.status === 200 || r.status === 503,
    'has status field': r => { try { return Boolean(JSON.parse(r.body).status); } catch { return false; } },
  });
  check(http.post(`${BRAIN_URL}/v1/brain/ingest`, JSON.stringify({
    id: `smoke-${Date.now()}`, channel: 'sms', content: 'hello', workspace_id: 'ws-smoke',
  }), { headers: { 'Content-Type': 'application/json' }, timeout: '10s' }), {
    'ingest 2xx': r => r.status === 200 || r.status === 202,
  });
}
