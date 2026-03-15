# Design Note: Provider Target Validation

Read when: changing provider-side IP acceptance, rejection, output shape, or target-set contracts.

Defines: the observable target-set contract for providers.

Does not define: provider-specific discovery mechanisms, provider syntax, resource ownership, or reconciliation ordering.

## Goal

Give providers one observable contract for the target sets they hand to DNS and WAF reconciliation.

## Core Model

Providers operate per requested IP family and return one family-specific target-set state for that run.

- one state means the desired target set is unavailable for that run
- the other means the desired target set is known for that run

Provider mode determines whether the known-target state may carry an empty IP list:

- dynamic observation providers are known only when they produce a non-empty usable target set for the requested family
- explicit static-empty provider modes use a known empty IP list to mean "manage this family to empty"

Conceptually, this note is how the IP-family ownership model lands at the provider target-set boundary for families that are in scope for provider evaluation:

| provider-target state | ownership-model meaning |
| --- | --- |
| unavailable | target-set unavailable for this run |
| known empty target set | explicit-empty family intent |
| known non-empty target set | non-empty desired-target intent |

Out-of-scope family ownership is represented outside this provider target-set contract, because out-of-scope families are not in provider evaluation scope for that run.

The exact in-memory representation of these states belongs in code comments near the provider-runtime contract, not in this design note.

## Observable Address Rules

Every IP that enters an available provider target set must satisfy these rules:

- it is a valid IP address
- it matches the requested family
- IPv4 mode accepts IPv4-mapped IPv6 only as its plain IPv4 meaning
- IPv4-mapped IPv6 is rejected in IPv6 mode
- unspecified, loopback, multicast, and link-local addresses are rejected
- zone-qualified addresses are rejected because they are not suitable target values
- addresses outside the usual global-unicast shape are warned about, not rejected, after the stronger checks above

## Shared Set Semantics

Available provider target sets follow deterministic set semantics:

- outputs are sorted by `netip.Addr.Compare`
- duplicates are removed
- validation fails fast on the first invalid detected IP
- multi-address providers still report one target set per requested family, not one mixed cross-family set

These rules keep reconciliation behavior independent of discovery order and duplicate observations.

The same target-set contract is consumed by both DNS and WAF reconciliation. Changing provider target-set shape does not by itself change Cloudflare API contracts or heartbeat/notifier reporting contracts.

## Scope Boundary

This note does not define:

- Cloudflare API payload or rate-limit behavior
- heartbeat or notifier message formats
- provider-specific source-scanning rules beyond the shared observable output contract

## Extension Points

- Future providers that return multiple addresses should preserve the same family-specific deterministic set semantics.
- If a provider needs source-specific semantic differences, document them outside this umbrella note in the smallest durable home.
- If reconciliation starts depending on finer-grained validation classes, update this note before introducing new provider-specific semantic exceptions.
