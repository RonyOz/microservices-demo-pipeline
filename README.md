# Microservices Demo Pipeline

This repository contains the application services for a voting demo and the application-side CI pipeline.

Baseline workflow:

- app-repo builds and pushes images only
- app-repo triggers ops-repo with the generated immutable tag
- ops-repo performs deployments with Helm using runtime values (no Git file mutation)

## Architecture

![Architecture diagram](architecture.png)

- Vote service (Java, Spring Boot): [vote](vote)
- Result service (Node.js): [result](result)
- Worker service (Go): [worker](worker)
- Queue: Kafka
- Database: PostgreSQL

## CI/CD Model

This repository uses the workflow in [.github/workflows/app.yml](.github/workflows/app.yml).

### Branch behavior

- feature/* pull requests:
  Detect changed services with path filters, run tests only for changed services, and skip build/deploy trigger.

- develop pushes:
  Build and push changed service images, then trigger ops-repo with one dispatch event per changed service for staging.

- main pushes:
  Build and push changed service images, then trigger ops-repo with one dispatch event per changed service for production.

### Image naming and tags

- Images:
  ghcr.io/OWNER/REPO-vote, ghcr.io/OWNER/REPO-result, ghcr.io/OWNER/REPO-worker.

- Tags:
  sha-COMMIT (immutable deploy tag), develop (develop branch), latest (main branch).

### Dispatch payload to ops-repo

Each dispatch includes at least:

- service
- image
- tag
- image_ref
- environment
- source_repo
- source_sha
- source_ref
- source_actor

## Required configuration in app-repo

GitHub Actions secrets:

- OPS_REPO_TOKEN: token with access to send repository dispatch events to ops-repo.

GitHub Actions variables:

- OPS_REPO: target ops repository in owner/repo format.

Note:

- Kubeconfig secrets are no longer required in this repository.
- Cluster credentials belong to ops-repo, where Helm and kubectl run.

## Ops-repo responsibilities

Ops-repo should keep two separate workflows:

- App deploy workflow: triggered by repository_dispatch from app-repo, deploys service using image:tag from payload.
- Infra deploy workflow: manual or infra-path based, deploys Kafka and PostgreSQL chart.

Reference templates created in this repository:

- [docs/ops-repo-workflow.example.yml](docs/ops-repo-workflow.example.yml)
- [docs/ops-repo-infra-workflow.example.yml](docs/ops-repo-infra-workflow.example.yml)

## Services inventory

- vote: Java, Maven, Docker, Helm chart
- result: Node.js, npm, Docker, Helm chart
- worker: Go, go modules, Docker, Helm chart

## Build contexts and Dockerfiles

- vote/Dockerfile with context vote/
- result/Dockerfile with context result/
- worker/Dockerfile with context worker/

GitHub automatically provides GITHUB_TOKEN for pushing images to GHCR in this repository scope.

## Deployment notes

- Ops-repo deploys with Helm runtime values and does not need Git file mutation.
- Reference manifests are available in k8s/examples/.

## Suggested structure improvements

- Add a top-level services/ directory and move vote/, result/, and worker/ under it.
- Keep all charts under a single top-level charts/ directory.
- Add optional per-service test folders:
  vote/src/test/java/..., result/test/..., worker/*_test.go
