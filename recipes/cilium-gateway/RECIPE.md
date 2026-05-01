# Recipe: Cilium Gateway

## The problem

Cilium is best known as an eBPF Kubernetes CNI, but it also implements
the K8s **Gateway API** — letting you skip a separate L7 ingress and
use Cilium for both the cluster's networking and its north-south
gateway. This recipe demonstrates that single-product topology.

A separate recipe — [`envoy-gateway`](../envoy-gateway/RECIPE.md) —
runs the same workloads behind Envoy Gateway. Compare them by running
each recipe end-to-end and overlaying the Grafana dashboards: metric
names align so the panels match up.

> Different from the planned `service-mesh-cilium` recipe, which will
> demonstrate Cilium's mesh features (Hubble flow logs, mTLS,
> sidecarless service mesh). This recipe is **just the gateway**.

## The setup

| Component        | Choice                                          |
| ---------------- | ----------------------------------------------- |
| **CNI**          | Cilium (replaces kindnet + kube-proxy)          |
| **Gateway**      | Cilium Gateway API — `cilium` GatewayClass      |
| **Backends**     | `user-service` + `catalog-service` from `services/` |
| **Observability** | kube-prometheus-stack inside the kind cluster  |

> **Deployment mode**: Mode B (kind + Helm + Tilt) — see
> [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md). Self-contained
> like every K8s-mode recipe: the Tiltfile, kind cluster config, and
> bootstrap script all live under `recipes/cilium-gateway/`.

## Architecture

```text
┌─────────────┐     ┌──────────────────┐
│  k6 client  │────►│ cilium gateway   │──┐
│ (host)      │     │ (host :8083)     │  │   ┌─────────────────┐
└─────────────┘     └──────────────────┘  ├──►│ user-service    │ /users
                                          │   └─────────────────┘
                                          │   ┌─────────────────┐
                                          └──►│ catalog-service │ /products
                                              └─────────────────┘
```

One product (Cilium) is doing CNI + L4 + L7 ingress all in one. That's
the lesson — fewer moving parts than separating CNI from gateway.

## The demo

```bash
# 1. Create the kind cluster + bootstrap Cilium with gatewayAPI=true.
#    The script disables kindnet + kube-proxy, pre-installs Gateway
#    API CRDs (Cilium needs them at agent startup), then helm-installs
#    Cilium as both the CNI and the gateway controller.
task kind_up RECIPE=cilium-gateway

# 2. Tilt brings up monitoring + the workloads + the Gateway/HTTPRoute.
#    No second helm install for a gateway — Cilium is already that.
task tilt_up RECIPE=cilium-gateway

# 3. Sanity-check the routes.
curl -s http://localhost:8083/users      | jq .
curl -s http://localhost:8083/users/3    | jq .
curl -s http://localhost:8083/products   | jq .
curl -s http://localhost:8083/products/5 | jq .

# 4. Verify the GatewayClass and Gateway are programmed.
kubectl get gatewayclass                          # cilium, Accepted=True
kubectl get gateway -n cilium-gateway             # PROGRAMMED=True

# 5. Load test.
BASE_URL=http://localhost:8083 task load_test_k8s RECIPE=cilium-gateway

# 6. Open Grafana → http://localhost:3001 (admin/admin).
#    Dashboard: "Cilium Gateway".

# 7. Tear down.
task tilt_down RECIPE=cilium-gateway
task kind_down RECIPE=cilium-gateway
```

## What you'll see on the dashboard

- **Hero panel — p99 latency** from the k6 client side.
- p50/p95/p99 latency over time.
- Throughput (RPS) and error rate.
- Backend service health (request rate, handler latency per service).

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

k6 metrics carry the tag `gateway: "cilium"` so dashboards in this
recipe and the sibling envoy-gateway recipe overlay cleanly when
imported into one Grafana.

## Why Gateway API CRDs are pre-installed (and not in envoy-gateway)

Cilium with `gatewayAPI.enabled=true` only registers its `cilium`
GatewayClass if the upstream Gateway API CRDs already exist when its
agents start — Cilium's helm chart does **not** ship them. The
envoy-gateway recipe doesn't have this problem because Envoy
Gateway's chart bundles the CRDs. So this recipe's `kind-up.sh`
applies the upstream `kubernetes-sigs/gateway-api` experimental
channel CRDs before installing Cilium. The experimental channel is
required because Cilium registers a field indexer for `TLSRoute`
(`gateway.networking.k8s.io/v1alpha2`) at startup, and TLSRoute lives
only in the experimental channel.

## Why Host Network mode (and the listener port is 8080, not 80)

Cilium-Gateway's default service mode is `LoadBalancer` — the chart
auto-creates a LoadBalancer-typed Service per Gateway and waits for
an external IP. **kind has no LoadBalancer controller**, so that
Service stays at `EXTERNAL-IP=<pending>` forever, the Gateway never
reaches `PROGRAMMED=True`, and no traffic is routed.

Cilium docs explicitly recommend
[Host Network mode](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/)
for kind:

> Host network mode allows you to expose the Cilium Gateway API
> Gateway directly on the host network. This is useful in cases
> where a LoadBalancer Service is unavailable, such as in development
> environments.

In this mode `cilium-envoy` binds the listener port directly on each
node's network namespace — no LB Service, no NodePort. Combined with
kind's `extraPortMappings`, the host reaches the listener as
`http://localhost:8083`.

Cilium's host-network mode requires a non-privileged listener port
(>1023). We use `8080` in the Gateway resource and map host:8083 →
kind container:8080 in `kind-cluster.yaml`. (Privileged ports below
1024 would need extra `envoy.securityContext.capabilities` config we
deliberately skip — keeps the recipe small.)
