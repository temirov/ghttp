# HT-01 – Multi-platform Docker distribution
- `Dockerfile`: split Linux build logic into explicit multi-platform stages that default to amd64/arm64 hosts while remaining BuildKit-friendly.
- `docker/Dockerfile.windows`: introduce a Windows-specific container recipe that packages the Windows binary with nanoserver.
- `.github/workflows/docker-publish.yml`: ensure the existing pipeline publishes Linux (amd64, arm64) images to GHCR on pushes and releases while Windows remains a cross-compiled binary.
- `README.md`: document the new published images, platforms, and usage guidance including Windows invocation notes.

# HT-02 – Stabilize container integration tests
- `tests/integration/docker_test.go`: consolidate docker helpers, add pre-flight prerequisite detection, and extend multi-platform coverage to include Windows manifests when the environment supports it.
- `tests/integration/prerequisites_test.go`: cover the new prerequisite detection helper with table-driven tests to prove skip conditions.
- `Dockerfile`: expose explicit metadata used by the tests to assert image expectations without needing network connectivity.
- `NOTES.md`: mark HT-01 and HT-02 as completed when the fixes and tests land.
