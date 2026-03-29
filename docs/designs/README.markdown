# Design Documents

This directory holds durable design notes for future developers.

Use this file as a retrieval map. Do not read the whole tree by default.

## Always Read

- [`core/project-principles.markdown`](core/project-principles.markdown): read for any design or implementation task that may involve tradeoffs.
- [`core/codebase-architecture.markdown`](core/codebase-architecture.markdown): read when changing package boundaries, configuration flow, or composition-root wiring.

## Read When Needed

- [`guides/readme-writing.markdown`](guides/readme-writing.markdown): read when editing `README.markdown`.
- [`guides/testing-boundaries.markdown`](guides/testing-boundaries.markdown): read when adding or moving tests.
- [`guides/go-lint-suppressions.markdown`](guides/go-lint-suppressions.markdown): read when adding or reviewing Go inline `//nolint`.
- [`guides/policy-for-scripts.markdown`](guides/policy-for-scripts.markdown): read when changing code, module boundaries, or module-wide `golangci-lint` policy in `scripts/github-actions/*`.
- [`guides/enforcement-point-explanations.markdown`](guides/enforcement-point-explanations.markdown): read when writing code comments, design-note pointers, or other non-README explanatory text at an enforcement point.
- [`guides/naming-conventions.markdown`](guides/naming-conventions.markdown): read when renaming code identifiers, config fields, or user-facing setting names.
- [`guides/operator-messages.markdown`](guides/operator-messages.markdown): read when editing operator-facing runtime messages outside `README.markdown`.

### Shared lifecycle and resource models

- [`features/lifecycle-model.markdown`](features/lifecycle-model.markdown): read when changing how one updater run starts, detects raw data, derives resource-specific targets, reconciles managed state, or cleans up on shutdown.
- [`features/ownership-model.markdown`](features/ownership-model.markdown): read when changing the shared ownership predicates or deletion-eligibility inference across DNS and WAF.
- [`features/reconciliation-algorithm.markdown`](features/reconciliation-algorithm.markdown): read when changing reconciliation semantics or interruption-risk policy across managed resources.

### Supporting feature models

- [`features/provider-raw-data-contract.markdown`](features/provider-raw-data-contract.markdown): read when changing provider-side IP acceptance, rejection, output shape, or raw-data contracts.
- [`features/network-security-model.markdown`](features/network-security-model.markdown): read when changing public-IP detection security behavior or security claims in docs.
- [`features/ipv6-default-prefix-length-policy.markdown`](features/ipv6-default-prefix-length-policy.markdown): read when changing the default meaning of bare detected IPv6 addresses, including IPv6 lifting defaults or WAF exact-address versus network-presence semantics.

### Resource-specific instantiations

- [`features/managed-record-ownership.markdown`](features/managed-record-ownership.markdown): read when changing the DNS instantiation of the shared ownership and reconciliation models, including DNS attribute-based ownership, filtering, or metadata reconciliation.
- [`features/managed-waf-item-ownership.markdown`](features/managed-waf-item-ownership.markdown): read when changing the WAF instantiation of the shared ownership and reconciliation models, including WAF attribute-based ownership, filtering, or deletion-target consequences.

## Directory Scope

Use `docs/designs/` only for durable design information that is broader than one local edit.

- use `core/` for project-wide principles and architecture that affect many tasks
- use `guides/` for shared editing or explanation rules reused across unrelated topics
- use `features/` for durable feature contracts, invariants, and scope boundaries
- update an existing note before adding a new one
- add a new note only when no existing note can own the information cleanly and the information is durable, cross-file, and likely to matter again
- keep temporary rollout notes, branch-local rationale, review notes, and one-file heuristics out of `docs/designs/`
