// k6 load test for the envoy-gateway recipe.
//
// One gateway per recipe (single production-realistic topology). Run:
//   BASE_URL=http://localhost:8082 task load_test_k8s RECIPE=envoy-gateway
//
// Each iteration randomly hits one of /users, /users/:id, /products,
// /products/:id so both backend services see traffic. Metrics carry
// a `gateway: envoy-gateway` tag so the Grafana panel queries match,
// and so dashboards from this recipe and the sibling cilium-gateway
// recipe overlay cleanly when imported into one Grafana.

import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL          = __ENV.BASE_URL          || "http://localhost:8082";
const CONCURRENT_USERS  = parseInt(__ENV.CONCURRENT_USERS  || "200");
const DURATION          = __ENV.DURATION          || "30s";
const RPS_CAP           = parseInt(__ENV.RPS_CAP           || "0");
const THINK_TIME_MS     = parseInt(__ENV.THINK_TIME_MS     || "0");

export const options = Object.assign(
  { tags: { gateway: "envoy-gateway" } },
  RPS_CAP > 0
    ? {
        scenarios: {
          ping: {
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
      },
);

const PATHS = [
  "/users",
  "/users/3",
  "/users/7",
  "/products",
  "/products/5",
  "/products/9",
];

export function setup() {
  console.log(`load target: ${BASE_URL} via envoy-gateway (${CONCURRENT_USERS} VUs, ${DURATION})`);

  // Sanity check: confirm both routes are reachable before the load
  // test starts. If the gateway isn't programmed yet (PROGRAMMED=False),
  // we want a clear error instead of a wall of failed iterations.
  for (const path of ["/users", "/products"]) {
    const r = http.get(`${BASE_URL}${path}`);
    if (r.status !== 200) {
      throw new Error(`pre-flight ${path} returned ${r.status}; gateway not ready`);
    }
  }
}

export default function () {
  const path = PATHS[Math.floor(Math.random() * PATHS.length)];
  const res = http.get(`${BASE_URL}${path}`);
  check(res, { "status is 200": (r) => r.status === 200 });
  if (THINK_TIME_MS > 0) sleep(THINK_TIME_MS / 1000);
}
