# gitops-slo-app

Service code and CI for **[gitops-slo-platform](https://github.com/Sinerror/gitops-slo-platform)**
(moving a microservice app to GitOps with SLOs). Русский: [README.md](README.md).

The app is [Online Boutique](https://github.com/GoogleCloudPlatform/microservices-demo) v0.10.7
(Apache-2.0). Most services use upstream images; this repo contains only what our CI builds:

| Service | What it is |
| --- | --- |
| `src/checkoutservice` | upstream fork: publishes an order event to Kafka after checkout (`events.go`, async — Kafka can't break checkout) and a controllable fault `FAULT_ERROR_RATE` for the "bad release" drill (`faults.go`) |
| `src/order-consumer` | our Go service: consumes `orders`, exposes Prometheus metrics, `PROCESSING_DELAY_MS` for the "consumer lag" drill |

## Pipeline

push to main → test (gofmt, vet, `go test -race`) → build → Trivy gate (HIGH/CRITICAL) → push to GHCR
→ commit `sha-xxxxxxx` tag to `gitops-slo-platform/envs/staging` → ArgoCD → smoke test.
Manual `promote` workflow copies the tag to `envs/prod`.

- Actions are pinned by commit SHA; Trivy runs from the official image pinned by digest
  (`aquasecurity/trivy-action` is not used after it was compromised).
- GitOps commits carry `Source-Commit` / `Source-Commit-Time` trailers used by `scripts/dora.sh` to compute lead time.
- The Trivy gate (same command as in CI, run locally before the first push) failed the initial Go 1.27.0
  images with 7 and 4 HIGH CVEs; after bumping the builder to Go 1.27.3 and dependencies — 0.

## One-time setup

1. Make GHCR packages public after the first push (kind pulls them anonymously).
2. Secret `GITOPS_TOKEN`: fine-grained PAT scoped **only** to `Sinerror/gitops-slo-platform`, *Contents: read & write*.
