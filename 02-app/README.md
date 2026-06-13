# 02 · Deploy the application

Goal: deploy the moving parts of the system — the **web app**, **MinIO** (object storage),
and the **mock-AI** service — and open the UI in your browser. This is a tour of the
everyday Kubernetes resources: Deployments, Services, Secrets, and Ingress.

Time: ~25 minutes.

> The controller comes later (step 04). For now we're just standing up the app the
> controller will eventually serve.

## 0. Make the images available

The manifests reference images like `ghcr.io/your-org/k8s-controller-workshop/web-app:latest`
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

## 6. Open the app

Open <http://localhost:8080> in your browser.

You should see the **AI Report Queue** UI with a form and an empty requests table. The
"connecting…" badge should switch to **live** — that's the browser holding an open
Server-Sent-Events stream backed by a Kubernetes *watch*.

> Try creating a report now. It will appear in the table as **Pending** and… stay there.
> That's expected — there's no controller yet. We fix that in step 04. First, let's
> understand the custom resource behind that row → step 03.

## Troubleshooting

- **Pod stuck in `ImagePullBackOff`** — the image isn't in the cluster. Run
  `./scripts/build-and-load.sh`, then `kubectl rollout restart deploy -n report-queue <name>`.
- **`http://localhost:8080` doesn't load** — confirm ingress-nginx is ready
  (`kubectl get pods -n ingress-nginx`) and that you created the cluster with
  `scripts/create-cluster.sh` (which sets up the port mapping).
- **MinIO not ready** — give it a few seconds; check `kubectl logs -n report-queue deploy/minio`.

---

Next: [`03-custom-resources/`](../03-custom-resources/) — the CRD behind the UI.
