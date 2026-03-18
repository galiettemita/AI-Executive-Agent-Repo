import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';

export const options = {
  scenarios: {
    d0: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D0' } },
    d1: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D1' } },
    d2: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D2' } },
    d3: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D3' } },
    d4: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D4' } },
    d5: { executor: 'constant-vus', vus: 8, duration: '45s', env: { SHED_TIER: 'D5' } },
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18082';
const WEBHOOK_SECRET = __ENV.WEBHOOK_SECRET || 'dev-secret';

function signatureFor(payload) {
  return crypto.hmac('sha256', WEBHOOK_SECRET, payload, 'hex');
}

export default function () {
  const tier = __ENV.SHED_TIER || 'D0';
  const payload = JSON.stringify({
    channel: 'whatsapp',
    channel_identifier: '+15550009999',
    user_channel_id: 'k6-load-shed-user',
    nonce: `k6-load-shed-${tier}-${__VU}-${__ITER}`,
    message: `load shedding probe ${tier}`,
    metadata: { load_shedding_tier: tier },
  });

  const res = http.post(`${BASE_URL}/v1/gateway/webhook/whatsapp`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Signature': signatureFor(payload),
      'X-Load-Shedding-Tier': tier,
    },
  });

  check(res, {
    'status accepted or controlled reject': (r) => [202, 429, 409, 401].includes(r.status),
  });

  sleep(0.2);
}
