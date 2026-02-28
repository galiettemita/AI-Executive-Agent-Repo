import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';

export const options = {
  thresholds: {
    http_req_failed: ['rate<0.005'],
    http_req_duration: ['p(95)<2500'],
  },
  scenarios: {
    steady: {
      executor: 'constant-vus',
      vus: 20,
      duration: '2m',
    },
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18082';
const WEBHOOK_SECRET = __ENV.WEBHOOK_SECRET || 'dev-secret';

function signatureFor(payload) {
  return crypto.hmac('sha256', WEBHOOK_SECRET, payload, 'hex');
}

export default function () {
  const payload = JSON.stringify({
    channel: 'whatsapp',
    channel_identifier: '+15550001111',
    user_channel_id: 'k6-user',
    nonce: `k6-${__VU}-${__ITER}`,
    message: 'prepare summary for today',
  });

  const res = http.post(`${BASE_URL}/v1/gateway/webhook/whatsapp`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Signature': signatureFor(payload),
    },
  });

  check(res, {
    'status is 2xx/4xx expected': (r) => r.status >= 200 && r.status < 500,
  });

  sleep(0.2);
}
