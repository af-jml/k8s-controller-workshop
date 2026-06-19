# 02 · Deploy the application

Goal: deploy the moving parts of the system — the **web app**, **MinIO** (object storage),
and the **mock-AI** service — and open the UI in your browser. This is a tour of the
everyday Kubernetes resources: Deployments, Services, Secrets, and Ingress.

Time: ~15 minutes.

## Learning objectives

- Deploy a multi-component app stack using standard Kubernetes resources.
- Understand the role of each component in the upcoming controller workflow.
- Observe RBAC boundaries between a user-facing app and a controller.
- Validate that watch-based live updates are working before the controller exists.

> The controller comes later (step 04). For now we're just standing up the app the
> controller will eventually serve.

## 0. Make the images available

The manifests reference images like `ghcr.io/appsfactory/k8s-controller-workshop/web-app:latest`
with `imagePullPolicy: IfNotPresent`. Choose one path:

- **Build & load locally (offline-friendly):**

  ```bash
  ./scripts/build-and-load.sh
  ```

  This builds every image and loads it straight into the kind node, so Kubernetes finds it
  without pulling from a registry.

- **Use prebuilt images from a registry:** set `IMAGE_PREFIX` in
  [`scripts/env.sh`](../scripts/env.sh) to your published images and make sure the manifests
  reference the same names. (The defaults are placeholders.)

## 1. Create the namespace

Everything lives in the `report-queue` namespace.

```bash
kubectl apply -f manifests/namespace.yaml
```

## 2. Deploy MinIO (object storage)

MinIO is an S3-compatible store. The worker will upload PDFs here; the web app streams them
back to the browser.

```bash
kubectl apply -f 02-app/minio.yaml
```

What's in this file:

- a **Secret** holding demo credentials (`minio-credentials`),
- a **Deployment** running MinIO,
- a **Service** exposing it inside the cluster.

## 3. Deploy the mock-AI service

```bash
kubectl apply -f 02-app/mock-ai.yaml
```

A tiny HTTP service that returns a deterministic markdown report — no API keys, no internet.

## 4. Deploy the web app

```bash
kubectl apply -f 02-app/web-app.yaml
```

This file contains the most to discuss:

- a **ServiceAccount** + **Role** + **RoleBinding** giving the app permission to
  `get/list/watch/create` ReportRequests — and *nothing else*. It cannot create Jobs; only
  the controller will.
- a **Deployment** running the app,
- a **Service**, and
- an **Ingress** that routes `http://localhost:8080/` to the app.

## 5. Wait for everything to be ready

```bash
kubectl get pods -n report-queue
# all should reach Running / READY 1/1
```

Or watch it live in k9s (`:pods`, namespace `report-queue`).

## While pods start (facilitator prompts)

Use this wait time to ask:

- Why can the web app create `ReportRequest` objects but not create Jobs?
- What changes once the controller is deployed in step 04?
- Why is the app already "live" even though report processing is not?

## 6. Open the app

Open <http://localhost:8080> in your browser.

You should see the **Operator Console** UI. It has two tabs — **1 · Buckets** and
**2 · Reports** (in that order) — each backed by its own Kubernetes custom resource. The
"connecting…" badge should switch to **live**: that's the browser holding an open
Server-Sent-Events stream backed by a Kubernetes *watch*.

> Nothing will actually process yet — there's no controller running. That's the whole point of
> the next steps. First we'll create our very first custom resource — the storage **bucket**
> the reports will need — and watch it sit inert until a controller brings it to life → step 03.

## Troubleshooting

- **Pod stuck in `ImagePullBackOff`** — the image isn't in the cluster. Run
  `./scripts/build-and-load.sh`, then `kubectl rollout restart deploy -n report-queue <name>`.
- **`http://localhost:8080` doesn't load** — confirm ingress-nginx is ready
  (`kubectl get pods -n ingress-nginx`) and that you created the cluster with
  `scripts/create-cluster.sh` (which sets up the port mapping).
- **MinIO not ready** — give it a few seconds; check `kubectl logs -n report-queue deploy/minio`.

---

Next: [`03-buckets/`](../03-buckets/) — your first custom resource (the `reports` bucket).
