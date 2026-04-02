# Design Note: Provider Raw-Data Contract

Read when: changing provider-side IP acceptance, rejection, output shape, or raw-data contracts.

Defines: the observable raw-data contract for providers.

Does not define: provider-specific discovery mechanisms, provider syntax, resource ownership, or reconciliation ordering.

## Goal

Give providers one observable contract for the raw data they output for lifecycle detection.

## Core Model

Providers currently operate per in-scope IP family and return one family-specific raw-data state for that run. The raw data is modeled as a family-scoped set of IP addresses with prefix lengths. For a round to yield known raw data, that raw data must already be admissible for all in-scope resources for that family. Future detection may vary by resource, not only by IP family.

This note specifies how in-scope providers produce raw data under the reconciliation intents defined by [Lifecycle Model](lifecycle-model.markdown):

| reconciliation intent | meaning                                      | carried raw data        |
| --------------------- | -------------------------------------------- | ----------------------- |
| `abort`               | admissible raw data unavailable for this run | not applicable          |
| `clear`               | known empty admissible raw data              | not applicable or empty |
| `update`              | known non-empty admissible raw data          | non-empty raw data      |

Out-of-scope families are outside this contract because no provider is called for them.

## Current Runtime Representation

Each element in the raw-data set is an address-plus-prefix-length pair (`ipnet.RawEntry`). The address carries the full observed bits; the prefix length rides alongside but does not clear host bits.

What counts as admissible is defined by derivation, per resource, in the resource-specific notes.

- Providers that discover bare addresses lift them using the effective default prefix length for that family (`IP4_DEFAULT_PREFIX_LEN`, default 32; `IP6_DEFAULT_PREFIX_LEN`, default 64). The default interpretation of bare IPv6 observations is owned by [IPv6 Default Prefix Length Policy](ipv6-default-prefix-length-policy.markdown).
- Providers that discover CIDR-notation entries may preserve the stated prefix length and the full address, or may ignore unsuitable source prefix lengths and fall back to the defaults.
- Host bits must not be eagerly masked. Preserving the original address bits through normalization keeps downstream host-ID derivation meaningful.

Every entry in the current known result must satisfy these rules:

- it must be a global unicast IP address without a zone
- it must match the requested family; IPv4-mapped IPv6 is accepted in IPv4 mode as its plain IPv4 meaning, but rejected in IPv6 mode

## Shared Set Semantics

Known results follow deterministic set semantics:

- outputs should be sorted and de-duplicated by `RawEntry.Compare` (address first, then prefix length)
- validation fails fast on the first invalid detected entry

These rules keep reconciliation behavior independent of discovery order and duplicate observations.

## Scope Boundary

This note does not define:

- Cloudflare API payload or rate-limit behavior
- heartbeat or notifier message formats
- provider-specific source-scanning rules beyond the shared observable output contract

## Extension Points

- Future providers that return multiple addresses should preserve the same family-specific deterministic set semantics.
- If a provider needs source-specific semantic differences, document them outside this umbrella note in the smallest durable home.
- If reconciliation starts depending on finer-grained validation classes, update this note before introducing new provider-specific semantic exceptions.
