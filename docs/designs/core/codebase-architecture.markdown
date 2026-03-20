# Design Note: Codebase Architecture

Read when: changing package boundaries, configuration flow, or the composition root in `cmd/ddns/`.

Defines: the high-level repository layout, internal package boundaries, and configuration lifecycle.

Does not define: message wording, test-boundary rules, README style, or local refactoring heuristics.

## Repository Layout

The codebase broadly follows the [standard Go project layout](https://github.com/golang-standards/project-layout), with a few repository-specific support directories:

- `cmd/` holds executable entry points. Today this is mainly `cmd/ddns/`.
- `internal/` holds the main application logic and supporting packages.
- `docs/` holds human-facing documentation.
  - `docs/designs/` holds durable design documents for future developers.
- `build/` holds release and packaging support files.
- `contrib/` holds external integration examples and platform-specific helper files.
- `test/` holds specialized tests that do not fit naturally under one package, such as fuzzing support.
- `.github/` holds repository automation such as CI workflows.

The repository root also contains module metadata, top-level user documentation, and packaging files such as `go.mod`, `Dockerfile`, and `README.markdown`.

## Internal Package Boundaries

The updater is split into small internal packages with explicit responsibilities instead of one large service layer.

- `internal/config/` reads raw environment inputs, derives validated runtime configs, and prints the resulting settings summary.
- `internal/provider/` implements family target providers that supply desired IP targets through dynamic observation or explicit provider modes. Creation functions are config-facing: they accept an environment variable key and emit user-facing validation messages. Pure protocol implementations live in `internal/provider/protocol/`.
- `internal/api/` talks to Cloudflare and applies caching around API-facing operations.
- `internal/setter/` reconciles desired DNS and WAF state against current remote state.
- `internal/updater/` orchestrates a full update cycle.
- `internal/heartbeat/` and `internal/notifier/` report outcomes to external systems.
- `internal/domain/`, `internal/domainexp/`, and `internal/ipnet/` hold domain- and IP-related core types and parsing logic.
- `internal/cron/`, `internal/signal/`, `internal/file/`, and `internal/pp/` provide cross-cutting runtime support.
- `internal/mocks/` holds generated test doubles.
- `internal/sliceutil/` holds small reusable helpers.

This separation is intentional: domain logic, provider logic, Cloudflare API logic, reconciliation logic, and user-facing reporting should stay decoupled enough to evolve independently.

See the [Go package reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for package-level API structure.

## Configuration Lifecycle

Configuration is intentionally split into one raw phase and several runtime-facing phases.

- `RawConfig` holds parsed environment inputs before cross-field validation and derivation.
- `BuiltConfig` groups the validated runtime config slices below so bootstrap code does not pass large config tuples around.
- `HandleConfig` holds validated settings needed to construct the Cloudflare API handle. In practice this is `Auth` plus handle-scoped `api.HandleOptions`, including stable ownership selectors that affect handle-local cache correctness.
- `LifecycleConfig` holds validated schedule and shutdown settings used by the main process loop.
- `UpdateConfig` holds validated provider, domain, WAF, timeout, and write-side settings used during reconciliation.

Configuration defaults must be ordinary semantic values. Omitting a setting must have exactly the same effect as explicitly writing that setting's default value. For example, omitting `IP4_PROVIDER` or `IP6_PROVIDER` must match explicitly writing `cloudflare.trace`, and omitting `PROXIED` must match explicitly writing `false`.

Constructed heartbeat and notifier services are runtime services, not config slices. The current bootstrap path wires them separately from `BuiltConfig`.

A key lifecycle invariant is that reporter services are initialized before config or handle setup failure paths, so startup-failure reporting uses the same heartbeat/notifier instances as normal runtime reporting.

This split keeps the composition root in `cmd/ddns/` honest about dependencies:

- handle construction consumes handle config
- process orchestration consumes lifecycle config
- update logic consumes update config

The design goal is not to minimize field copying. The goal is to keep each runtime layer from silently depending on settings it does not own.

## Boundary Notes

- Keep domain logic, provider logic, API logic, reconciliation logic, and user-facing reporting decoupled enough to evolve independently.
- Keep configuration slices honest about ownership. The goal is not to minimize field copying; the goal is to prevent a runtime layer from silently depending on settings it does not own.
- Design notes may define models that are broader than one concrete runtime carrier. Code comments should describe the current carrier precisely without narrowing the design note to today's representation choices.
- A documented implementation gap means the codebase is ready to move the current carrier closer to the design model.
- Put task-specific editing rules in `docs/designs/guides/`, and feature-specific contracts in `docs/designs/features/`.
