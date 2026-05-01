# Recipe: Envoy Gateway

## The problem

You have microservices in a Kubernetes cluster and you need an L7
ingress in front of them. Envoy Gateway is the reference K8s
**Gateway API** implementation: a control plane that translates
`Gateway` and `HTTPRoute` resources into Envoy proxy configuration.

This recipe stands up a kind cluster running Envoy Gateway, routes
two universal backend services (`user-service`, `catalog-service`)
through it, and load-tests with k6.

A separate recipe — [`cilium-gateway`](../cilium-gateway/RECIPE.md) —
runs the same backends behind Cilium's Gateway API implementation.
Compare them by running each recipe end-to-end and overlaying the
Grafana dashboards: the metric names align so the panels match up.

## The setup

| Component        | Choice                                          |
| ---------------- | ----------------------------------------------- |
| **CNI**          | kindnet (default kind networking)               |
| **Gateway**      | Envoy Gateway v1.6.7 — `eg` GatewayClass        |
| **Backends**     | `user-service` + `catalog-service` from `services/` |
| **Observability** | kube-prometheus-stack inside the kind cluster  |

> **Deployment mode**: Mode B (kind + Helm + Tilt) — see
> [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md). Each K8s-mode
> recipe is self-contained: its Tiltfile, kind cluster config, and
> bootstrap script all live under `recipes/<name>/`. The compose-based
> monitoring stack from other recipes stays untouched.
>
> **Hero metric deviation** (per CONVENTIONS § 2): this recipe's
> "correctness claim" is *performance under load*, not a 0/non-zero
> invariant like flashsale's `oversell_total`. The hero panel is p99
> latency from the k6 client side.

## Architecture

```text
┌─────────────┐     ┌──────────────────┐
│  k6 client  │────►│ envoy-gateway    │──┐
│ (host)      │     │ (host :8082)     │  │   ┌─────────────────┐
└─────────────┘     └──────────────────┘  ├──►│ user-service    │ /users
                                          │   └─────────────────┘
                                          │   ┌─────────────────┐
                                          └──►│ catalog-service │ /products
                                              └─────────────────┘
```

One gateway, one cluster — production-realistic. To compare against
Cilium Gateway, tear this recipe down and run the
`cilium-gateway` recipe (different cluster, different host port).

## The demo

```bash
# 1. Create the kind cluster (kindnet CNI; no special bootstrap needed).
task kind_up RECIPE=envoy-gateway

# 2. Bring up the recipe via Tilt (kube-prometheus-stack, envoy-gateway
#    controller, the two backend services, the Gateway + HTTPRoute, the
#    Grafana dashboard ConfigMap).
task tilt_up RECIPE=envoy-gateway
# Open http://localhost:10350 (Tilt UI) and wait for everything green.

# 3. Sanity-check the routes.
curl -s http://localhost:8082/users      | jq .   # → user-service
curl -s http://localhost:8082/users/3    | jq .
curl -s http://localhost:8082/products   | jq .   # → catalog-service
curl -s http://localhost:8082/products/5 | jq .

# 4. Verify the Gateway is programmed.
kubectl get gatewayclass                          # eg, Accepted=True
kubectl get gateway -n envoy-gateway              # PROGRAMMED=True
kubectl get httproute -n envoy-gateway            # ACCEPTED=True

# 5. Load test.
BASE_URL=http://localhost:8082 task load_test_k8s RECIPE=envoy-gateway

# 6. Open Grafana → http://localhost:3001 (admin/admin).
#    Dashboard: "Envoy Gateway".

# 7. Tear down.
task tilt_down RECIPE=envoy-gateway
task kind_down RECIPE=envoy-gateway
```

## What you'll see on the dashboard

- **Hero panel — p99 latency** from the k6 client side.
- p50/p95/p99 latency over time.
- Throughput (RPS) and error rate.
- Backend service health (request rate, handler latency per service) —
  sanity check that load reaches both pods evenly.

## HTTP surface (through the gateway)

| Method | Path             | Backend service                   |
| ------ | ---------------- | --------------------------------- |
| GET    | `/users`         | `user-service` `/users`           |
| GET    | `/users/:id`     | `user-service` `/users/:id`       |
| GET    | `/products`      | `catalog-service` `/products`     |
| GET    | `/products/:id`  | `catalog-service` `/products/:id` |

## Metrics

Backend services emit (per `docs/CONVENTIONS.md § 2`, namespace
`cookbook`, subsystem = service name):

- `cookbook_user_service_requests_total{route, status}`
- `cookbook_user_service_request_duration_seconds{route}`
- `cookbook_catalog_service_requests_total{route, status}`
- `cookbook_catalog_service_request_duration_seconds{route}`

k6 metrics (e.g. `k6_http_req_duration_p99`) come from `k6 run --out
experimental-prometheus-rw`, written to the in-cluster Prometheus.

## Production notes

This recipe matches what an Envoy-Gateway-only production cluster
looks like: any standard CNI (here kindnet, in prod typically Calico
or Cilium-as-CNI) plus Envoy Gateway as the L7 ingress. If you want
the same comparison done with Cilium-as-Gateway-API, run the sibling
`cilium-gateway` recipe — it spins up its own kind cluster with
Cilium acting as both CNI and Gateway controller.
