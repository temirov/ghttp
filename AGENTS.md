# AGENTS.md

## Core Principles

* Reuse existing code first; extend or adapt before writing new code.
* Generalize existing implementations instead of duplicating them.
* Favor data structures (maps, registries, tables) over branching logic.
* Use composition, interfaces, and method sets (“object-oriented Go”).
* Depend on interfaces; return concrete types.
* Group behavior on receiver types with cohesive methods.
* Inject all external effects (I/O, network, time, randomness, OS).
* No hidden globals for behavior.
* Treat inputs as immutable; return new values instead of mutating.
* Separate pure logic from effectful layers.
* Keep units small and composable.
* Minimal public API surface.
* Provide only the best solution — no alternatives.

---

## Deliverables

* Only changed files.
* No diffs, snippets, or examples.
* Must compile cleanly.
* Must pass `go fmt ./... && go vet ./... && go test ./...`.

---

## Code Style

* No single-letter identifiers.
* Long, descriptive names for all identifiers.
* No inline comments.
* Only GoDoc for modules and exported identifiers.
* No repeated inline string literals — lift to constants.
* Return `error`; wrap with `%w` or `errors.Join`.
* No panics in library code.
* Use zap for logging; no `fmt.Println`.
* Prefer channels and contexts over shared mutable state.
* Guard critical sections explicitly.

---

## Project Structure

* `cmd/` for CLI entrypoints.
* `internal/` for private packages.
* `pkg/` for reusable libraries.
* No package cycles.
* Respect existing layout and naming.

---

## Configuration & CLI

* Use Viper + Cobra.
* Flags optional when provided via config/env.
* Validate config in `PreRunE`.
* Read secrets from environment.

---

## Dependencies (Approved)

* Core: `spf13/viper`, `spf13/cobra`, `uber/zap`.
* HTTP: `gin-gonic/gin`, `gin-contrib/cors`.
* Data: `gorm.io/gorm`, `gorm.io/driver/postgres`, `jackc/pgx/v5`.
* Auth/Validation: `golang-jwt/jwt/v5`, `go-playground/validator/v10`.
* Testing: `stretchr/testify`.
* Optional: `joho/godotenv`, `prometheus/client_golang`, `robfig/cron/v3`.
* Prefer standard library whenever possible.

---

## Testing

* No filesystem pollution.
* Use `t.TempDir()` for temporary dirs.
* Dependency injection for I/O.
* Table-driven tests.
* Mock external boundaries via interfaces.
* Use real, integration tests with comprehensive coverage

---

## Web/UI

* Use Gin for routing.
* Middleware for CORS, auth, logging.
* Use Bootstrap built-ins only.
* Header fixed top; footer fixed bottom via Bootstrap utilities.

---

## Performance & Reliability

* Measure before optimizing.
* Favor clarity first, optimize after.
* Use maps and indexes for hot paths.
* Always propagate `context.Context`.
* Backoff/retry as data-driven config.

---

## Security

* Secrets from env.
* Never log secrets or PII.
* Validate all inputs.
* Principle of least privilege.

---

## Assistant Workflow

* Read repo and scan existing code.
* Plan reuse and extension.
* Replace branching with data tables where appropriate.
* Implement minimal, cohesive types.
* Inject dependencies.
* Prove with table-driven tests.

---

## Review Checklist

* [ ] Reused/extended existing code.
* [ ] Replaced branching with data structures where appropriate.
* [ ] Minimal, cohesive public API.
* [ ] All side effects injected.
* [ ] No single-letter identifiers.
* [ ] Constants used for repeated strings.
* [ ] zap logging; contextual errors.
* [ ] Config via Viper; validated in `PreRunE`.
* [ ] Table-driven tests; no filesystem pollution.
* [ ] `go fmt`, `go vet`, `go test ./...` pass.
