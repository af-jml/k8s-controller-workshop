# 04 · Deploy the controller

Goal: deploy the controller and watch it bring your `ReportRequest` objects to life. This is
the payoff: the reconcile loop, Jobs spawned per request, status updates, and live UI changes.

Time: ~25 minutes.

## What the controller does

For each `ReportRequest`, the controller's reconcile loop:

1. Sees the new object (via a **watch**).
2. Creates a worker **Job**, owned by the request (an *owner reference*).
3. Sets `status.phase = Processing` and records the Job name.
4. Watches the Job. When it succeeds, sets `phase = Completed` and the PDF's object key;
   if it fails, `phase = Failed`.

The worker Job calls the mock-AI service, renders a PDF, and uploads it to MinIO.

## 1. Deploy the controller

```bash
kubectl apply -f 04-controller/controller.yaml
```

This file contains:

- a **ServiceAccount** for the controller,
- a **ClusterRole** + **ClusterRoleBinding** granting exactly the permissions the reconcile
  loop needs: watch/update ReportRequests and their `status`, and create/observe Jobs,
- the controller **Deployment**, configured via env vars (worker image, MinIO, mock-AI).

## 2. Watch it reconcile the request you already created

You created a `ReportRequest` in step 03 that has been sitting at Pending. The moment the
controller starts, it reconciles it. Watch it happen:

```bash
kubectl get reportrequests -n report-queue -w
```

You should see the phase move: (empty) → **Processing** → **Completed** within a few seconds.

In parallel, watch the Job appear:

```bash
kubectl get jobs,pods -n report-queue -w
```

Or — much nicer — watch all of it in **k9s**: `:reportrequests`, `:jobs`, `:pods`. You'll
literally see a worker pod spin up, run, and complete.

## 3. Watch the UI update live

Open <http://localhost:8080>. The row for your request updates **without a refresh**:
Pending → Processing → Completed, and a **Download** link appears. Click it to view the
generated PDF (the web app streams it from MinIO).

## 4. Create more — feel the loop

Submit a few requests from the UI. Each one triggers the same cycle. Watch the Jobs come and
go in k9s. This is the operator pattern in action.

## 5. Look under the hood

```bash
# Controller logs — see the reconcile decisions
kubectl logs -n report-queue deploy/report-controller -f

# A finished request's full object, including controller-owned status
kubectl get reportrequest <name> -n report-queue -o yaml

# The Job's owner reference points back at the ReportRequest
kubectl get job -n report-queue -o yaml | grep -A6 ownerReferences
```

### Owner references & garbage collection

Because each Job is *owned* by its ReportRequest, deleting the request cleans up its Job
automatically:

```bash
kubectl delete reportrequest <name> -n report-queue
kubectl get jobs -n report-queue   # the owned Job is gone too
```

## Talking points

- The controller never talks to the web app directly — both just interact with the **API
  server**. That decoupling is the heart of the Kubernetes model.
- The controller writes **status**, the user writes **spec**. The reconcile loop closes the
  gap between them.
- RBAC is explicit and minimal: the controller can create Jobs; the web app cannot.

## Troubleshooting

- **Phase never changes** — check the controller logs
  (`kubectl logs -n report-queue deploy/report-controller`). Permission errors point to RBAC;
  image errors point to a missing worker image (`./scripts/build-and-load.sh`).
- **Job fails** — inspect the worker pod:
  `kubectl logs -n report-queue job/report-<name>`. Common causes: MinIO not ready, or the
  mock-AI service not reachable.
- **No Download link** — the request must be `Completed`. Confirm the PDF exists in MinIO
  (the worker logs print the uploaded object key).

---

Next: [`05-wrap-up/`](../05-wrap-up/) — recap & challenges.
