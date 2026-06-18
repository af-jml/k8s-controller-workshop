# 05 · Reports — the pipeline in action

Goal: create `ReportRequest` resources and watch the controller turn each one into a real PDF,
stored in the **`reports` bucket you provisioned in step 03**. This is the operator pattern
end to end: spec in, Job spawned, status out, PDF in object storage.

Time: ~20 minutes.

## What the controller does for a ReportRequest

The `ReportRequest` CRD was registered in step 04 (so the controller could start), and the
controller is already running. For each `ReportRequest`, its reconcile loop:

1. Sees the new object (via a **watch**).
2. Creates a worker **Job**, owned by the request (an *owner reference*).
3. Sets `status.phase = Processing` and records the Job name.
4. Watches the Job. On success → `phase = Completed` and the PDF's object key; on failure →
   `phase = Failed`.

The worker Job calls the mock-AI service, renders a PDF, and uploads it to the `reports`
bucket. It does **not** create that bucket — it expects the one from step 03 to exist (try the
failure drill at the bottom to see what happens otherwise).

## 1. Create a ReportRequest — two ways

### a) With kubectl

```bash
kubectl apply -f 05-reports/sample-reportrequest.yaml
kubectl get reportrequests -n report-queue
```

You'll see your object listed with custom columns (Phase, Title, Job, Age) defined by the CRD.

### b) From the web UI

Open <http://localhost:8080>, stay on the **Reports** tab, and submit the form. The new row
appears instantly (the app is *watching* ReportRequests).

## 2. Watch it reconcile

```bash
kubectl get reportrequests -n report-queue -w
```

The phase moves (empty) → **Processing** → **Completed** within a few seconds. In parallel,
watch the worker Job appear and finish:

```bash
kubectl get jobs,pods -n report-queue -w
```

Or — much nicer — watch it all in **k9s**: `:reportrequests`, `:jobs`, `:pods`. You'll
literally see a worker pod spin up, run, and complete.

## 3. Watch the UI update live

The row for your request updates **without a refresh**: Pending → Processing → Completed, and a
**Download** link appears. Click it to view the generated PDF — the web app streams it from the
`reports` bucket in MinIO.

## 4. Owner references & garbage collection

Each Job is *owned* by its ReportRequest, so deleting the request cleans up its Job
automatically:

```bash
kubectl get job -n report-queue -o yaml | grep -A6 ownerReferences
kubectl delete reportrequest <name> -n report-queue
kubectl get jobs -n report-queue   # the owned Job is gone too
```

> Contrast with the Bucket from step 04: a Job is a Kubernetes object, so owner references
> garbage-collect it for free. A MinIO bucket isn't, which is why the Bucket controller needs
> a **finalizer**. Same goal, two different mechanisms.

## 5. Look under the hood

```bash
# Controller logs — see the reconcile decisions
kubectl logs -n report-queue deploy/report-controller -f

# A finished request's full object, including controller-owned status
kubectl get reportrequest <name> -n report-queue -o yaml

# The PDF object now living in the reports bucket
kubectl port-forward -n report-queue deploy/minio 9001:9001
# open http://localhost:9001 → bucket "reports" → your PDF
```

## Failure drill: the missing bucket

The worker depends on the `reports` bucket existing. To see the dependency fail loudly,
imagine step 03 was skipped: a report's worker exits with

```
bucket "reports" does not exist — create it first via a Bucket resource (workshop step 03):
kubectl apply -f 03-buckets/reports-bucket.yaml
```

and the request goes **Failed**. That's the point of provisioning the bucket explicitly: the
dependency is visible, not hidden behind silent auto-creation.

## Talking points

- The controller never talks to the web app directly — both just interact with the **API
  server**. That decoupling is the heart of the Kubernetes model.
- The controller writes **status**, the user writes **spec**. The reconcile loop closes the gap.
- RBAC is explicit and minimal: the controller can create Jobs; the web app cannot.

## Assessment checkpoint

- Can you map one request from creation to Job completion using commands and logs?
- Can you show the resulting PDF in the `reports` bucket?
- Can you explain why the Job is garbage-collected by an owner reference but the bucket needs a
  finalizer?

---

Next: [`06-wrap-up/`](../06-wrap-up/) — recap & challenges.
