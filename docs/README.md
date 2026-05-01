# Distributed Cookbook — Docs

This directory holds the meta-docs for the cookbook itself. Per-recipe
docs live next to the recipe as `recipes/<name>/RECIPE.md`.

- **[ARCHITECTURE.md](./ARCHITECTURE.md)** — how recipes are laid out,
  how infrastructure is shared, and the design patterns each piece of
  code implements (with line-number pointers).
- **[CONVENTIONS.md](./CONVENTIONS.md)** — the hard rules: port
  allocation registry, metric namespace, schema naming, env var prefix,
  file naming. Contributors copy from here.
- **[DECISIONS.md](./DECISIONS.md)** — lightweight ADR log explaining
  deliberate non-choices (no Wire, no ORM, no Viper, …) with rationale.
  Read this before proposing "why aren't we using X?" changes.
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** — concrete checklist for
  adding a new recipe, including the `pkg/` vs recipe-local decision tree.
- **[ROADMAP.md](./ROADMAP.md)** — catalogue of planned future recipes,
  grouped by topic.

## Current recipes

| Recipe                                                | Status       | Concept                                                       | Infra                                       |
| ----------------------------------------------------- | ------------ | ------------------------------------------------------------- | ------------------------------------------- |
| [flashsale](../recipes/flashsale/RECIPE.md)           | ✅ available | atomic stock decrement under high concurrency                 | Postgres, Redis, LGTM (compose)             |
| [envoy-gateway](../recipes/envoy-gateway/RECIPE.md)   | ✅ available | K8s L7 ingress via Envoy Gateway (reference Gateway API impl) | kind (kindnet), Envoy Gateway, kube-prom-stack |
| [cilium-gateway](../recipes/cilium-gateway/RECIPE.md) | ✅ available | K8s L7 ingress via Cilium (CNI + Gateway API in one product)  | kind, Cilium, kube-prom-stack                |

`envoy-gateway` and `cilium-gateway` share the universal
`user-service` and `catalog-service` workloads from
[../services/README.md](../services/README.md) — they're the first two
recipes built on the services-reuse pattern. Each recipe runs its own
kind cluster (production-realistic single-gateway topology); compare
the two by running each in turn and overlaying their Grafana
dashboards.

## Reading order for new contributors

1. [ARCHITECTURE.md](./ARCHITECTURE.md) — what this repo is and why
2. [DECISIONS.md](./DECISIONS.md) — why it isn't the other thing
3. [CONVENTIONS.md](./CONVENTIONS.md) — the rules your PR must satisfy
4. [CONTRIBUTING.md](./CONTRIBUTING.md) — the checklist

## Top-level

- Top-level [README](../README.md) — getting started
