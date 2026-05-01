#!/usr/bin/env bash
# Bootstrap the kind cluster for the envoy-gateway recipe.
#
# Trivial compared to cilium-gateway: no special CNI, no CRD pre-install,
# no kube-proxy replacement. Tilt does everything else (helm-installs
# kube-prometheus-stack and envoy-gateway, applies kustomize, port-forwards).
#
# Tunables (override via env):
#   CLUSTER_NAME   default: cookbook-envoy-gateway

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-cookbook-envoy-gateway}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

kind create cluster \
    --config "${REPO_ROOT}/recipes/envoy-gateway/kind-cluster.yaml"

echo "kind cluster ${CLUSTER_NAME} ready. Now run: task tilt_up RECIPE=envoy-gateway"
