# 03 · Custom resources

Goal: teach the cluster a brand new resource type with a **CustomResourceDefinition (CRD)**,
create a `ReportRequest`, and observe the key lesson of the whole workshop:

> A CRD on its own is just **data**. Nothing happens to it until a **controller** acts.

Time: ~15 minutes.

## Learning objectives

- Apply and inspect a CRD-backed custom resource type.
- Verify that CRDs provide schema, validation, and API ergonomics by themselves.
- Observe that no processing happens until a controller reconciles the resource.
- Predict the exact behavior the controller should add in step 04.

## 1. Apply the CRD

```bash
kubectl apply -f manifests/crd.yaml
```

This registers a new resource type, `ReportRequest`, in the API group
`reports.workshop.io`. The API server now serves it like any built-in resource.

```bash
kubectl get crds | grep reports.workshop.io
kubectl explain reportrequest.spec
kubectl explain reportrequest.status
```

`kubectl explain` works because the CRD includes an OpenAPI schema. The API server uses that
same schema to **validate** objects you submit.

## 2. Create a ReportRequest — two ways

### a) With kubectl

```bash
kubectl apply -f 03-custom-resources/sample-reportrequest.yaml
kubectl get reportrequests -n report-queue
```

You'll see your object listed, with custom columns (Phase, Title, Job, Age) defined by the
CRD. Note the **Phase is empty** — no status has been set.

### b) From the web UI

Open <http://localhost:8080> and submit the form. The new row appears instantly in the table
(the app is *watching* ReportRequests) — and it sits at **Pending**.

## 3. Observe: nothing is processing them

```bash
kubectl get reportrequests -n report-queue -o yaml | less
```

Look at the object: it has your `spec`, but no meaningful `status`, and **no Job** was
created:

```bash
kubectl get jobs -n report-queue
# No resources found
```

This is the whole point. The API happily stores `ReportRequest` objects, validates them, and
serves them over the API and to the UI — but they are **inert**. There is no behaviour
attached to this new noun yet.

## 4. Inspect the resource in k9s

In k9s, type `:reportrequests` (or `:rr`, the CRD's short name) to list them. They're
first-class citizens now — selectable, describable, deletable — just like Pods.

## Talking points

- A CRD adds a **noun** to the API. A controller adds the **verb**.
- Validation, `kubectl` integration, RBAC, and watches all work *for free* once the CRD
  exists — that's a lot of leverage.
- The `status` subresource is separate from `spec`: users own `spec`, controllers own
  `status`. We'll see the controller write `status` next.

---

Next: [`04-controller/`](../04-controller/) — bring the resources to life.
