#!/usr/bin/env bash
# Bootstrap the kind cluster for the cilium-gateway recipe.
#
# Steps:
#   1. Create kind cluster (kindnet + kube-proxy disabled — Cilium
#      replaces both).
#   2. Pre-load the Cilium image into the kind node's containerd
#      so subsequent `task kind_up` runs skip the upstream pull.
#   3. Apply Gateway API CRDs (experimental channel — Cilium needs
#      TLSRoute v1alpha2 at agent startup). Server-side apply because
#      the experimental HTTPRoute schema overflows the
#      last-applied-configuration annotation budget.
#   4. helm-install Cilium with kubeProxyReplacement=true and
#      gatewayAPI.enabled=true. Cilium becomes the CNI and registers
#      the `cilium` GatewayClass.
#
# Tunables (override via env):
#   CLUSTER_NAME          default: cookbook-cilium-gateway
#   CILIUM_VERSION        default: from deployments/helm/cilium/release.yaml
#   GATEWAY_API_VERSION   default: v1.5.1

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-cookbook-cilium-gateway}"
GATEWAY_API_VERSION="${GATEWAY_API_VERSION:-v1.5.1}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CILIUM_RELEASE="${REPO_ROOT}/deployments/helm/cilium/release.yaml"

CILIUM_VERSION="${CILIUM_VERSION:-$(yq '.version' "${CILIUM_RELEASE}")}"
CILIUM_REPO="$(yq '.repo' "${CILIUM_RELEASE}")"
CILIUM_NAMESPACE="$(yq '.namespace' "${CILIUM_RELEASE}")"
CILIUM_CHART="$(yq '.chart' "${CILIUM_RELEASE}")"

kind create cluster \
    --config "${REPO_ROOT}/recipes/cilium-gateway/kind-cluster.yaml"

echo "Pre-loading cilium image into kind cluster..."
docker pull "quay.io/cilium/cilium:v${CILIUM_VERSION}"
kind load docker-image "quay.io/cilium/cilium:v${CILIUM_VERSION}" \
    --name "${CLUSTER_NAME}"

echo "Installing Gateway API CRDs (${GATEWAY_API_VERSION}, experimental channel)..."
kubectl apply --server-side -f \
    "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/experimental-install.yaml"

helm repo add "$(echo "${CILIUM_CHART}" | cut -d/ -f1)" "${CILIUM_REPO}" >/dev/null || true
helm repo update >/dev/null

# Cilium installs with:
#   gatewayAPI.enabled=true              — register the `cilium` GatewayClass
#   gatewayAPI.hostNetwork.enabled=true  — bind listener directly on the
#                                          kind node's network namespace
#                                          (no LB Service, no NodePort).
#                                          Per Cilium docs, this is the
#                                          recommended mode for kind dev
#                                          clusters since kind has no
#                                          LoadBalancer controller.
# These overrides keep the shared deployments/helm/cilium/values.yaml
# recipe-agnostic — see that file for the rationale.
helm upgrade --install cilium "${CILIUM_CHART}" \
    --namespace "${CILIUM_NAMESPACE}" \
    --version "${CILIUM_VERSION}" \
    --values "${REPO_ROOT}/deployments/helm/cilium/values.yaml" \
    --set gatewayAPI.enabled=true \
    --set gatewayAPI.hostNetwork.enabled=true \
    --set "k8sServiceHost=${CLUSTER_NAME}-control-plane" \
    --set k8sServicePort=6443 \
    --wait

kubectl --context "kind-${CLUSTER_NAME}" -n "${CILIUM_NAMESPACE}" \
    rollout status ds/cilium --timeout=300s

echo "kind cluster ${CLUSTER_NAME} + Cilium gateway ready."
echo "Now run: task tilt_up RECIPE=cilium-gateway"
