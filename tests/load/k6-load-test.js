/**
 * Brevio k6 Load Test
 * Usage: k6 run tests/load/k6-load-test.js
 * Env vars:
 *   GATEWAY_URL    — defaults to http://localhost:18080
 *   BRAIN_URL      — defaults to http://localhost:18081
 *   WORKSPACE_ID   — defaults to ws-loadtest-001
 */
import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Trend, Counter, Rate, Gauge } from 'k6/metrics';

const e2eLatency      = new Trend('brevio_e2e_latency_ms',    true);
const workflowLatency = new Trend('brevio_workflow_start_ms', true);
const successRate     = new Rate('brevio_success_rate');
const timeoutErrors   = new Counter('brevio_timeout_errors');
const activeWorkflows = new Gauge('brevio_active_workflows');

export const options = {
  scenarios: {
    ramp_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m',  target: 10  },
        { duration: '5m',  target: 50  },
        { duration: '2m',  target: 100 },
        { duration: '2m',  target: 50  },
        { duration: '1m',  target: 0   },
      ],
    },
  },
  thresholds: {
    'brevio_e2e_latency_ms': ['p(95)<15000', 'p(99)<30000'],
    'brevio_success_rate': ['rate>0.99'],
    'http_req_duration': ['p(99)<20000'],
    'http_req_failed': ['rate<0.01'],
    'brevio_timeout_errors': ['count<500'],
  },
};

const GATEWAY_URL  = (__ENV.GATEWAY_URL  || 'http://localhost:18080').replace(/\/$/, '');
const BRAIN_URL    = (__ENV.BRAIN_URL    || 'http://localhost:18081').replace(/\/$/, '');
const WORKSPACE_ID = __ENV.WORKSPACE_ID  || 'ws-loadtest-001';

const TEST_MESSAGES = [
  "What's on my calendar today?",
  "Schedule a 30-minute call with Bob tomorrow at 2pm",
  "When is my next meeting?",
  "Move my 3pm meeting to 4pm",
  "Show me unread emails from today",
  "Send an email to alice@example.com about the Q2 review",
  "Add a task to review the budget proposal by Friday",
  "What tasks are due this week?",
  "Search for the latest news about AI agents",
  "What is the weather in New York today?",
  "Hello",
  "Thank you",
  "What can you help me with?",
  "What's my account balance?",
];

export function setup() {
  const healthChecks = [
    { name: 'gateway', url: `${GATEWAY_URL}/healthz/ready` },
    { name: 'brain',   url: `${BRAIN_URL}/healthz/ready` },
  ];
  for (const svc of healthChecks) {
    const res = http.get(svc.url, { timeout: '10s' });
    if (res.status !== 200) {
      console.error(`PREFLIGHT FAILED: ${svc.name} not ready (status ${res.status})`);
    }
  }
  return { startTime: Date.now() };
}

export default function () {
  const message = TEST_MESSAGES[Math.floor(Math.random() * TEST_MESSAGES.length)];
  const msgID   = `load-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

  group('ingest_message', () => {
    const payload = JSON.stringify({
      id:           msgID,
      channel:      'whatsapp',
      content:      message,
      workspace_id: WORKSPACE_ID,
    });

    const start = Date.now();
    const res = http.post(`${BRAIN_URL}/v1/brain/ingest`, payload, {
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': msgID },
      timeout: '30s',
    });
    const elapsed = Date.now() - start;

    const ok = check(res, {
      'status 200 or 202': r => r.status === 200 || r.status === 202,
      'has workflow_id':   r => {
        try { const b = JSON.parse(r.body); return Boolean(b.workflow_id || b.message_id); }
        catch { return false; }
      },
      'latency < 5s': () => elapsed < 5000,
    });

    e2eLatency.add(elapsed);
    workflowLatency.add(elapsed);
    successRate.add(ok ? 1 : 0);
    if (res.status === 408 || res.status === 504 || res.timings.duration > 25000) {
      timeoutErrors.add(1);
    }
    activeWorkflows.add(ok ? 1 : 0);
  });

  sleep(1);
}

export function teardown(data) {
  console.log(`Load test completed in ${((Date.now() - data.startTime) / 1000).toFixed(1)}s`);
}

export function handleSummary(data) {
  return {
    'tests/load/k6-results.json': JSON.stringify(data, null, 2),
    'stdout': buildSummary(data),
  };
}

function buildSummary(data) {
  const m = data.metrics;
  const lines = [
    '\n=== BREVIO LOAD TEST SUMMARY ===',
  ];
  const e2e = m.brevio_e2e_latency_ms;
  if (e2e) {
    lines.push(`  E2E latency p95: ${e2e['p(95)'].toFixed(0)}ms  (SLO: < 15,000ms)`);
    lines.push(`  E2E latency p99: ${e2e['p(99)'].toFixed(0)}ms`);
  }
  const sr = m.brevio_success_rate;
  if (sr) lines.push(`  Success rate: ${(sr.rate * 100).toFixed(2)}%  (SLO: >= 99%)`);
  const to = m.brevio_timeout_errors;
  if (to) lines.push(`  Timeout errors: ${to.count}`);
  lines.push('');
  return lines.join('\n');
}
