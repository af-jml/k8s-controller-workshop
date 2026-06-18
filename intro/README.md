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

## What it covers (core: ~15 min)

1. Why Kubernetes matters now — the two macro forces (European digital sovereignty and self-hosted AI) and the portability + automation they both need.
2. Kubernetes as a declarative system built on control loops.
3. Why controllers are useful: CRD (noun) + controller (verb).
4. The workshop scenario — AI Report Queue — and how it maps to real AI batch/inference workflows.
5. The 90-minute agenda and success criteria.

## Optional appendix (only if time allows)

- Control-plane internals (API server, etcd, scheduler, controller-manager, kubelet).
- Deeper watch mechanics and networking internals.
