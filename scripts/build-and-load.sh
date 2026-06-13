#!/usr/bin/env bash
# Build all workshop images and load them into the kind cluster.
# Use this for the offline path (no registry required).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/env.sh"

build_and_load() {
  local name="$1" context="$2" image="$3"
  echo "==> Building ${image}"
  docker build -t "${image}" "${context}"
  echo "==> Loading ${image} into kind cluster '${CLUSTER_NAME}'"
  kind load docker-image "${image}" --name "${CLUSTER_NAME}"
}

build_and_load "mock-ai"    "${ROOT_DIR}/src/mock-ai"    "${MOCK_AI_IMAGE}"
build_and_load "worker"     "${ROOT_DIR}/src/worker"     "${WORKER_IMAGE}"
build_and_load "controller" "${ROOT_DIR}/src/controller" "${CONTROLLER_IMAGE}"
build_and_load "web-app"    "${ROOT_DIR}/src/web-app"    "${WEB_APP_IMAGE}"

echo ""
echo "All images built and loaded into kind cluster '${CLUSTER_NAME}'."
