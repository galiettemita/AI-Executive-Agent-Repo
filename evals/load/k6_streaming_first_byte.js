import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';
import { Trend } from 'k6/metrics';

const firstByteMs = new Trend('BREVIO_streaming_first_byte_ms');

export const options = {
  thresholds: {
    http_req_failed: ['rate<0.005'],
    checks: ['rate>0.999'],
    BREVIO_streaming_first_byte_ms: ['p(95)<500'],
  },
  scenarios: {
    streaming_first_byte_probe: {
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
    channel_identifier: '+15550007777',
    user_channel_id: 'k6-streaming-user',
    nonce: `k6-streaming-${__VU}-${__ITER}`,
    message: 'streaming first-byte probe',
    metadata: { workload: 'streaming_first_byte' },
  });

  const res = http.post(`${BASE_URL}/v1/gateway/webhook/whatsapp`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Signature': signatureFor(payload),
      'X-Workload-Tier': 'T1',
    },
  });

  firstByteMs.add(res.timings.waiting);

  check(res, {
    'status accepted or controlled reject': (r) => [202, 429, 409, 401].includes(r.status),
  });

  sleep(0.1);
}
