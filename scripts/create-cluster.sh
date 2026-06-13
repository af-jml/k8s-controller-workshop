#!/usr/bin/env bash
# Create the local kind cluster and install ingress-nginx.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/env.sh"

echo "==> Creating kind cluster '${CLUSTER_NAME}'"
if kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  echo "    Cluster '${CLUSTER_NAME}' already exists, skipping creation."
else
  kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml"
fi

echo "==> Setting kubectl context to 'kind-${CLUSTER_NAME}'"
kubectl config use-context "kind-${CLUSTER_NAME}"

echo "==> Installing ingress-nginx"
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

echo "==> Waiting for the ingress-nginx controller to be created"
kubectl -n ingress-nginx rollout status deployment/ingress-nginx-controller --timeout=180s

echo "==> Waiting for ingress-nginx to be ready (this can take a minute)"
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s

echo ""
echo "Cluster '${CLUSTER_NAME}' is ready."
echo "Context: kind-${CLUSTER_NAME}"
echo "Try:     kubectl get nodes"
echo "Or open k9s and explore."
