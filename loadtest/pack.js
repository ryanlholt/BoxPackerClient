// k6 load test for boxpackerclient's POST /pack endpoint.
//
//   brew install k6            # if not already installed
//   boxpackerclient -http :8080
//   k6 run loadtest/pack.js
//
// Env knobs (all optional):
//   BASE=http://localhost:8080   target service
//   PROFILE=ramp|rate|spike|soak which scenario to run (default: ramp)
//   RATE=200                      requests/sec for the constant-rate scenario
//
// Example:
//   PROFILE=rate RATE=500 k6 run loadtest/pack.js

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const BASE = __ENV.BASE || 'http://localhost:8080';
const PROFILE = __ENV.PROFILE || 'ramp';
const RATE = parseInt(__ENV.RATE || '200', 10);

// Custom metric so we can see latency split out by payload size.
const packDuration = new Trend('pack_duration', true);
const packFailures = new Counter('pack_failures');

// ---------------------------------------------------------------------------
// Payload generation: real traffic is a MIX of problem sizes, not one shape.
// The packer is CPU-bound, so item/box count dominates per-request cost.
// We weight toward small/medium (typical) with a long tail of large problems.
// ---------------------------------------------------------------------------

const BOX_CATALOG = [
  { reference: 'small mailer', outerWidth: 230, outerLength: 300, outerDepth: 240, emptyWeight: 160, innerWidth: 220, innerLength: 290, innerDepth: 230, maxWeight: 15000 },
  { reference: 'large mailer', outerWidth: 370, outerLength: 375, outerDepth: 380, emptyWeight: 410, innerWidth: 360, innerLength: 365, innerDepth: 370, maxWeight: 15000 },
  { reference: 'xl box',       outerWidth: 500, outerLength: 500, outerDepth: 500, emptyWeight: 800, innerWidth: 490, innerLength: 490, innerDepth: 490, maxWeight: 30000 },
];

const ITEM_TEMPLATES = [
  { description: 'mug',  width: 110, length: 110, depth: 105, weight: 350, rotation: 'never' },
  { description: 'book', width: 210, length: 130, depth: 30,  weight: 450, rotation: 'keepFlat' },
  { description: 'toy',  width: 80,  length: 60,  depth: 60,  weight: 150, rotation: 'best' },
  { description: 'cable',width: 40,  length: 40,  depth: 120, weight: 80,  rotation: 'best' },
];

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

// Pick a problem-size bucket: 70% small, 22% medium, 8% large (long tail).
function pickItemCount() {
  const r = Math.random();
  if (r < 0.70) return randInt(1, 5);     // small  — typical order
  if (r < 0.92) return randInt(6, 20);    // medium
  return randInt(21, 80);                 // large  — stress the packer
}

function buildPayload() {
  const lines = pickItemCount();
  const items = [];
  for (let i = 0; i < lines; i++) {
    const t = ITEM_TEMPLATES[randInt(0, ITEM_TEMPLATES.length - 1)];
    items.push(Object.assign({}, t, { quantity: randInt(1, 6) }));
  }
  return {
    boxes: BOX_CATALOG,
    items: items,
    options: { allowPartialResults: true },
  };
}

// ---------------------------------------------------------------------------
// Scenarios. One is selected via PROFILE so you can run them independently.
// ---------------------------------------------------------------------------

const scenarios = {
  // Ramp concurrent virtual users up, hold, ramp down. Good first look at how
  // latency degrades as you push past CPU saturation.
  ramp: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 50 },
      { duration: '1m',  target: 200 },
      { duration: '1m',  target: 200 },
      { duration: '30s', target: 0 },
    ],
    gracefulRampDown: '10s',
  },
  // Hold a fixed REQUEST RATE regardless of latency. This is the realistic
  // "we get N orders/sec" model and the one to report SLOs against.
  rate: {
    executor: 'constant-arrival-rate',
    rate: RATE,
    timeUnit: '1s',
    duration: '2m',
    preAllocatedVUs: 50,
    maxVUs: 2000,
  },
  // Sudden traffic spike (flash sale / batch job kicks in).
  spike: {
    executor: 'ramping-arrival-rate',
    startRate: 50,
    timeUnit: '1s',
    preAllocatedVUs: 100,
    maxVUs: 3000,
    stages: [
      { duration: '20s', target: 50 },
      { duration: '10s', target: 1000 },  // spike
      { duration: '30s', target: 1000 },
      { duration: '20s', target: 50 },
    ],
  },
  // Long, steady run to catch leaks / GC pauses / degradation over time.
  soak: {
    executor: 'constant-arrival-rate',
    rate: RATE,
    timeUnit: '1s',
    duration: '30m',
    preAllocatedVUs: 50,
    maxVUs: 1000,
  },
};

export const options = {
  scenarios: { [PROFILE]: scenarios[PROFILE] },
  thresholds: {
    http_req_failed: ['rate<0.01'],          // <1% errors
    http_req_duration: ['p(95)<500', 'p(99)<1500'],
  },
};

export default function () {
  const payload = JSON.stringify(buildPayload());
  const res = http.post(`${BASE}/pack`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  packDuration.add(res.timings.duration);

  const ok = check(res, {
    'status 200': (r) => r.status === 200,
    'has body':   (r) => r.body && r.body.length > 0,
  });
  if (!ok) packFailures.add(1);
}
