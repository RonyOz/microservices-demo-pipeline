# CI/CD Pipeline Design

This repository uses a branch-aware GitHub Actions workflow defined in:

- `.github/workflows/microservices-ci-cd.yml`

## Service inventory

- `vote` (Java, Maven, Docker, Helm chart)
- `result` (Node.js, npm, Docker, Helm chart)
- `worker` (Go, go modules, Docker, Helm chart)

Build contexts and Dockerfiles:

- `vote/Dockerfile` with context `vote/`
- `result/Dockerfile` with context `result/`
- `worker/Dockerfile` with context `worker/`

## Branch behavior

- `feature/*` pull requests:
  - Detect changed services using path filters.
  - Run tests only for changed services.
  - No image build and no deployment.

- `develop` pushes:
  - Build and push images to GitHub Container Registry for changed services only.
  - Deploy changed services to `staging` namespace using Helm.

- `main` pushes:
  - Build and push images to GitHub Container Registry for changed services only.
  - Deploy changed services to `production` namespace using Helm.

## Required repository secrets

- `KUBE_CONFIG_STAGING`: base64-encoded kubeconfig for staging cluster access.
- `KUBE_CONFIG_PRODUCTION`: base64-encoded kubeconfig for production cluster access.

GitHub automatically provides `GITHUB_TOKEN` for pushing to GHCR in the same repository scope.

## Image naming

Images are pushed as:

- `ghcr.io/<owner>/<repo>-vote`
- `ghcr.io/<owner>/<repo>-result`
- `ghcr.io/<owner>/<repo>-worker`

Tags used:

- `sha-<commit>` for deterministic deployments.
- `develop` on `develop` branch.
- `latest` on `main` branch.

## Kubernetes deployment approach

- Deployments use each service's Helm chart:
  - `vote/chart`
  - `result/chart`
  - `worker/chart`
- The workflow sets image values dynamically per service per commit.

Reference raw manifest examples are available in `k8s/examples/` for best-practice baselines (replicas, probes, resources).

## Suggested structure improvements

Current structure already separates services well. Recommended refinements:

- Add a top-level `services/` directory and move `vote/`, `result/`, and `worker/` under it.
- Keep all charts under a single top-level `charts/` directory for consistent release tooling.
- Add optional per-service test folders:
  - `vote/src/test/java/...`
  - `result/test/...`
  - `worker/*_test.go`

These changes are optional; the current workflow works with the existing layout.
