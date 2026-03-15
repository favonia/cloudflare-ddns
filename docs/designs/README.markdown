# Design Documents

This directory holds durable design notes for future developers, including AI agents.

Use this file as a retrieval map. Do not read the whole tree by default.

## Always Read

- [`core/project-principles.markdown`](core/project-principles.markdown): read for any design or implementation task that may involve tradeoffs.
- [`core/codebase-architecture.markdown`](core/codebase-architecture.markdown): read when changing package boundaries, configuration flow, or composition-root wiring.

## Read When Needed

- [`guides/readme-writing.markdown`](guides/readme-writing.markdown): read when editing `README.markdown`.
- [`guides/testing-boundaries.markdown`](guides/testing-boundaries.markdown): read when adding or moving tests.
- [`guides/lint-suppressions.markdown`](guides/lint-suppressions.markdown): read when adding or reviewing inline `//nolint`.
- [`features/network-security-model.markdown`](features/network-security-model.markdown): read when changing public-IP detection security behavior or security claims in docs.
- [`features/ip-family-intent-and-target-providers.markdown`](features/ip-family-intent-and-target-providers.markdown): read when changing `IP4_PROVIDER` / `IP6_PROVIDER` semantics, family scope, desired-target intent, or the target-provider model and shutdown behavior derived from IP-family intent.
- [`features/unified-reconciliation-and-lifecycle-ownership.markdown`](features/unified-reconciliation-and-lifecycle-ownership.markdown): read when changing shared DNS/WAF reconciliation semantics, interruption-risk policy, or shutdown authority rules across managed resources.
- [`features/managed-record-ownership.markdown`](features/managed-record-ownership.markdown): read when changing DNS ownership, filtering, or reconciliation semantics.
- [`features/managed-waf-item-ownership.markdown`](features/managed-waf-item-ownership.markdown): read when changing WAF list ownership, filtering, or cleanup semantics.
- [`features/local-iface-multi-address.markdown`](features/local-iface-multi-address.markdown): read when changing `local.iface` multi-address behavior.
- [`features/shoutrrr-input-format.markdown`](features/shoutrrr-input-format.markdown): read when changing `SHOUTRRR` parsing.

## Recording Durable Info

Record durable information in the smallest correct home:

- use `core/` only for project-wide principles or architecture that should affect many tasks
- use `guides/` for shared editing rules reused across unrelated features
- use `features/` for durable feature contracts, invariants, or scope boundaries
- otherwise prefer code comments, tests, or contributor docs instead of growing `docs/designs/`
- default to not creating a new design note
- create a new design note only when no existing note can own the information cleanly, no smaller home fits, and the rule is durable, cross-file, and likely to matter again
- keep local message wording rules, one-file heuristics, temporary rollout notes, and branch-local rationale out of `docs/designs/`

When writing or editing a design note:

- describe present semantics, invariants, scope boundaries, and extension points directly
- avoid rollout phrasing such as "currently", "for now", or "the latest version"
- keep temporary staging plans, branch-local rationale, and review notes out of `docs/designs/`
- keep each note single-purpose and easy to scan
- link to shared policy instead of restating it
- preserve unrelated durable content when revising a note; replace or delete it only when the new design explicitly supersedes it
- update an existing note before considering a new one
- do not create a new note unless the information is durable, cross-file, and likely to matter again
