import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const p95Latency = new Trend('p95_latency');
const throughputRate = new Trend('throughput_rps');

export const options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 500 },
    { duration: '2m', target: 1000 },
    { duration: '5m', target: 1000 },
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<5000', 'p(99)<10000'],
    errors: ['rate<0.10'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function generateLargeDiff() {
  let diff = '';
  for (let i = 0; i < 50; i++) {
    diff += `diff --git a/src/component${i}.go b/src/component${i}.go\n`;
    diff += `new file mode 100644\n`;
    diff += `index 0000000..1234567\n`;
    diff += `--- /dev/null\n`;
    diff += `+++ b/src/component${i}.go\n`;
    diff += `@@ -0,0 +1,100 @@\n`;
    for (let j = 0; j < 100; j++) {
      diff += `+package component\n`;
      diff += `+\n`;
      diff += `+func Process${j}() error {\n`;
      diff += `+    // Implementation code\n`;
      diff += `+    return nil\n`;
      diff += `+}\n`;
    }
  }
  return diff;
}

export default function () {
  const prNumber = (__VU * 1000) + __ITER;
  
  const payload = {
    action: 'opened',
    number: prNumber,
    pull_request: {
      number: prNumber,
      title: 'Large PR for stress testing',
      body: generateLargeDiff(),
      state: 'open',
      base: { ref: 'main' },
      head: { ref: 'stress-test-' + prNumber },
      user: { login: 'stressuser' },
      additions: 5000,
      deletions: 0,
      changed_files: 50,
    },
    repository: {
      id: 123456,
      name: 'stress-repo',
      full_name: 'stressowner/stress-repo',
      owner: { login: 'stressowner' },
    },
    installation: { id: 88888 },
    sender: { login: 'stressuser' },
  };
  
  const payloadStr = JSON.stringify(payload);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-GitHub-Event': 'pull_request',
      'X-Hub-Signature-256': 'sha256=stress-test-signature',
    },
    timeout: '60s',
  };
  
  const start = Date.now();
  const response = http.post(`${BASE_URL}/webhook`, payloadStr, params);
  const duration = Date.now() - start;
  
  p95Latency.add(duration);
  
  const success = check(response, {
    'request handled': (r) => r.status === 202 || r.status === 200 || r.status === 429,
    'response time acceptable': () => duration < 10000,
  });
  
  if (!success) {
    errorRate.add(1);
  }
  
  throughputRate.add(1);
}