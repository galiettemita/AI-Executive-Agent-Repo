import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';
import { Trend } from 'k6/metrics';

const t1Latency = new Trend('BREVIO_tier_t1_latency_ms');
const t2Latency = new Trend('BREVIO_tier_t2_latency_ms');
const t3Latency = new Trend('BREVIO_tier_t3_latency_ms');

export const options = {
  thresholds: {
    http_req_failed: ['rate<0.005'],
    checks: ['rate>0.9995'],
    BREVIO_tier_t1_latency_ms: ['p(95)<2500'],
    BREVIO_tier_t2_latency_ms: ['p(95)<6000'],
    BREVIO_tier_t3_latency_ms: ['p(95)<20000'],
  },
  scenarios: {
    tier_t1: {
      executor: 'constant-vus',
      vus: 24,
      duration: '3m',
      env: { WORKLOAD_TIER: 'T1' },
    },
    tier_t2: {
      executor: 'constant-vus',
      vus: 12,
      duration: '3m',
      env: { WORKLOAD_TIER: 'T2' },
    },
    tier_t3: {
      executor: 'constant-vus',
      vus: 6,
      duration: '2m',
      env: { WORKLOAD_TIER: 'T3' },
    },
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18082';
const WEBHOOK_SECRET = __ENV.WEBHOOK_SECRET || 'dev-secret';

function signatureFor(payload) {
  return crypto.hmac('sha256', WEBHOOK_SECRET, payload, 'hex');
}

export default function () {
  const tier = __ENV.WORKLOAD_TIER || 'T1';
  const messagesByTier = {
    T1: 'prepare summary for today',
    T2: 'draft a client-ready weekly status memo with highlights and risks',
    T3: 'plan a multi-step executive operation with dependencies, contingencies, and rollout sequencing',
  };
  const payload = JSON.stringify({
    channel: 'whatsapp',
    channel_identifier: '+15550001111',
    user_channel_id: 'k6-user',
    nonce: `k6-${tier}-${__VU}-${__ITER}`,
    message: messagesByTier[tier] || messagesByTier.T1,
    metadata: { tier },
  });

  const res = http.post(`${BASE_URL}/v1/gateway/webhook/whatsapp`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Signature': signatureFor(payload),
      'X-Workload-Tier': tier,
    },
  });

  if (tier === 'T1') {
    t1Latency.add(res.timings.duration);
  } else if (tier === 'T2') {
    t2Latency.add(res.timings.duration);
  } else if (tier === 'T3') {
    t3Latency.add(res.timings.duration);
  }

  check(res, {
    'status is expected': (r) => r.status === 202 || r.status === 429 || r.status === 409 || r.status === 401,
  });

  sleep(0.2);
}
