# Intro presentation

Open [`index.html`](index.html) in a browser to run the slide deck (reveal.js).

```bash
# from the repo root
open intro/index.html        # macOS
# or just double-click the file
```

Controls: arrow keys to navigate, `S` for speaker notes, `F` for fullscreen, `Esc` for the
slide overview.

## Offline use

The deck loads reveal.js from a CDN. If the venue's network is unreliable, vendor it
locally:

1. Download a reveal.js release from <https://github.com/hakimel/reveal.js/releases>.
2. Unzip it into this folder (so you have `intro/reveal.js/`).
3. Replace the CDN URLs in [`index.html`](index.html) with the local paths.

## What it covers (~30 min)

1. Why the workshop — the gap between using and understanding Kubernetes.
2. Kubernetes as a declarative system built on control loops.
3. Core resource types and how they build on each other.
4. Internals: API server, etcd, scheduler, controller-manager, kubelet.
5. How a pod "responds" to resources via the watch mechanism.
6. CRDs and the operator/controller pattern, with real-world use cases.
7. The workshop scenario — the AI Report Queue — and why it teaches well.
8. The agenda.
