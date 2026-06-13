# 01 · Setup

Goal: get a local Kubernetes cluster running and the tools to explore it. By the end you'll
have a single-node cluster created with **kind**, reachable via **kubectl**, and visible in
**k9s**.

Time: ~20 minutes.

## 1. Install the tools

You need four tools. Install whichever you don't already have.

### Docker Desktop

kind runs Kubernetes nodes as Docker containers, so you need a container runtime.

- macOS / Windows: install **Docker Desktop** → <https://www.docker.com/products/docker-desktop/>
- Linux: install Docker Engine → <https://docs.docker.com/engine/install/>

Verify:

```bash
docker run --rm hello-world
```

### kubectl — the Kubernetes CLI

| OS | Command |
| --- | --- |
| macOS | `brew install kubectl` |
| Windows | `choco install kubernetes-cli` or `winget install -e --id Kubernetes.kubectl` |
| Linux | See <https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/> |

Verify: `kubectl version --client`

### kind — Kubernetes IN Docker

| OS | Command |
| --- | --- |
| macOS | `brew install kind` |
| Windows | `choco install kind` or `winget install Kubernetes.kind` |
| Linux | `curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind` |

Verify: `kind version`

### k9s — a terminal UI for clusters

k9s gives you a live, navigable view of everything in the cluster. New to most people and
genuinely delightful.

| OS | Command |
| --- | --- |
| macOS | `brew install k9s` |
| Windows | `choco install k9s` or `winget install k9s` |
| Linux | See <https://k9scli.io/topics/install/> |

Verify: `k9s version`

## 2. Create the cluster

From the **repository root**:

```bash
./scripts/create-cluster.sh
```

This script:

1. Creates a kind cluster named `report-queue` using [`scripts/kind-config.yaml`](../scripts/kind-config.yaml).
   The config maps the cluster's ingress ports to `localhost:8080` so you can open the app
   in your browser later.
2. Sets your `kubectl` context to `kind-report-queue` (this updates your kubeconfig).
3. Installs **ingress-nginx** and waits for it to be ready.

> Creating the cluster automatically adds it to your kubeconfig (`~/.kube/config`) and
> switches your current context to it. Check with `kubectl config current-context`.

## 3. Verify

```bash
kubectl get nodes
# NAME                          STATUS   ROLES           AGE   VERSION
# report-queue-control-plane    Ready    control-plane   1m    v1.xx.x

kubectl get pods -A
```

## 4. Take k9s for a spin

```bash
k9s
```

Things to try:

- Type `:pods` then Enter to list pods. Add `0` (zero) to see all namespaces.
- Type `:namespaces`, `:deployments`, `:services` to explore.
- Press `?` for help, `Esc` to back out, `:quit` to exit.

Keep k9s open in a second terminal throughout the workshop — you'll watch resources appear
and change in real time.

## Troubleshooting

- **`docker` not running** — start Docker Desktop and wait for it to be ready.
- **Cluster already exists** — the script is safe to re-run; it skips creation. To start
  fresh: `./scripts/cleanup.sh` then re-run.
- **ingress-nginx wait times out** — re-run the wait:
  `kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=180s`

---

Next: [`02-app/`](../02-app/) — deploy the application.
