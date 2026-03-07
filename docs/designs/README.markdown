# Design Documents

This directory holds self-contained design notes for future developers, including AI agents.

Start here for the current intended design of the codebase, including durable constraints, invariants, and intended extension points.

## Writing Design Notes

Design notes here are reference material for the current intended design. Keep them durable by describing present semantics, invariants, scope boundaries, and extension points directly, without relying on pull-request or release context.

When writing or editing a design note:

- tighten wording so the note stays high-signal, precise, and easy to scan as reference material
- describe present intended behavior directly; avoid vague rollout phrasing such as "the first implementation", "currently", "for now", or "the latest version"
- include historical rationale only when it explains the current design, and anchor it to a concrete version, change, or migration
- mention future work only to mark scope boundaries or extension points, and state what is deferred or what must change
- keep temporary staging plans, migration sequencing, and branch-local rationale out of `docs/designs/`
- extract project-wide policy to project-wide documents such as `project-principles.markdown` or `codebase-architecture.markdown`
- follow the shared feature-availability lifecycle in project-wide docs (`unreleased` before first release tag, then `since version X.Y.Z`)
- keep feature-specific notes focused on the feature contract, and link to shared project-wide rules instead of restating local policy
- if a feature note uncovers a general rule, move that rule upward rather than duplicating it across feature notes

## Project-Wide Documents

- [`project-principles.markdown`](project-principles.markdown): project-wide priorities that guide design tradeoffs.
- [`codebase-architecture.markdown`](codebase-architecture.markdown): the high-level code layout and repository-wide conventions.
- [`lint-suppressions.markdown`](lint-suppressions.markdown): how inline `//nolint` suppressions are used across the repository.
- [`network-security-model.markdown`](network-security-model.markdown): the attacker model and security limits for public-IP detection.

## Feature-Specific Documents

- [`managed-record-ownership.markdown`](managed-record-ownership.markdown): DNS record ownership and `MANAGED_RECORDS_COMMENT_REGEX`.
- [`managed-waf-item-ownership.markdown`](managed-waf-item-ownership.markdown): WAF list item ownership, `WAF_LIST_ITEM_COMMENT`, and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.
- [`local-iface-multi-address.markdown`](local-iface-multi-address.markdown): `local.iface` multi-address semantics.
- [`shoutrrr-input-format.markdown`](shoutrrr-input-format.markdown): `SHOUTRRR` parsing and suspicious single-line space handling.
