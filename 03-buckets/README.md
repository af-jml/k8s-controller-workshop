# 03 · Buckets — your first custom resource

Goal: teach the cluster a brand new resource type with a **CustomResourceDefinition (CRD)**,
create the **`reports` bucket** your report pipeline will need later, and meet the key lesson
of the whole workshop:

> A CRD on its own is just **data**. Nothing happens to it until a **controller** acts.

Time: ~10 minutes.

## Why buckets first?

The report pipeline (step 05) writes its PDFs into a MinIO bucket called `reports`. Rather
than have something create that bucket implicitly, you'll provision it **explicitly**, as a
Kubernetes object — a `Bucket` custom resource. This is the "Kubernetes as a cloud API"
pattern: you declare a domain object ("I want object storage, private, capped at 200Mi") and
a controller makes it real inside MinIO. Crossplane and the AWS/GCP/Azure operators work
exactly this way; here we build a tiny local cloud.

## Learning objectives

- Apply and inspect a CRD-backed custom resource type.
- See that a CRD gives you schema, validation, `kubectl` support and watches *for free*.
- Observe that **nothing is provisioned** until a controller reconciles the resource.
- Create the `reports` bucket that step 05 depends on.

## 1. Apply the Bucket CRD

```bash
kubectl apply -f manifests/bucket-crd.yaml
```

This registers a new resource type, `Bucket`, in the API group `storage.workshop.io`. The API
server now serves it like any built-in resource:

```bash
kubectl get crds | grep storage.workshop.io
kubectl explain bucket.spec
```

`kubectl explain` works because the CRD ships an OpenAPI schema — the same schema the API
server uses to **validate** objects you submit.

## 2. Create the `reports` bucket

```bash
kubectl apply -f 03-buckets/reports-bucket.yaml
kubectl get buckets -n report-queue
```

You'll see your object listed with custom columns (Bucket, Policy, Quota, Phase, Age) defined
by the CRD. Note the **Phase is empty** — no status has been set.

You can also use the **Buckets** tab in the web UI (<http://localhost:8080>) to create buckets
and watch them live — the app is *watching* `Bucket` objects, just like it watches reports.

## 3. Observe: nothing is provisioning it

```bash
kubectl get bucket reports -n report-queue -o yaml | less
```

The object has your `spec`, but no meaningful `status`. And crucially — **there is no real
bucket in MinIO yet**. Check the MinIO console to prove it:

```bash
# Port-forward the MinIO console, then open http://localhost:9001 (minioadmin / minioadmin)
kubectl port-forward -n report-queue deploy/minio 9001:9001
```

No `reports` bucket. This is the whole point: the API happily stores and validates `Bucket`
objects, but they are **inert**. There is no behaviour attached to this new noun yet — we add
it in step 04.

## 4. Inspect the resource in k9s

In k9s, type `:buckets` (or `:bk`, the CRD's short name) to list them. They're first-class
citizens now — selectable, describable, deletable — just like Pods.

## Talking points

- A CRD adds a **noun** to the API. A controller adds the **verb**.
- Validation, `kubectl` integration, RBAC and watches all work *for free* once the CRD
  exists — that's a lot of leverage.
- The `status` subresource is separate from `spec`: users own `spec`, controllers own
  `status`. We'll see the controller write `status` next.
- The `reports` bucket you just declared is a hard dependency for step 05 — the report worker
  will refuse to run without it.

---

Next: [`04-controller/`](../04-controller/) — deploy the controller and watch the bucket become real.
