# Design Note: Codebase Architecture

This document records the high-level code organization and repository-wide coding conventions.

## Repository Layout

The codebase broadly follows the [standard Go project layout](https://github.com/golang-standards/project-layout), with a few repository-specific support directories:

- `/cmd/` holds executable entry points. Today that is primarily `cmd/ddns/`.
- `/internal/` holds the application logic and supporting packages.
- `/docs/` holds human-facing documentation.
  - `docs/designs/` holds durable design documents for future developers.
- `/build/` holds release and packaging support files.
- `/contrib/` holds external integration examples and platform-specific helper files.
- `/test/` holds specialized tests that do not fit naturally under one package, such as fuzzing support.
- `/.github/` holds repository automation such as CI workflows.

The repository root also contains module metadata, top-level user documentation, and packaging files such as `go.mod`, `Dockerfile`, and `README.markdown`.

## Internal Package Boundaries

The updater is split into small internal packages with explicit responsibilities instead of one large service layer.

- `internal/config/` reads, validates, normalizes, and prints configuration.
- `internal/provider/` detects current IP addresses from different sources.
- `internal/api/` talks to Cloudflare and applies caching around API-facing operations.
- `internal/setter/` reconciles desired DNS and WAF state against current remote state.
- `internal/updater/` orchestrates a full update cycle.
- `internal/monitor/` and `internal/notifier/` report outcomes to external systems.
- `internal/domain/`, `internal/domainexp/`, and `internal/ipnet/` hold domain- and IP-related core types and parsing logic.
- `internal/cron/`, `internal/signal/`, `internal/file/`, and `internal/pp/` provide cross-cutting runtime support.
- `internal/mocks/` holds generated test doubles.
- `internal/sliceutil/` holds small reusable helpers.

This separation is intentional: keep domain logic, provider logic, Cloudflare API logic, reconciliation logic, and user-facing reporting decoupled enough that they can evolve independently.

See the [Go package reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for package-level API structure.

## Coding Conventions

1. Use `%s` instead of `%q` in logs for values that contain only safe characters and are unlikely to be misunderstood without quotes:
   - Cloudflare IDs such as zone, record, and WAF list IDs
   - domain names
   - full WAF list references in the form `account/name`
2. Do not pluralize a variable only because its type is `map[..]...`. For example, a mapping from IP families to detected IPs should be named `detectedIP`, not `detectedIPs`.
3. In tests, keep an expected mocked call on one line in most cases, even if the line becomes long.
