# Design Note: Codebase Architecture

This document records the high-level repository layout, internal package boundaries, configuration flow, and coding conventions.

## Repository Layout

The codebase broadly follows the [standard Go project layout](https://github.com/golang-standards/project-layout), with a few repository-specific support directories:

- `/cmd/` holds executable entry points. Today this is mainly `cmd/ddns/`.
- `/internal/` holds the main application logic and supporting packages.
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

This separation is intentional: domain logic, provider logic, Cloudflare API logic, reconciliation logic, and user-facing reporting should stay decoupled enough to evolve independently.

See the [Go package reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for package-level API structure.

## Configuration Lifecycle

Configuration is intentionally split into one raw phase and several runtime-facing phases.

- `RawConfig` holds parsed environment inputs before cross-field validation and derivation.
- `BuiltConfig` groups the validated runtime config slices below so bootstrap code does not pass large config tuples around.
- `HandleConfig` holds validated settings needed to construct the Cloudflare API handle. In practice this is `Auth` plus handle-scoped `api.HandleOptions`, including stable ownership selectors that affect handle-local cache correctness.
- `LifecycleConfig` holds validated schedule and shutdown settings used by the main process loop.
- `UpdateConfig` holds validated provider, domain, WAF, timeout, and write-side settings used during reconciliation.

Constructed heartbeat and notifier services are runtime services, not config slices. The current bootstrap path wires them separately from `BuiltConfig`.

A key lifecycle invariant is that reporter services are initialized before config or handle setup failure paths, so startup-failure reporting uses the same heartbeat/notifier instances as normal runtime reporting.

This split keeps the composition root in `cmd/ddns/` honest about dependencies:

- handle construction consumes handle config
- process orchestration consumes lifecycle config
- update logic consumes update config

The design goal is not to minimize field copying. The goal is to keep each runtime layer from silently depending on settings it does not own.

## Coding Conventions

### Naming and Structure

- Use `%s` instead of `%q` in logs for values that contain only safe characters and are unlikely to be misunderstood without quotes:
  - Cloudflare IDs such as zone, record, and WAF list IDs
  - domain names
  - full WAF list references in the form `account/name`
- Do not pluralize a variable only because its type is `map[..]...`. For example, a mapping from IP families to detected IPs should be named `detectedIP`, not `detectedIPs`.
- In tests, keep an expected mocked call on one line in most cases, even if the line becomes long.
- For user-facing setting names and config field names:
  - keep write-side settings singular when they describe one value written to one managed object, such as `RECORD_COMMENT`
  - keep ownership selectors plural when they scope a managed set, such as `MANAGED_RECORDS_COMMENT_REGEX`

### User-Facing Messages and Logs

- Keep a short primary message and emit follow-up hints or details with `NoticeOncef` or `InfoOncef` when helpful.
- Prefer a two-layer model: keep summary messages plain and compact, and put operational nuance (for example uncertainty or inconsistency risk) in follow-up details.
- Keep summary and detail messages semantically aligned; details may add nuance but should not contradict summaries.
- For user-facing warnings and hints, describe observable behavior and user-action boundaries first.
- Mention mechanism only when it changes what the operator should do; otherwise prefer outcome wording such as `reported but not corrected` or `does not change management scope`.
- Use `%q` for parser and validation diagnostics on raw or untrusted inputs, such as environment values or parser tokens.
- For advisory values where exact text is not required for remediation (for example ignored or overridden settings), avoid assignment-like forms such as `KEY=%q`; prefer `KEY (%s)` where `%s` is a quoted preview value (truncated when long).
- Keep exact non-truncated values in mismatch/validation diagnostics where full-fidelity strings are required for user remediation.
- Continue to use `%s` for safe identifiers listed above.
- Keep short operational `Noticef` and `Infof` messages without trailing periods.
- Handle long fixed guidance text either by splitting string literals across lines or by using `//nolint:lll` when that keeps the message clearer.
- Factor repeated guidance into helper functions, such as permission or mismatch hints, instead of duplicating long messages.
- For mutation failures against external APIs:
  - if the outcome is ambiguous (for example, network timeout), prefer wording like `could not confirm ...`
  - if the failure is definitive (for example, explicit API rejection), `failed to ...` is acceptable
  - keep inconsistency hints explicit when relevant (`records might be inconsistent`, `content may be inconsistent`)
  - future work may add finer error splitting so wording can distinguish definitive rejections from ambiguous transport failures

### Refactoring and Linting

- When addressing `unparam`, do not remove a parameter mechanically.
  - First check whether the parameter is part of the helper's honest contract.
  - If removing it would hard-code a real dependency into a generic-looking helper, prefer deleting the thin wrapper and calling a more explicit helper directly, or keep the parameter with a local suppression and reason.
  - If a wrapper intentionally stays specialized, make that specialization explicit in the helper name instead of hiding it behind a generic-looking signature.
  - Avoid "fixing" `unparam` by turning an explicit dependency into hidden coupling.
- For final cleanup flows:
  - keep cleanup idempotent where possible; missing resources should usually be treated as already cleaned
  - use warning-level notices for unexpected but tolerated cleanup drift instead of hard failures
  - keep mode differences as explicit pre-steps over a shared cleanup pipeline instead of duplicating logic

### Documentation and Comments

- For user-facing feature availability notes:
  - use `unreleased` before the first release tag for that feature
  - use `since version X.Y.Z` (or `available since version X.Y.Z`) only after the release tag exists
  - do not label a planned next release as already released
- For user-facing documentation of settings with default and opt-in modes:
  - describe the common behavior first when there is a meaningful mode delta
  - avoid forced default-versus-opt-in contrasts when semantics are uniform (for example, an empty regex naturally matches any string)
  - emphasize operational deltas that affect safety or lifecycle behavior, such as shutdown cleanup scope
- For design documents under `docs/designs/`, prefer tight and precise wording over broad tutorial-style narrative.
- For `README.markdown`, prioritize readability for not-so-technical users; keep deep mapping details in setting tables or reference sections instead of dense introductory prose.
- For code comments:
  - prioritize tightening comments that describe contracts, invariants, ownership, safety boundaries, lifecycle guarantees, or other non-obvious behavior
  - avoid churn on obvious local comments that only restate nearby code
  - prefer deleting redundant comments over rewriting them with equivalent wording
  - when code relies on external vendor behavior or source material, note whether the dependency is documented upstream or inferred from observed behavior
- For tests:
  - prefer `package foo_test` by default
  - use `*_internal_test.go` for small white-box tests of private helpers
  - use `export_test.go` only as a narrow escape hatch for black-box tests
  - see [testing-boundaries.markdown](testing-boundaries.markdown) for the full rule
