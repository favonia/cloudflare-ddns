# Design Note: Managed DNS Record Ownership

Read when: changing DNS ownership, managed-record filtering, or DNS reconciliation semantics tied to ownership.

Defines: the durable contract for `MANAGED_RECORDS_COMMENT_REGEX`, `RECORD_COMMENT`, and ownership-aware DNS reconciliation.

Does not define: exact Cloudflare request payload shapes or local warning text.

`MANAGED_RECORDS_COMMENT_REGEX` lets one updater instance decide which existing DNS records it owns.

## Goal

Safely isolate ownership when multiple updater instances may touch overlapping DNS names. Ownership determines record discovery, updates, duplicate cleanup, and deletion.

## Core Model

- `RECORD_COMMENT` is the comment this instance writes to DNS records that it creates or updates.
- `MANAGED_RECORDS_COMMENT_REGEX` is the selector used to decide which existing DNS records are managed by this instance.
- These settings are intentionally separate: one controls what this instance writes, and the other controls what it may mutate.

The selector uses Go `regexp` RE2 syntax with `MatchString` semantics. It is not an implicit full-match pattern.

The empty default matches all comments, preserving pre-selector behavior. Ownership isolation is opt-in.

## Required Invariants

- `MANAGED_RECORDS_COMMENT_REGEX` is compiled during config building and stored in the handle-facing runtime config.
- After successful config building, the compiled regex is always non-nil, including the default empty template.
- `RECORD_COMMENT` must match `MANAGED_RECORDS_COMMENT_REGEX`.

The last rule prevents self-orphaning.

## Reconciliation Semantics

Managed-record filtering happens immediately after listing DNS records from Cloudflare.

Only matched records participate in:

- IP parsing
- TTL, proxied, and comment drift warnings
- stale-record detection
- duplicate cleanup
- `DELETE_ON_STOP`

Unmatched records are invisible to DNS mutation logic, so the updater may create a new managed record even if an unmanaged record already has the desired IP address.

### Metadata Reconciliation for New Creates

When DNS reconciliation needs to satisfy unmatched targets, metadata is resolved per `(domain, record type)` unit from stale records only.

The flow is intentionally split into two independent paths:

1. Matched-path reconciliation: reduce multiple matched records for one target to one keeper.
2. Stale-to-new reconciliation: derive metadata from stale records, then satisfy unmatched targets.

In stale-to-new reconciliation, recycling a stale record is only an optimization of delete+create to reduce downtime; the target metadata is always the reconciled stale-source metadata.

- Scalar fields (`TTL`, `PROXIED`, `RECORD_COMMENT`):
  - empty source set: use configured value
  - unanimous source value: inherit source value
  - non-unanimous source values: use configured value and emit one ambiguity warning per field
- Tag field (`TAGS`):
  - tag name is compared case-insensitively
  - tag value is compared case-sensitively
  - configured-default tags are sticky unless all sources omit them
  - non-default tags require unanimity across sources to be inherited

Duplicate records with the target IP are reduced deterministically: select one keeper, then delete the rest.

### Interruption Risk Tiers

For timeout-sensitive mutation ordering, DNS reconciliation uses the following risk tiers:

- `R0`: missing target coverage
- `R1`: wrong-IP exposure (managed records on non-target IPs)
- `R2a`: proxied mismatch (expected `PROXIED=false`, actual `true`)
- `R2b`: proxied mismatch (expected `PROXIED=true`, actual `false`)
- `R2c`: TTL drift
- `R2d`: comment/tags drift
- `R3`: duplicate-hygiene residual risk

Runtime ordering intentionally stays coarse for maintainability. The implementation
does not schedule separate sub-stages for each `R2*` subtype.

### Timeout-Aware Mutation Ordering

DNS reconciliation follows interruption-aware ordering so partial execution under timeout/failure is still useful:

1. Satisfy unmatched targets first (recycle stale records, then create if needed).
2. Delete stale leftovers.
3. Update kept matched records if metadata reconciliation requires it.
4. Delete duplicate matched records that do not match resolved metadata.
5. Delete duplicate matched records that already match resolved metadata.

This ordering prioritizes higher-impact prefix improvements (`R0`, then `R1`)
before lower-tier metadata and hygiene risks.

### API Contract Boundary

`setter` and `api.Handle` use the following DNS mutation contract:

- `UpdateRecord` reconciles one managed record to desired state for both:
  - content/IP
  - metadata in scope (`TTL`, `PROXIED`, `RECORD_COMMENT`, `TAGS`)
- desired-state mutation source is `desiredParams`.

This contract is intentionally explicit because historical versions used an
IP-only update path that preserved metadata. Any future contract change here
must update interface comments, implementation comments, and API write tests
together.

## Caching Contract

Record-list caches store already-filtered managed records.

This requires one handle and its bound setter to use one stable managed-record filter for their lifetime. The current cache key does not include filter identity.

## Tradeoffs

- The design prefers strict ownership isolation over reusing foreign records. This may leave parallel records with the same IP address, but it avoids mutating another deployment's records.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `RECORD_COMMENT` to reduce operator confusion.

## Scope Boundary

This design applies only to DNS record ownership based on DNS record comments.

It is not a general ownership abstraction for all managed resources. WAF list item ownership remains separate, and DNS-less or WAF-only runs do not use this selector.

## Extension Points

- If one process ever needs multiple ownership scopes for the same domain and IP family, the cache design must change so filter identity becomes part of the caching model.
- Future configuration and UI work should continue to keep ownership selection separate from the parameters written to DNS records.
- If future work needs ownership semantics beyond DNS comments, or shared ownership rules across DNS and WAF resources, that should be designed as a new abstraction instead of extending this selector implicitly.
