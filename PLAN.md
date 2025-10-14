# PLAN.md

This document outlines the plan to address the open issues in the `ghttp` repository.

## Issue HT-01: Add Docker images for multi-platform support

### 1. Branching Strategy
- Create a new git branch: `feature/HT-01-docker-images`

### 2. Dockerfile
- Create a multi-stage `Dockerfile`.
  - Stage 1: Use an official Go image (e.g., `golang:1.21-alpine`) as the builder.
  - Copy `go.mod`, `go.sum` and download dependencies.
  - Copy the source code.
  - Build the `ghttp` binary with optimizations for a static build.
  - Stage 2: Use a minimal base image (e.g., `scratch` or `alpine:latest`).
  - Copy the compiled binary from the builder stage.
  - Set the entrypoint to the `ghttp` binary.

### 3. GitHub Actions Workflow
- Create a new workflow file at `.github/workflows/docker-publish.yml`.
- The workflow will trigger on pushes to `main` and `feature/HT-01-docker-images`.
- Use `docker/setup-qemu-action` and `docker/setup-buildx-action` to enable multi-platform builds.
- Add a step to log in to a Docker registry (I will use GitHub Container Registry).
- Add a step to build and push the Docker image using `docker/build-push-action`.
- Configure the `build-push-action` to build for `linux/amd64`, `linux/arm64`, and `windows/amd64`.
- Define tags for the images (e.g., `latest`, git SHA).

### 4. Testing
- Build the Docker image locally to verify it works.
- Run the container and test the `ghttp` command.
- Push the changes to the remote branch to trigger the GitHub Actions workflow.
- Verify that the workflow completes successfully and the images are published.

### 5. Documentation & Cleanup
- Update `NOTES.md` to mark `[HT-01]` as complete.
- Commit all the changes with a descriptive message.
