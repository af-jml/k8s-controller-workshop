# 04 · Deploy the controller

Goal: deploy the controller and watch it bring your custom resources to life. This is the
payoff for step 03: the `reports` **Bucket** you declared becomes a **real bucket in MinIO**.
This is the reconcile loop in action — desired state (your `Bucket` spec) driven into actual
state (a provisioned bucket).

Time: ~15 minutes.

## What the controller does

It's a single controller binary that reconciles **two** custom resources:

- **`Bucket`** → provisions the bucket directly in MinIO (create, set access policy, set
  quota) and cleans it up via a finalizer on deletion. *(This step.)*
- **`ReportRequest`** → creates a worker Job per request that renders a PDF. *(Step 05.)*

## 1. Register the ReportRequest type first

The controller watches **both** CRDs, and controller-runtime won't start if either type is
missing. You installed the `Bucket` CRD in step 03; install the `ReportRequest` CRD now so the
controller can come up cleanly:

```bash
kubectl apply -f manifests/crd.yaml
```

> Try skipping this and you'll see the controller pod crash-loop with a "no matches for kind
> ReportRequest" error — a good illustration that a controller needs its API types to exist.

## 2. Deploy the controller

```bash
kubectl apply -f 04-controller/controller.yaml
```

This file contains:

- a **ServiceAccount** for the controller,
- a **ClusterRole** + **ClusterRoleBinding** granting exactly the permissions the reconcile
  loops need: watch/update `Buckets` and `ReportRequests` (and their `status`), and
  create/observe Jobs,
- the controller **Deployment**. Note its env includes the real MinIO **credentials** — the
  Bucket controller talks to MinIO itself (unlike the report worker, which gets them passed
  down to its Job).

## 3. Watch the `reports` bucket become real

The moment the controller starts, it reconciles the `reports` Bucket that was sitting inert
since step 03. Watch the phase flip:

```bash
kubectl get buckets -n report-queue -w
# reports … Phase: (empty) → Ready  within a second or two
```

Now confirm the **real bucket exists** in MinIO (it didn't in step 03):

```bash
kubectl port-forward -n report-queue deploy/minio 9001:9001
# open http://localhost:9001 (minioadmin / minioadmin) → the `reports` bucket is now there
```

Inspect the controller-owned status and the endpoint it recorded:

```bash
kubectl get bucket reports -n report-queue -o jsonpath='{.status}' | jq
```

## 4. See the finalizer (external cleanup)

Owner references only garbage-collect **Kubernetes** objects. A MinIO bucket isn't one, so the
controller uses a **finalizer** to clean up external state on delete. Apply the two extra
sample buckets and compare their deletion policies:

```bash
kubectl apply -f 03-buckets/sample-bucket.yaml
kubectl get bucket analytics-exports -n report-queue \
  -o jsonpath='{.metadata.finalizers}'   # => ["storage.workshop.io/finalizer"]
```

- Delete `analytics-exports` (`deletionPolicy: Delete`): the controller empties and removes
  the real bucket *before* Kubernetes drops the object.
- Delete `public-assets` (`deletionPolicy: Retain`): the Kubernetes object goes, the MinIO
  bucket stays.

```bash
kubectl delete bucket analytics-exports public-assets -n report-queue
# watch the MinIO console: analytics-exports disappears, public-assets remains
```

> Don't delete the `reports` bucket — step 05 needs it. (It's `Retain` anyway, so its PDFs
> would survive, but keep the resource around.)

## Talking points

- The controller never talks to the web app directly — both just interact with the **API
  server**. That decoupling is the heart of the Kubernetes model.
- The controller writes **status**, the user writes **spec**. The reconcile loop closes the
  gap between them — here, all the way out into an external system.
- Two operator styles in one binary: **create Kubernetes objects** (Jobs, step 05) vs
  **provision external state** (MinIO buckets, this step).

## Troubleshooting

- **Controller crash-loops on startup** — a watched CRD is missing. Confirm both
  `kubectl get crd buckets.storage.workshop.io` and `…reportrequests.reports.workshop.io`
  exist (steps 03 and step 1 above).
- **Bucket stuck with no/empty phase** — check the controller logs
  (`kubectl logs -n report-queue deploy/report-controller -f`). RBAC errors point to the
  ClusterRole; connection errors point to `MINIO_ENDPOINT/MINIO_ACCESS_KEY/MINIO_SECRET_KEY`.
- **Bucket `Failed`** — read `.status.message`. Usually MinIO isn't ready yet; it will
  reconcile again shortly.

## Assessment checkpoint

- Can you show the `reports` bucket as a real bucket in MinIO, and point to the controller
  log line that created it?
- Can you explain why a finalizer is needed for a Bucket but not for a worker Job?

---

Next: [`05-reports/`](../05-reports/) — drive the report pipeline into the bucket you just provisioned.
