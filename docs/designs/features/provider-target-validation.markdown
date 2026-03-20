# Design Note: Provider Target Validation

Read when: changing provider-side IP acceptance, rejection, output shape, or raw target-data contracts.

Defines: the observable raw target-data contract for providers.

Does not define: provider-specific discovery mechanisms, provider syntax, resource ownership, or reconciliation ordering.

## Goal

Give providers one observable contract for the raw target data they hand to lifecycle derivation.

## Core Model

Providers operate per requested IP family and return one family-specific raw target-data state for that run.

- one state means the raw target data is unavailable for that run
- the other means the raw target data is known for that run

The raw target data is modeled as a family-scoped set of CIDR prefixes.

Provider mode determines whether the known raw-data state may carry an empty result:

- dynamic observation providers are known only when they produce a non-empty usable result for the requested family
- explicit static-empty provider modes use a known empty result to mean "manage this family to empty"

Conceptually, this note is how in-scope IP-family ownership lands at the provider raw-data boundary:

| provider raw-data state    | lifecycle meaning                       |
| -------------------------- | --------------------------------------- |
| unavailable                | raw target data unavailable for this run |
| known empty raw target data | known empty raw target data             |
| known non-empty raw target data | known non-empty raw target data     |

Out-of-scope family ownership is represented outside this provider raw-data contract, because out-of-scope families are not in provider evaluation scope for that run.

## Current Runtime Specialization

The current runtime specialization is address-only:

- IPv4 raw prefixes are carried as their host address and interpreted as singleton `/32` raw data
- IPv6 raw prefixes are carried as their subnet address and interpreted as singleton `/64` raw data

Every IP in the current known result must satisfy these rules:

- it is a valid IP address
- it matches the requested family
- IPv4 mode accepts IPv4-mapped IPv6 only as its plain IPv4 meaning
- IPv4-mapped IPv6 is rejected in IPv6 mode
- unspecified, loopback, multicast, and link-local addresses are rejected
- zone-qualified addresses are rejected because they are not suitable target values
- addresses outside the usual global-unicast shape are warned about, not rejected, after the stronger checks above

## Shared Set Semantics

Under the current runtime specialization, known results follow deterministic set semantics:

- outputs are sorted by `netip.Addr.Compare`
- duplicates are removed
- validation fails fast on the first invalid detected IP
- multi-address providers still report one family-scoped result, not one mixed cross-family result

These rules keep reconciliation behavior independent of discovery order and duplicate observations.

Lifecycle derivation consumes the raw target-data semantics above through this runtime specialization.

## Scope Boundary

This note does not define:

- Cloudflare API payload or rate-limit behavior
- heartbeat or notifier message formats
- provider-specific source-scanning rules beyond the shared observable output contract

## Extension Points

- Future providers that return multiple addresses should preserve the same family-specific deterministic set semantics.
- If a provider needs source-specific semantic differences, document them outside this umbrella note in the smallest durable home.
- If reconciliation starts depending on finer-grained validation classes, update this note before introducing new provider-specific semantic exceptions.
