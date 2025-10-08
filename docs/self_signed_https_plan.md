# Plan: Self-Signed HTTPS Support

## Objective
Enable `ghttp` to provision and serve HTTPS locally using a self-signed Certificate Authority (CA) that can be registered in macOS, Windows, and Linux trust stores. The feature should make it possible to browse the local development site over `https://localhost:<port>` without browser warnings.

## Guiding Principles
1. **User Experience**: Provide a single CLI workflow to install the root certificate, generate site certificates on demand, and launch the HTTPS server.
2. **Security**: Store CA private keys securely with filesystem permissions restricted to the invoking user. Provide commands to uninstall certificates and rotate credentials.
3. **Portability**: Support macOS, Windows, and Linux with OS-specific helpers while keeping shared logic within `internal/` packages.
4. **Maintainability**: Follow the existing struct-oriented design guidelines, use widely adopted packages, and keep configuration in constants and enumerations.

## Architectural Overview
- Introduce a `cmd/server/https` subcommand (using Cobra) to orchestrate certificate management and server startup.
- Implement an `internal/certificates` package with the following key types:
  - `CertificateAuthorityManager`: Generates and persists a development root CA using Go's `crypto/x509` and `crypto/rsa` packages.
  - `TrustStoreInstaller`: Interface with OS-specific implementations (`internal/certificates/truststore`) for macOS, Windows, and Linux to install/remove the root certificate.
  - `ServerCertificateIssuer`: Issues leaf certificates (SAN: localhost, 127.0.0.1, ::1, and custom hosts) signed by the root CA.
- Extend the existing `internal/serverdetails` package with HTTPS metadata to reuse the logging formatter for both HTTP and HTTPS URLs.
- Provide configuration via constants and Viper-managed settings (for file paths, key sizes, certificate validity periods, etc.).

## Key Workstreams
### 1. CLI Enhancements
- Add Cobra-based commands:
  - `ghttp https setup`: Generates CA if missing and installs it into the OS trust store.
  - `ghttp https serve`: Issues a leaf certificate (regenerating when expired) and runs the HTTPS server.
  - `ghttp https uninstall`: Removes the CA from the trust store and deletes local key material.
- Update documentation (`README.md`) with instructions, security considerations, and cleanup steps.

### 2. Certificate Authority Management
- Use Go's `crypto/x509`, `crypto/rand`, and `encoding/pem` to create a 4096-bit RSA CA certificate valid for ~5 years.
- Store the CA in `~/.config/ghttp/certs/ca.{pem,key}` with `0600` permissions.
- Implement rotation logic: regenerate CA when nearing expiry or when explicitly requested.
- Provide comprehensive unit tests for the CA manager using table-driven behavior-based scenarios.

### 3. Trust Store Integration
- macOS: Use `security add-trusted-cert` and `security delete-certificate` commands; require sudo for installation. Wrap command execution with descriptive error handling.
- Windows: Utilize `certutil -addstore -f Root` and `certutil -delstore Root` via `os/exec`. Handle PowerShell detection for user prompts.
- Linux: Support major distributions by placing the CA in `/usr/local/share/ca-certificates` (Debian/Ubuntu) and invoking `update-ca-certificates`, plus `trust anchor` for Fedora-based systems. Document manual steps for unsupported distros.
- Provide integration tests guarded by build tags or using mocks to avoid modifying the developer machine during automated runs.

### 4. HTTPS Server Support
- Expand the server startup to accept TLS configuration (certificate and key paths) and run `http.Server` with `TLSConfig` from Go's standard library.
- Reuse the `ServingAddressFormatter` to ensure HTTPS URLs are fully qualified in logs.
- Offer flags for overriding certificate locations and enabling mutual TLS in future iterations.

### 5. Testing Strategy
- Unit tests for certificate generation, serialization, and renewal logic using temporary directories.
- Mock-based tests for trust store installers to validate command invocation without touching the OS.
- Integration tests that start an HTTPS server with generated certificates and perform GET requests using Go's `http.Client` configured with the generated root CA.
- Document manual QA steps for verifying browser trust on each platform.

## Risks and Mitigations
- **OS Permission Requirements**: Trust store modifications often need elevated privileges. Mitigation: detect permission issues early, provide clear prompts, and support `--print-script` mode for manual execution.
- **Certificate Leakage**: Ensure CA and private keys use restrictive permissions and are excluded from logs. Offer `ghttp https uninstall` to clean up.
- **Toolchain Compatibility**: Depend exclusively on Go standard library and widely used packages (`cobra`, `viper`, `zap`) to reduce external dependencies.

## Estimated Effort
- Design and scaffolding: 1-2 days
- Certificate management implementation: 2-3 days
- Trust store installers (macOS/Windows/Linux) with testing: 4-5 days
- HTTPS server integration and logging polish: 1 day
- Documentation and QA: 1 day

**Total:** Approximately 9-12 days of engineering effort with cross-platform manual verification.

## Next Steps
1. Review and refine this plan with stakeholders.
2. Create GitHub issues for each workstream with detailed acceptance criteria.
3. Prioritize implementation milestones (e.g., start with macOS + Linux support, then add Windows).
