# Changelog

## v0.1.1 — 2025-10-07

### Added
- Published contributor operating guidelines in `AGENTS.md` covering coding standards, testing policy, and delivery requirements.

### Changed
- Normalized the server listening address reported in logs to favor `localhost` for wildcard and loopback binds, backed by a dedicated formatter and table-driven tests.
- Expanded README guidance with installation prerequisites, usage scenarios, and refreshed licensing details.

## v0.1.0 — 2025-08-19

### Added
- Introduced the `ghttpd` CLI as a minimal file server compatible with `python -m http.server` flags for port, bind address, directory, and protocol selection.
- Enabled optional TLS via `--tls-cert` and `--tls-key`, enforcing presence checks for both files before starting the server.
- Implemented structured request logging with latency reporting and graceful shutdown handling for `SIGINT` and `SIGTERM`.
- Added the `GHTTPD_DISABLE_DIR_INDEX` environment toggle to block directory listings while still serving files.
- Bootstrapped the project scaffolding with licensing, documentation, and ignore rules.
