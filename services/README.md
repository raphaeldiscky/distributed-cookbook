# services/ — universal cookbook microservices

A small, stable catalogue of HTTP services that recipes can compose
together. Inspired by demo apps like Google's Online Boutique and
HashiCorp's HashiCups: instead of every recipe inventing its own
backend, recipes pick from this shared catalogue and focus on the
*lesson* (gateway routing, mesh policy, tracing fan-out, retries, …).

## What's here

| Service                                | Domain    | Endpoints                       |
| -------------------------------------- | --------- | ------------------------------- |
| [user-service](./user-service)         | Users     | `/users`, `/users/:id`          |
| [catalog-service](./catalog-service)   | Products  | `/products`, `/products/:id`    |

Every service also exposes `/healthz` and `/metrics`, wired by
`pkg/httpserver`.

## Why a top-level `services/` directory?

The cookbook has two natural homes for code that runs as a server:

| Location                         | When to use                                                                                                                                                                |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `recipes/<name>/cmd/server/`     | The server **is the lesson** (e.g. `flashsale` — three adapters behind one Go interface). Coupled to the recipe; lives under it.                                           |
| `services/<name>/cmd/server/`    | The server is **infrastructure under the lesson**. Reused across recipes that need a generic "users API" or "products API" workload to demonstrate something *around* it.  |

Decision rule, mirroring the `pkg/` rule in `docs/CONTRIBUTING.md`:

> A workload becomes a `services/<name>` only when **2+ recipes** can
> use it as-is. Default home for new server code is the recipe.
> Promoting a recipe-local server to `services/` later costs ~30
> minutes; demoting a leaky shared service costs hours.

## Universal-services contract

Every service in `services/` follows the same contract so recipes can
swap them in without surprises:

1. **Stateless by default** — in-memory data seeded at startup. If a
   recipe needs persistence, it adds a Postgres adapter behind a
   repository interface for that recipe only — without changing the
   service for everyone.
2. **Single port, configurable** — listens on `SERVICE_PORT` (default `8080`).
3. **Standard envelope endpoints** — `/healthz` (liveness) and
   `/metrics` (Prometheus) are wired by `pkg/httpserver`. Domain
   endpoints are JSON-only.
4. **Standard env vars** — `SERVICE_PORT`, `LOG_LEVEL`, `OTLP_ENDPOINT`.
   No service-specific env namespace; services don't carry
   recipe-specific config.
5. **Metric namespace** — `cookbook_<service-name-with-underscores>_*`
   (e.g. `cookbook_user_service_requests_total`). Distinct from recipe
   metrics (which use the recipe name as subsystem).
6. **Trace service-name** — `<service-name>` exactly as the directory
   (e.g. `user-service`). Recipes' Grafana dashboards use this to
   filter Tempo spans.
7. **Internal layout** mirrors the recipe layout (CONVENTIONS § 9), one
   level lighter:

   ```text
   services/<name>/
   ├── README.md
   ├── Dockerfile
   ├── cmd/server/main.go
   └── internal/
       ├── config/config.go
       ├── domain/<entity>.go        # data types + seed
       ├── handler/<entity>.go       # HTTP handlers + in-memory store
       ├── metrics/metrics.go
       └── routes/routes.go
   ```

   Drop `service/` and `repository/` until a recipe genuinely needs
   them; promoting later is cheap.

## Reusing services in a recipe

Recipes own the wiring, not the workloads. The K8s base manifests for
each service (Deployment, Service, ServiceMonitor) live at:

```text
deployments/k8s/services/<name>/
```

A recipe's kustomization references these bases:

```yaml
# deployments/k8s/recipes/<recipe>/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../services/user-service
  - ../../services/catalog-service
  # plus the recipe's own gateway/mesh/etc. resources
  - my-gateway.yaml
  - my-httproute.yaml
```

This way, `recipes/envoy-gateway`, `recipes/cilium-gateway`, and future
recipes share the same workloads and only differ in the *fabric* they
wrap them in. See `recipes/envoy-gateway/RECIPE.md` for the canonical
pattern.

## Adding a new universal service

1. **Confirm the gate**: 2+ recipes will use it as-is (or there's a
   credible plan they will). Otherwise, build it inside the recipe.
2. **Mirror the layout** of one of the existing services. Same internal
   directory shape, same env var names, same `/healthz`+`/metrics`
   contract, same metric namespace pattern.
3. **Write `services/<name>/README.md`** with a Contract table and
   Configuration table.
4. **Write `services/<name>/Dockerfile`** mirroring the existing ones
   (build context = repo root, multi-stage, `USER app`, `EXPOSE 8080`).
5. **Add K8s base manifests** at `deployments/k8s/services/<name>/`
   (Deployment, Service, ServiceMonitor, kustomization).
6. **Update this `services/README.md`** to list the new service in the
   "What's here" table.
