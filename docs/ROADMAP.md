# Roadmap — Future Recipes

A curated catalogue of distributed-systems concepts to cover. Recipes are
grouped by topic; each row lists the infra atoms it would pull in and the
primary deployment mode (docker-compose vs kubernetes).

Status legend: ✅ shipped · 🚧 in progress · — planned

## Concurrency & Contention

| Recipe           | Status | Infra           | Teaches                                               |
| ---------------- | ------ | --------------- | ----------------------------------------------------- |
| `flashsale`      | ✅     | postgres, redis | atomic stock decrement, naive vs pg_cond vs redis_lua |
| `idempotency`    | —      | postgres, redis | idempotency keys, dedup windows, retries              |
| `ratelimiting`   | —      | redis           | token bucket, sliding window, leaky bucket            |
| `circuitbreaker` | —      | postgres        | Hystrix-style breaker, half-open probing              |

## Data Consistency

| Recipe          | Status | Infra                     | Teaches                                                           |
| --------------- | ------ | ------------------------- | ----------------------------------------------------------------- |
| `outbox`        | —      | postgres, kafka           | transactional outbox, fixes the dual-write problem from flashsale |
| `cdc`           | —      | postgres, kafka, debezium | change data capture from Postgres WAL                             |
| `saga`          | —      | postgres, kafka           | orchestration vs. choreography, compensations                     |
| `eventsourcing` | —      | postgres, kafka           | event log as source of truth, CQRS read model                     |
| `exactly-once`  | —      | kafka                     | Kafka transactions, idempotent producers                          |

## Coordination

| Recipe               | Status | Infra                  | Teaches                                  |
| -------------------- | ------ | ---------------------- | ---------------------------------------- |
| `leader-election`    | —      | etcd (or redis)        | lease-based leadership, fencing tokens   |
| `distributed-locks`  | —      | redis, etcd, zookeeper | Redlock critique, etcd/zk alternatives   |
| `consistent-hashing` | —      | in-memory              | rendezvous hashing, partition assignment |

## Networking & Service Routing

| Recipe                | Status | Infra                | Teaches                                        |
| --------------------- | ------ | -------------------- | ---------------------------------------------- |
| `microservices`       | —      | 2–3 Go apps          | synchronous vs. async comms, error propagation |
| `api-gateway`         | —      | envoy, kong, traefik | routing, auth, rate limit at the edge          |
| `service-mesh-istio`  | —      | kind + istio         | sidecar mesh, mTLS, traffic shifting           |
| `service-mesh-cilium` | —      | kind + cilium        | eBPF sidecarless mesh, Hubble observability    |
| `grpc-vs-rest`        | —      | envoy                | streaming, multiplexing, interceptors          |

## Kubernetes-native

| Recipe                  | Status | Infra              | Teaches                                    |
| ----------------------- | ------ | ------------------ | ------------------------------------------ |
| `kubernetes-basics`     | —      | kind               | deployments, services, configmaps, secrets |
| `operators-controllers` | —      | kind + kubebuilder | CRDs, reconcile loops                      |
| `autoscaling`           | —      | kind + KEDA        | HPA, VPA, event-driven scaling             |
| `helm-kustomize`        | —      | kind               | templating + overlays                      |

## Resilience

| Recipe               | Status | Infra             | Teaches                                           |
| -------------------- | ------ | ----------------- | ------------------------------------------------- |
| `retries-backoff`    | —      | postgres          | exponential + jitter + budgets                    |
| `bulkheads`          | —      | postgres          | pool isolation, noisy-neighbor isolation          |
| `load-shedding`      | —      | prometheus        | adaptive concurrency (Netflix concurrency-limits) |
| `timeouts-deadlines` | —      | postgres          | context propagation across services               |
| `chaos-engineering`  | —      | kind + chaos-mesh | fault injection, hypothesis-driven testing        |

## Observability

| Recipe                     | Status | Infra                   | Teaches                                                         |
| -------------------------- | ------ | ----------------------- | --------------------------------------------------------------- |
| `distributed-tracing-deep` | —      | existing otel stack     | context propagation edge cases, baggage                         |
| `red-use-methods`          | —      | prometheus              | RED (rate/errors/duration), USE (utilization/saturation/errors) |
| `profiling`                | —      | pprof + parca/pyroscope | continuous CPU/heap profiling                                   |

## Advanced Algorithms

| Recipe          | Status | Infra         | Teaches                             |
| --------------- | ------ | ------------- | ----------------------------------- |
| `raft`          | —      | 3 Go binaries | consensus, log replication          |
| `gossip`        | —      | 5 Go binaries | SWIM, anti-entropy                  |
| `crdts`         | —      | 3 Go binaries | G-counter, PN-counter, LWW-register |
| `vector-clocks` | —      | in-memory     | happens-before, causal ordering     |

---

## Suggested learning path

Rough ordering if you want to work through recipes in sequence:

1. **Concurrency & Contention** → start here; builds intuition for atomicity
2. **Data Consistency** → once you understand atomicity, tackle cross-system consistency
3. **Resilience** → how systems survive failures
4. **Coordination** → how multiple nodes agree
5. **Networking & Service Routing** → how traffic flows between services
6. **Kubernetes-native** → the runtime substrate
7. **Observability** → skills to debug everything above
8. **Advanced Algorithms** → the theoretical underpinnings

Not a strict order — each recipe stands alone.
