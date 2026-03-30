# CI/CD Pipeline Design

This repository uses a branch-aware GitHub Actions workflow defined in:

- `.github/workflows/app.yml`

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
  - After successful build/push stage completion, trigger `ops-repo` with one event per changed service.
  - Trigger `ops-repo` with a repository dispatch event per changed service.
  - Event payload includes image repository, unique tag, and target environment (`staging`).

- `main` pushes:
  - Build and push images to GitHub Container Registry for changed services only.
  - After successful build/push stage completion, trigger `ops-repo` with one event per changed service.
  - Trigger `ops-repo` with a repository dispatch event per changed service.
  - Event payload includes image repository, unique tag, and target environment (`production`).

## Required repository secrets

- `OPS_REPO_TOKEN`: token with permission to send repository dispatch events to the ops repository.

## Required repository variables

- `OPS_REPO`: target repository in `owner/repo` format (example: `my-org/microservices-ops`).

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

The dispatch payload to `ops-repo` includes at least:

- `service`
- `image`
- `tag` (`sha-<commit>`)
- `image_ref`
- `environment`
- source metadata (`source_repo`, `source_sha`, `source_ref`, `source_actor`)

## Kubernetes deployment approach (ops-repo)

- Deployment responsibility is delegated to `ops-repo`.
- `ops-repo` uses separate workflows:
  - app deploy workflow triggered by `repository_dispatch` from app-repo.
  - infra deploy workflow for Kafka/PostgreSQL (`infrastructure/k8s`) triggered manually or by infra path changes.
- `ops-repo` receives the image tag and deploys with Helm using runtime flags (for example `--set image=<repo>:<tag>`), without modifying files in Git.

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
