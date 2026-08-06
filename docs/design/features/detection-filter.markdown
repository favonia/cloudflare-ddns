# Design Note: Detection Filter

Read when: changing `IP4_DETECTION_FILTER`, `IP6_DETECTION_FILTER`, detection filter syntax, filter placement in the update lifecycle, or filter-caused abort behavior.

Defines: the durable contract for filtering detected raw IP data before DNS and WAF derivation.

Does not define: provider discovery internals, resource-specific ownership selectors, or future predicates beyond the currently supported filter language.

## Goal

Let operators constrain detected IP addresses before DNS and WAF targets are derived, while preserving the existing independent IPv4 and IPv6 management model.

## Scope

Detection filters are configured per IP family:

- `IP4_DETECTION_FILTER` applies only to detected IPv4 raw data.
- `IP6_DETECTION_FILTER` applies only to detected IPv6 raw data.

The default value is `keep-all`, which is semantically equivalent to omitting the setting. A missing family in the built runtime configuration means that family is out of scope; an in-scope family always has an explicit filter value.

The filter applies only to detected raw data. It is not a final DNS-record filter, WAF-item filter, provider-source filter, ownership selector, or cleanup rule.

## Lifecycle Placement

The filter is part of detection. It runs after provider output normalization and before derivation:

```text
detection: provider -> normalization -> filter
derivation -> reconciliation
```

Providers return the same raw data regardless of whether a filter is configured. Filtering narrows the normalized raw-data set that downstream derivation receives.

The filter does not mutate remote state and does not decide reconciliation actions. It affects reconciliation only through the raw data and detection intent produced for the family.

## Filter Language

A filter is either the special whole-expression value `keep-all` or a boolean expression over one detected raw entry.

The supported predicate is:

- `addr-in(P)`: true when the address part of the detected raw entry is contained in CIDR prefix `P`

`P` must use explicit CIDR notation. Bare host literals such as `8.8.8.8` or `2001:db8::1` are invalid; use `/32` for one IPv4 address and `/128` for one IPv6 address. Prefix literals must match the filter family.

The supported operators are `!`, `&&`, `||`, and parentheses. `keep-all` is a mode sentinel, not a predicate, so it is valid only as the entire filter.

## Raw-Entry Semantics

Filtering is pointwise. Each normalized raw entry is evaluated independently, and the filtered raw-data set contains exactly the entries whose expression result is true.

`addr-in(P)` checks only the raw entry address. The detected prefix length remains attached to the raw entry and continues into downstream DNS and WAF derivation, but the predicate does not compare that prefix length.

Filtering preserves the normalized set's deterministic ordering. It must not introduce provider-specific ordering or deduplication behavior.

## Detection Intents

Filtering interacts with the lifecycle detection intents as follows:

- Provider unavailable: the family remains `abort`; the filter is not evaluated.
- Provider known empty: the family remains `clear`; the filter is vacuously satisfied.
- Provider known non-empty and at least one entry remains after filtering: the family remains `update`, and downstream derivation sees the filtered non-empty raw-data set.
- Provider known non-empty and no entries remain after filtering: the family becomes `abort`.

There is no separate filtered-away reconciliation intent. Filter-caused emptying is an ordinary lifecycle `abort` so existing managed DNS records and WAF items for that family are preserved for the round. The other IP family remains independent.

## Operator Messaging

Filter-caused `abort` must be distinguishable from provider-caused `abort` in logs, heartbeat summaries, and notifier summaries. The notice should name filtering and avoid provider or network troubleshooting hints that do not apply.
