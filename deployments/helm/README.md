# deployments/helm/ — chart release manifests + values

Each Helm chart we install (locally via Tilt, eventually in prod via
ArgoCD/Helmfile/raw helm) lives in its own directory under here:

```text
deployments/helm/<chart-name>/
├── release.yaml    # what to install: chart source, version, namespace, flags
└── values.yaml     # how to configure it: helm values, raw helm syntax
```

## Why two files

`release.yaml` is the **single source of truth** for chart pins. It is
read by:

| Consumer                    | Mechanism                          |
| --------------------------- | ---------------------------------- |
| Per-recipe `recipes/<name>/Tiltfile` (local dev) | Starlark `read_yaml()` + `helm_release()` helper |
| Per-recipe `recipes/<name>/scripts/kind-up.sh` (charts that must precede Tilt, like Cilium-as-CNI) | `yq` |
| Future prod tooling (ArgoCD ApplicationSet, Helmfile, raw helm CI) | Native YAML — same file |

Pinning chart versions inside the Tiltfile would couple them to local
dev only and force prod tooling to duplicate the pins (and drift).
This layout keeps dev and prod aligned by construction.

## `release.yaml` schema

```yaml
chart: <chart-spec>            # required. e.g. `cilium/cilium` or `oci://docker.io/...`
repo: <https-url>              # optional. only for non-OCI charts; URL of the helm repo
version: <semver>              # required. exact pin
namespace: <ns>                # required. where the chart installs
createNamespace: true|false    # optional. default false. add `--create-namespace` to helm
skipCRDs: true|false           # optional. default false. add `--skip-crds` to helm
                               #          (use when CRDs are managed elsewhere)
```

## `values.yaml`

Plain helm values for the chart. No envelope, no front-matter — just
what you'd pass to `helm install -f values.yaml`. Comments at the top
of each file explain why specific knobs are tuned the way they are.

## Adding a new chart

1. Create `deployments/helm/<name>/release.yaml` and `values.yaml`.
2. If installed by Tilt, add **one line** to the Tiltfile:
   `helm_release('<name>', labels=['…'])`. The `helm_release` helper
   reads `release.yaml`, registers the helm repo if needed, and calls
   `helm_resource()` with the right flags.
3. If installed by `kind-up.sh` (rare — only Cilium today, because it
   must be in place before any pod can network), read the values out
   via `yq` like the existing Cilium block.

No edits to docs, taskfile, or other shared files. Adding a new chart
is purely additive — see [docs/ARCHITECTURE.md § Add-files-don't-edit].

## Currently installed

| Chart                    | Version | Installed by                                       | Used by recipes              |
| ------------------------ | ------: | -------------------------------------------------- | ---------------------------- |
| cilium                   |  1.19.3 | `recipes/cilium-gateway/scripts/kind-up.sh`        | `cilium-gateway`             |
| envoy-gateway            |  v1.6.7 | `recipes/envoy-gateway/Tiltfile`                   | `envoy-gateway`              |
| kube-prometheus-stack    |  84.4.0 | per-recipe Tiltfiles                               | `envoy-gateway`, `cilium-gateway` |
