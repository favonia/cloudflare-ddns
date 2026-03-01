# Design Documents

This directory contains self-contained design documents for future developers, including AI agents.

Start here for the current intended design of the codebase, including durable constraints, invariants, and intended extension points.

## Writing Design Notes

Design notes in this directory are reference material for understanding the current intended design of the codebase. They should stay durable by describing present semantics, invariants, scope boundaries, and extension points directly, without depending on the pull request or release context that introduced them.

When writing or editing a design note:

- describe present intended behavior directly instead of using vague rollout phrasing such as "the first implementation", "currently", "for now", or "the latest version"
- include historical rationale only when it helps explain why the current design is the way it is, and anchor that context to a concrete version, change, or migration
- mention future work only to mark a scope boundary or extension point, and say explicitly what is deferred or what would need to change
- keep temporary staging plans, migration sequencing, and branch-local rationale out of `docs/designs/`
- extract project-wide policy to project-wide documents such as `project-principles.markdown` or `codebase-architecture.markdown`
- keep feature-specific notes focused on the feature's own contract, while linking to shared project-wide rules instead of restating a separate local philosophy
- if a feature note uncovers a general rule, move that rule upward rather than duplicating it across feature notes

## Project-Wide Documents

- [`project-principles.markdown`](project-principles.markdown): project priorities that guide design tradeoffs.
- [`codebase-architecture.markdown`](codebase-architecture.markdown): the high-level code layout and coding conventions.
- [`network-security-model.markdown`](network-security-model.markdown): the network threat model for public IP detection and its security boundaries.

## Feature-Specific Documents

- [`managed-record-ownership.markdown`](managed-record-ownership.markdown): DNS record ownership and `MANAGED_RECORDS_COMMENT_REGEX`.
- [`managed-waf-item-ownership.markdown`](managed-waf-item-ownership.markdown): WAF list item ownership, `WAF_LIST_ITEM_COMMENT`, and `MANAGED_WAF_LIST_ITEM_COMMENT_REGEX`.
- [`local-iface-multi-address.markdown`](local-iface-multi-address.markdown): multi-address handling for `local.iface`.
