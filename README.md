# Kubernetes Controllers Workshop — "AI Report Queue"

A hands-on workshop that demystifies Kubernetes internals and the **operator / controller
pattern** by building one running scenario end-to-end.

Participants create `ReportRequest` custom resources through a small web UI. A controller
reacts to each request by spawning a Kubernetes **Job** that calls a (mock) AI service,
renders a PDF financial report, and stores it in MinIO. The UI shows the status changing
**live** — `Pending → Processing → Completed` — with a download link for the finished PDF.

> The point of the scenario: a Custom Resource Definition (CRD) is just *data* in the
> Kubernetes API. It only becomes useful when a **controller** continuously reconciles the
> desired state you declared with the actual state of the world. You'll feel this directly
> in step `03` (nothing happens) versus step `04` (the controller brings the CR to life).

## Agenda (~2 hours)

| Part | Folder | Time | What happens |
| ---- | ------ | ---- | ------------ |
| Presentation | [`intro/`](intro/) | ~30 min | Kubernetes overview, core resources, API & internals, CRDs/operators |
| Setup | [`01-setup/`](01-setup/) | ~20 min | Install Docker Desktop, kubectl, kind, k9s; create a local cluster |
| Deploy the app | [`02-app/`](02-app/) | ~25 min | Deploy the web app + MinIO + mock-AI; open it in the browser |
| Custom resources | [`03-custom-resources/`](03-custom-resources/) | ~20 min | Apply the CRD, create a `ReportRequest`, observe that **nothing processes it** |
| Deploy the controller | [`04-controller/`](04-controller/) | ~25 min | Deploy the controller; watch it reconcile, spawn Jobs, produce PDFs, update the UI live |
| Wrap-up | [`05-wrap-up/`](05-wrap-up/) | ~10 min | Recap, cleanup, extension challenges |

## Architecture

```mermaid
flowchart LR
    User([Participant]) -->|creates ReportRequest| UI[Web App UI]
    UI -->|K8s API| API[(kube-apiserver)]
    Controller[ReportRequest Controller] -->|watches| API
    Controller -->|creates Job| Job[Worker Job]
    Job -->|prompt| AI[mock-ai service]
    Job -->|PDF| MinIO[(MinIO S3)]
    Controller -->|updates status| API
    UI -->|watch + SSE| API
    UI -->|presigned URL| MinIO
```

1. The web app creates a `ReportRequest` custom resource via the Kubernetes API.
2. The controller's reconcile loop sees it, sets `phase: Pending`, and creates a worker **Job**
   (owned by the CR via an owner reference).
3. The worker calls the mock-AI service, renders a PDF, and uploads it to MinIO.
4. The controller watches the Job and updates the CR status to `Completed` (or `Failed`).
5. The web app **watches** `ReportRequest` objects and streams live updates to the browser
   over Server-Sent Events; the finished PDF is served through a MinIO presigned URL.

## Repository layout

```
intro/                  reveal.js presentation (open index.html in a browser)
01-setup/ … 05-wrap-up/ numbered workshop steps, each with a README + manifests
src/                    buildable source for all container images
  web-app/              TypeScript app (Express backend + static frontend)
  controller/           Go controller (controller-runtime)
  worker/               Go worker run by each Job (AI call + PDF + MinIO upload)
  mock-ai/              tiny mock AI report service (deterministic, no API key)
scripts/                create-cluster.sh, build-and-load.sh, cleanup.sh, kind-config.yaml
manifests/              shared CRD + namespace referenced by the steps
```

## Prerequisites

You'll install these in [`01-setup/`](01-setup/), but to save venue time, ask participants
to install them beforehand:

- **Docker Desktop** (or another Docker-compatible engine)
- **kubectl** — the Kubernetes CLI
- **kind** — Kubernetes IN Docker, for a local cluster
- **k9s** — a terminal UI for exploring clusters

## Two ways to run the workshop

The container images can be obtained two ways. Pick one before the workshop:

1. **Prebuilt images (default, fastest)** — images are pulled from a public registry.
   Requires internet at the venue. Set the registry in `scripts/env.sh`.
2. **Build locally + load into kind (offline-capable)** — run
   `scripts/build-and-load.sh` to build every image and load it straight into the kind
   node. Slower, but needs no registry.

Both paths are documented in [`01-setup/`](01-setup/).

## Quick start (facilitator)

```bash
# 1. Create the cluster
./scripts/create-cluster.sh

# 2. Build images and load them into kind (offline path)
./scripts/build-and-load.sh

# 3. Follow the numbered steps from 02 onward
```

When you're done: `./scripts/cleanup.sh` deletes the cluster.
