# catalog-service

Universal HTTP service used by cookbook recipes that need a "products"
or "inventory" workload (envoy-gateway, cilium-gateway, future
etc.). Stateless, in-memory.

## Contract

| Method | Path             | Purpose                                  |
| ------ | ---------------- | ---------------------------------------- |
| GET    | `/products`      | List all seeded products (10 records)    |
| GET    | `/products/:id`  | Fetch one product; 404 if unknown        |
| GET    | `/healthz`       | Liveness probe                           |
| GET    | `/metrics`       | Prometheus exposition                    |

## Configuration

| Env var          | Default            | Purpose                                      |
| ---------------- | ------------------ | -------------------------------------------- |
| `SERVICE_PORT`   | `8080`             | TCP listen port                              |
| `LOG_LEVEL`      | `info`             | `debug` / `info` / `warn` / `error`          |
| `OTLP_ENDPOINT`  | `localhost:4318`   | OTel collector OTLP HTTP endpoint            |

## Metrics

Per `docs/CONVENTIONS.md § 2` (services use `<service-name>` as the subsystem):

- `cookbook_catalog_service_requests_total{route, status}` — counter
- `cookbook_catalog_service_request_duration_seconds{route}` — histogram

## Running

### Locally (host binary)

```bash
go run ./services/catalog-service/cmd/server
curl http://localhost:8080/products | jq .
```

### Containerised

```bash
docker build -t cookbook/catalog-service:dev -f services/catalog-service/Dockerfile .
docker run -p 8080:8080 cookbook/catalog-service:dev
```

### In a recipe (K8s)

The service's K8s base manifests (`Deployment`, `Service`, `ServiceMonitor`)
live at `deployments/k8s/services/catalog-service/`. Recipes reference
this base from their own kustomization. See `recipes/envoy-gateway/` for an
example.
