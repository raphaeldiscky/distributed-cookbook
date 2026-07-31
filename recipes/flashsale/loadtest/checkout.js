// k6 load test for the flashsale recipe.
//
// All three adapters are live on one server. Choose which adapter to test
// via ADAPTER=<name>; unset it to hit the default (/checkout).
//
// Single-adapter run (simplest):
//   ADAPTER=naive     CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
//   ADAPTER=pg_cond   CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
//   ADAPTER=redis_lua CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
//
// Between runs, the server keeps running. No restart needed.
//
// Set BASE_URL to override the default http://localhost:8081.

import http from "k6/http";
import { check, sleep } from "k6";

// BASE_URLS (comma-separated) spreads load across replicas, which is the only way
// token_queue's per-replica quota fragmentation becomes visible. A single
// BASE_URL stays the default.
const BASE_URLS         = (__ENV.BASE_URLS || __ENV.BASE_URL || "http://localhost:8081")
                            .split(",").map((s) => s.trim()).filter(Boolean);
const ADAPTER           = __ENV.ADAPTER           || "";    // "" → server default
const PRODUCT_COUNT     = parseInt(__ENV.PRODUCT_COUNT     || "1");
const INITIAL_STOCK     = parseInt(__ENV.INITIAL_STOCK     || "200");
const CONCURRENT_USERS  = parseInt(__ENV.CONCURRENT_USERS  || "200");
const DURATION          = __ENV.DURATION          || "15s";
const RPS_CAP           = parseInt(__ENV.RPS_CAP           || "0");
const THINK_TIME_MS     = parseInt(__ENV.THINK_TIME_MS     || "0");

const CHECKOUT_PATH = ADAPTER ? `/checkout/${ADAPTER}` : "/checkout";

export const options = RPS_CAP > 0
  ? {
      scenarios: {
        checkout: {
          executor: "constant-arrival-rate",
          rate: RPS_CAP,
          timeUnit: "1s",
          duration: DURATION,
          preAllocatedVUs: CONCURRENT_USERS,
          maxVUs: CONCURRENT_USERS,
        },
      },
    }
  : {
      vus: CONCURRENT_USERS,
      duration: DURATION,
    };

export function setup() {
  // POST /seed primes ALL adapters' state (PG row for the pg_* kinds, Redis key
  // for the redis_* ones, in-memory counters for go_chan and token_queue). One
  // call per product covers every adapter on that server.
  //
  // With several replicas, each gets an equal SHARE of the stock, because a
  // replica-local quota that started at the full stock would let every replica
  // sell the whole sale. Splitting it is the pattern, and the stranded remainder
  // is its cost.
  const share = Math.floor(INITIAL_STOCK / BASE_URLS.length);

  for (const base of BASE_URLS) {
    for (let i = 1; i <= PRODUCT_COUNT; i++) {
      const r = http.post(
        `${base}/seed`,
        JSON.stringify({ product_id: i, name: `item-${i}`, stock: share }),
        { headers: { "Content-Type": "application/json" } }
      );
      check(r, { "seed 200": (res) => res.status === 200 });
    }
  }
  console.log(
    `load target: ${BASE_URLS.length} replica(s) ${CHECKOUT_PATH} ` +
      `(${CONCURRENT_USERS} VUs, ${DURATION}, ${share}/replica x ${PRODUCT_COUNT} products)`
  );
}

export default function () {
  const productID = (Math.floor(Math.random() * PRODUCT_COUNT)) + 1;
  // Round-robin by VU id so load spreads evenly rather than by luck of the draw.
  const base = BASE_URLS[(__VU - 1) % BASE_URLS.length];
  const res = http.post(
    `${base}${CHECKOUT_PATH}`,
    JSON.stringify({ product_id: productID, qty: 1 }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(res, {
    "status is 200 or 409": (r) => r.status === 200 || r.status === 409,
  });
  if (THINK_TIME_MS > 0) sleep(THINK_TIME_MS / 1000);
}
