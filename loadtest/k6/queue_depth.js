import http from 'k6/http';
import { check, sleep } from 'k6';
import { Gauge, Trend } from 'k6/metrics';

const queueDepth = new Gauge('queue_depth');
const queueLatency = new Trend('queue_latency');

export const options = {
  scenarios: {
    queue_flood: {
      executor: 'constant-arrival-rate',
      rate: 200,
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 50,
      maxVUs: 500,
      exec: 'queueFloodTest',
    },
    queue_drain: {
      executor: 'ramping-arrival-rate',
      startRate: 100,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 100,
      stages: [
        { duration: '30s', target: 0 },
      ],
      startTime: '70s',
      exec: 'queueDrainTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    queue_latency: ['p(95)<500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

let jobIds = [];

export function queueFloodTest() {
  const prNumber = (__VU * 1000) + __ITER;
  
  const payload = {
    action: 'opened',
    number: prNumber,
    pull_request: {
      number: prNumber,
      title: 'Queue depth test PR',
      body: 'Testing queue behavior under load',
      state: 'open',
      base: { ref: 'main' },
      head: { ref: 'queue-test-' + prNumber },
      user: { login: 'queuetester' },
    },
    repository: {
      id: 99999,
      name: 'queue-test-repo',
      full_name: 'queuetest/queue-test-repo',
      owner: { login: 'queuetest' },
    },
    installation: { id: 77777 },
    sender: { login: 'queuetester' },
  };
  
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=queue-test-signature',
    },
    timeout: '30s',
  };
  
  const start = Date.now();
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  const latency = Date.now() - start;
  
  queueLatency.add(latency);
  
  check(response, {
    'enqueued or rate limited': (r) => 
      r.status === 202 || r.status === 200 || r.status === 429 || r.status === 503,
  });
  
  if (response.status === 202) {
    try {
      const body = JSON.parse(response.body);
      if (body.job_id) {
        jobIds.push(body.job_id);
      }
    } catch (e) {}
  }
  
  sleep(0.05);
}

export function queueDrainTest() {
  if (jobIds.length === 0) {
    sleep(1);
    return;
  }
  
  const jobId = jobIds.pop();
  
  const response = http.get(`${BASE_URL}/jobs/${jobId}`);
  
  check(response, {
    'job status retrieved': (r) => r.status === 200 || r.status === 404,
  });
  
  if (response.status === 200) {
    try {
      const job = JSON.parse(response.body);
      if (job.status === 'completed' || job.status === 'failed') {
        queueDepth.add(0);
      } else {
        queueDepth.add(jobIds.length);
      }
    } catch (e) {}
  }
  
  sleep(0.5);
}

export function handleSummary(data) {
  console.log('Queue Test Summary:');
  console.log(`Peak queue depth: ${queueDepth}`);
  console.log(`Queue latency p95: ${queueLatency}`);
  return {
    'stdout': JSON.stringify(data, null, 2),
    '/tmp/k6-queue-summary.json': JSON.stringify(data),
  };
}