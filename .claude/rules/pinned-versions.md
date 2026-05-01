# Rule: Where pinned versions live

Pinned chart/tool/CRD versions belong in **`deployments/`** — the
deployment-config-of-record that any tool (Tilt, ArgoCD, Helmfile,
raw helm CI, bash scripts) reads. They do **not** belong inside
orchestration code (Tiltfile, taskfile, scripts) where prod tooling
can't see them and version drift becomes inevitable.

## ✅ DO

- For helm charts: put the pin in
  `deployments/helm/<chart>/release.yaml` alongside the chart's
  `values.yaml`. Schema lives in
  [`deployments/helm/README.md`](../../deployments/helm/README.md).
- Read pins with `read_yaml()` in the Tiltfile (via the
  `helm_release()` helper) and with `yq` in bash scripts.
- Add **one** chart = **one** new directory `deployments/helm/<name>/`
  with two files (`release.yaml` + `values.yaml`) and **one** line in
  the Tiltfile (or kind-up.sh for cluster-CNI charts like Cilium).
- Keep an env-var override (`CILIUM_VERSION="${CILIUM_VERSION:-$(yq …)}"`)
  for one-off bumps without editing the pin file.
- For non-helm pins that genuinely don't fit (e.g. Gateway API CRDs
  installed via `kubectl apply -f <upstream-yaml>`), keep the pin as an
  env-overridable constant in the script that uses it AND document why
  it doesn't live under `deployments/helm/`.

## ❌ DON'T

- **Don't** hardcode `--version=v1.6.7` in the Tiltfile. The Tiltfile
  is local-dev orchestration; production tooling can't read it.
- **Don't** define `CHART_VERSION="..."` defaults inside `scripts/`
  shell variables when a `release.yaml` already exists. Read from the
  YAML.
- **Don't** duplicate a pin in two places "for safety". Two pins drift.
- **Don't** put pins in `taskfile.yml` either — same reason as Tiltfile.
- **Don't** use `:latest` or omit `version:`. Pinning is non-negotiable
  per [`docs/DECISIONS.md § 8`](../../docs/DECISIONS.md).

## Why

See [`docs/DECISIONS.md § 15`](../../docs/DECISIONS.md). Short version:
"single source of truth that prod can read" beats "convenient where
the script is".

## Example

```yaml
# deployments/helm/envoy-gateway/release.yaml
chart: oci://docker.io/envoyproxy/gateway-helm
version: v1.6.7
namespace: envoy-gateway-system
createNamespace: true
skipCRDs: true
```

```python
# Tiltfile — one line, pin lives elsewhere
helm_release('envoy-gateway', labels=['gateways'])
```

```bash
# scripts/kind-up.sh — read with yq, env-overridable
CILIUM_VERSION="${CILIUM_VERSION:-$(yq '.version' "${CILIUM_RELEASE}")}"
```
