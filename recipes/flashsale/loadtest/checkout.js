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

const BASE_URL          = __ENV.BASE_URL          || "http://localhost:8081";
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
  // POST /seed primes ALL adapters' state (PG row for naive/pg_cond,
  // Redis key for redis_lua). One call per product covers everyone.
  for (let i = 1; i <= PRODUCT_COUNT; i++) {
    const r = http.post(
      `${BASE_URL}/seed`,
      JSON.stringify({ product_id: i, name: `item-${i}`, stock: INITIAL_STOCK }),
      { headers: { "Content-Type": "application/json" } }
    );
    check(r, { "seed 200": (res) => res.status === 200 });
  }
  console.log(`load target: ${BASE_URL}${CHECKOUT_PATH} (${CONCURRENT_USERS} VUs, ${DURATION})`);
}

export default function () {
  const productID = (Math.floor(Math.random() * PRODUCT_COUNT)) + 1;
  const res = http.post(
    `${BASE_URL}${CHECKOUT_PATH}`,
    JSON.stringify({ product_id: productID, qty: 1 }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(res, {
    "status is 200 or 409": (r) => r.status === 200 || r.status === 409,
  });
  if (THINK_TIME_MS > 0) sleep(THINK_TIME_MS / 1000);
}
