# Design Note: `local.iface` Multi-Address Handling

`local.iface:<IFACE>` detects and manages all global unicast addresses per IP family. It no longer selects only one address.

## Goal

Support multi-address interfaces without changing behavior for non-`local.iface` providers.

This affects DNS reconciliation, WAF reconciliation, and any code that previously assumed one target address per family.

## Core Model

- Detection produces a target set per family, not one mixed address set.
- Reconciliation manages one target set per family for both DNS and WAF flows.
- Only global unicast addresses are considered targets.
- Address ordering is deterministic to avoid unnecessary churn.

## Required Decisions

- Zoned addresses remain rejected.
- Empty detection for a configured family is treated as a failure.
- Metadata drift remains warn-only. TTL, proxy, or comment mismatches do not trigger destructive correction by themselves.
- WAF reconciliation uses keep-and-fill semantics to preserve coverage.
- Monitor and notifier contracts remain unchanged.

## Scope Boundary

This design applies only to `local.iface`.

It does not change:

- provider syntax
- behavior for non-`local.iface` providers
- Cloudflare API contracts
- Cloudflare rate-limit strategy

## Future Development Notes

- Future provider work that returns multiple addresses should preserve family-separated set semantics and deterministic ordering.
- Any change to empty-detection handling must be treated as a destructive-behavior change, because the current model treats empty results as failure for safety.
- Any change to metadata reconciliation or WAF keep-and-fill behavior should be explicit, because both are intentional non-destructive choices.
