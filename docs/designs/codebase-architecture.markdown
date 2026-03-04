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

- `internal/config/` reads raw environment inputs, derives validated runtime configs, and prints the resulting settings summary.
- `internal/provider/` detects current IP addresses from different sources.
- `internal/api/` talks to Cloudflare and applies caching around API-facing operations.
- `internal/setter/` reconciles desired DNS and WAF state against current remote state.
- `internal/updater/` orchestrates a full update cycle.
- `internal/heartbeat/` and `internal/notifier/` report outcomes to external systems.
- `internal/domain/`, `internal/domainexp/`, and `internal/ipnet/` hold domain- and IP-related core types and parsing logic.
- `internal/cron/`, `internal/signal/`, `internal/file/`, and `internal/pp/` provide cross-cutting runtime support.
- `internal/mocks/` holds generated test doubles.
- `internal/sliceutil/` holds small reusable helpers.

This separation is intentional: keep domain logic, provider logic, Cloudflare API logic, reconciliation logic, and user-facing reporting decoupled enough that they can evolve independently.

See the [Go package reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for package-level API structure.

## Configuration Lifecycle

Configuration is intentionally split into one raw phase and several runtime-facing phases.

- `RawConfig` holds parsed environment inputs before cross-field validation and derivation.
- `BuiltConfig` groups the validated runtime config slices below so bootstrap code does not pass large config tuples around.
- `HandleConfig` holds validated settings needed to construct the Cloudflare API handle. In practice this is `Auth` plus handle-scoped `api.HandleOptions`, including stable ownership selectors that affect handle-local cache correctness.
- `LifecycleConfig` holds validated schedule and shutdown settings used by the main process loop.
- `UpdateConfig` holds validated provider, domain, WAF, timeout, and write-side settings used during reconciliation.

Constructed heartbeat and notifier services are runtime services, not config slices. The current bootstrap path wires them separately from `BuiltConfig`.

This split keeps the composition root in `cmd/ddns/` honest:

- handle construction consumes handle config
- process orchestration consumes lifecycle config
- update logic consumes update config

The design goal is not to minimize field copying. The goal is to keep each runtime layer from silently depending on settings it does not own.

## Coding Conventions

1. Use `%s` instead of `%q` in logs for values that contain only safe characters and are unlikely to be misunderstood without quotes:
   - Cloudflare IDs such as zone, record, and WAF list IDs
   - domain names
   - full WAF list references in the form `account/name`
2. Do not pluralize a variable only because its type is `map[..]...`. For example, a mapping from IP families to detected IPs should be named `detectedIP`, not `detectedIPs`.
3. In tests, keep an expected mocked call on one line in most cases, even if the line becomes long.
4. For log messages, common existing patterns include:
   - Keep a short primary message and emit follow-up hints or details with `NoticeOncef` or `InfoOncef` when helpful.
   - Use `%q` for raw or untrusted inputs such as user-provided environment values or parser tokens, while continuing to use `%s` for the safe identifiers listed above.
   - Handle long fixed guidance text either by splitting string literals across lines or by using `//nolint:lll` when that keeps the message clearer.
   - Factor repeated guidance into helper functions, such as permission or mismatch hints, instead of duplicating long messages.
5. For user-facing setting names and config field names:
   - keep write-side settings singular when they describe one value written to one managed object, such as `RECORD_COMMENT`
   - keep ownership selectors plural when they scope a managed set, such as `MANAGED_RECORDS_COMMENT_REGEX`
6. When addressing `unparam`, do not remove a parameter mechanically.
   - First check whether the parameter is part of the helper's honest contract.
   - If removing it would hard-code a real dependency into a generic-looking helper, prefer deleting the thin wrapper and calling a more explicit helper directly, or keep the parameter with a local suppression and reason.
   - Avoid "fixing" `unparam` by turning an explicit dependency into hidden coupling.
