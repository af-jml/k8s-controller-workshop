# Shared environment for workshop scripts.
# Source this file or rely on the scripts sourcing it.

# Cluster name used by kind and kubectl context (kind-<name>).
export CLUSTER_NAME="${CLUSTER_NAME:-report-queue}"

# Kubernetes namespace everything is deployed into.
export NAMESPACE="${NAMESPACE:-report-queue}"

# Registry/prefix for prebuilt images. Override to use your own published images.
# Images are referenced as ${IMAGE_PREFIX}/<name>:${IMAGE_TAG}
export IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/your-org/k8s-controller-workshop}"
export IMAGE_TAG="${IMAGE_TAG:-latest}"

# Local image names (used when building + loading into kind).
export WEB_APP_IMAGE="${IMAGE_PREFIX}/web-app:${IMAGE_TAG}"
export CONTROLLER_IMAGE="${IMAGE_PREFIX}/controller:${IMAGE_TAG}"
export WORKER_IMAGE="${IMAGE_PREFIX}/worker:${IMAGE_TAG}"
export MOCK_AI_IMAGE="${IMAGE_PREFIX}/mock-ai:${IMAGE_TAG}"
