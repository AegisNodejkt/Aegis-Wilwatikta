import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const errorRate = new Rate('errors');
const webhookLatency = new Trend('webhook_latency');
const queueEnqueueLatency = new Trend('queue_enqueue_latency');
const jobsProcessed = new Counter('jobs_processed');

export const options = {
  scenarios: {
    normal_load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m', target: 100 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '30s',
      exec: 'normalLoadTest',
    },
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 0 },
        { duration: '5s', target: 500 },
        { duration: '30s', target: 500 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '30s',
      exec: 'spikeTest',
      startTime: '2m',
    },
    sustained_high: {
      executor: 'constant-vus',
      vus: 200,
      duration: '60s',
      exec: 'sustainedHighTest',
      startTime: '4m',
    },
    concurrent_reviews: {
      executor: 'per-vu-iterations',
      vus: 100,
      iterations: 10,
      maxDuration: '5m',
      exec: 'concurrentReviewTest',
      startTime: '5m30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    errors: ['rate<0.05'],
    webhook_latency: ['p(95)<300', 'p(99)<500'],
    queue_enqueue_latency: ['p(95)<100'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const GITHUB_SECRET = __ENV.GITHUB_WEBHOOK_SECRET || 'test-secret';

function generateSignature(payload, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(payload);
  return 'sha256=' + hmac.digest('hex');
}

function generatePREvent(prNumber, action) {
  return {
    action: action || 'opened',
    number: prNumber,
    pull_request: {
      number: prNumber,
      title: 'Test PR #' + prNumber,
      body: 'Load test PR for performance benchmarking',
      state: 'open',
      base: { ref: 'main' },
      head: { ref: 'feature-' + prNumber },
      user: { login: 'testuser' },
    },
    repository: {
      id: 123456,
      name: 'test-repo',
      full_name: 'testowner/test-repo',
      owner: { login: 'testowner' },
    },
    installation: { id: 98765 },
    sender: { login: 'testuser' },
  };
}

function makeWebhookRequest(eventType, payload) {
  const payloadStr = JSON.stringify(payload);
  // In production, we'd use actual HMAC signature
  // For load testing, we use a mock signature or test mode
  const signature = 'sha256=test-signature';
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': eventType,
      'X-Hub-Signature-256': signature,
    },
    timeout: '30s',
  };
  
  return http.post(`${BASE_URL}/webhook`, payloadStr, params);
}

export function normalLoadTest() {
  const prNumber = (__VU * 100) + __ITER;
  const payload = generatePREvent(prNumber % 1000, 'opened');
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=loadtest-mock-signature',
    },
    timeout: '30s',
  };
  
  const start = Date.now();
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  const latency = Date.now() - start;
  
  webhookLatency.add(latency);
  
  const success = check(response, {
    'status is 202 (accepted)': (r) => r.status === 202 || r.status === 200,
    'response has event_id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.event_id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });
  
  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
    jobsProcessed.add(1);
  }
  
  sleep(1);
}

export function spikeTest() {
  const payload = generatePREvent(__VU, 'synchronize');
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=loadtest-mock-signature',
    },
    timeout: '30s',
  };
  
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  queueEnqueueLatency.add(response.timings.waiting);
  
  check(response, {
    'request accepted (202/200/429)': (r) => r.status === 202 || r.status === 200 || r.status === 429,
  });
  
  sleep(0.1);
}

export function sustainedHighTest() {
  const payload = generatePREvent((__VU * 1000) + __ITER, 'opened');
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=loadtest-mock-signature',
    },
    timeout: '30s',
  };
  
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  
  check(response, {
    'status acceptable': (r) => r.status < 500,
  });
  
  errorRate.add(response.status >= 400 ? 1 : 0);
  
  sleep(0.5);
}

export function concurrentReviewTest() {
  const payload = generatePREvent(__ITER, 'opened');
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=loadtest-mock-signature',
    },
    timeout: '30s',
  };
  
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  
  check(response, {
    'webhook accepted': (r) => r.status === 202,
    'has job_id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.job_id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });
  
  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    '/tmp/k6-summary.json': JSON.stringify(data),
  };
}

function textSummary(data, opts) {
  return JSON.stringify(data, null, 2);
}