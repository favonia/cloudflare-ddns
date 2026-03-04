# Design Documents

This directory holds self-contained design notes for future developers, including AI agents.

Start here for durable design decisions, constraints, and extension points.

## Project-Wide Documents

- [`project-principles.markdown`](project-principles.markdown): project-wide priorities that guide design tradeoffs.
- [`codebase-architecture.markdown`](codebase-architecture.markdown): the high-level code layout and repository-wide conventions.
- [`network-security-model.markdown`](network-security-model.markdown): the attacker model and security limits for public-IP detection.

## Feature-Specific Documents

- [`managed-record-ownership.markdown`](managed-record-ownership.markdown): DNS record ownership and `MANAGED_RECORDS_COMMENT_REGEX`.
- [`managed-waf-item-ownership.markdown`](managed-waf-item-ownership.markdown): WAF list item ownership and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.
- [`local-iface-multi-address.markdown`](local-iface-multi-address.markdown): `local.iface` multi-address semantics.
