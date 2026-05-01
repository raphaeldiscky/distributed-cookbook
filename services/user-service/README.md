# user-service

Universal HTTP service used by cookbook recipes that need a "users" workload
(envoy-gateway, cilium-gateway, future mesh/tracing recipes). Stateless, in-memory.

## Contract

| Method | Path           | Purpose                                  |
| ------ | -------------- | ---------------------------------------- |
| GET    | `/users`       | List all seeded users (10 records)       |
| GET    | `/users/:id`   | Fetch one user; 404 if unknown           |
| GET    | `/healthz`     | Liveness probe                           |
| GET    | `/metrics`     | Prometheus exposition                    |

## Configuration

| Env var          | Default            | Purpose                                      |
| ---------------- | ------------------ | -------------------------------------------- |
| `SERVICE_PORT`   | `8080`             | TCP listen port                              |
| `LOG_LEVEL`      | `info`             | `debug` / `info` / `warn` / `error`          |
| `OTLP_ENDPOINT`  | `localhost:4318`   | OTel collector OTLP HTTP endpoint            |

## Metrics

Per `docs/CONVENTIONS.md § 2` (services use `<service-name>` as the subsystem):

- `cookbook_user_service_requests_total{route, status}` — counter
- `cookbook_user_service_request_duration_seconds{route}` — histogram

## Running

### Locally (host binary)

```bash
go run ./services/user-service/cmd/server
curl http://localhost:8080/users | jq .
```

### Containerised

```bash
docker build -t cookbook/user-service:dev -f services/user-service/Dockerfile .
docker run -p 8080:8080 cookbook/user-service:dev
```

### In a recipe (K8s)

The service's K8s base manifests (`Deployment`, `Service`, `ServiceMonitor`)
live at `deployments/k8s/services/user-service/`. Recipes reference this
base from their own kustomization. See `recipes/envoy-gateway/` for an example.
