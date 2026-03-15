# Design Note: DNS Ownership Instantiation

Read when: changing DNS ownership, managed-record filtering, or DNS reconciliation semantics tied to DNS record ownership.

Defines: the DNS instantiation of the ownership model, including DNS attribute-based ownership via `MANAGED_RECORDS_COMMENT_REGEX` and `RECORD_COMMENT`, plus ownership-aware DNS reconciliation.

Does not define: exact Cloudflare request payload shapes or local warning text.

`MANAGED_RECORDS_COMMENT_REGEX` lets one updater instance decide which DNS records it recognizes as its own.

## Goal

Safely isolate DNS record ownership when multiple updater instances may touch overlapping DNS names. This note defines the DNS attribute-based ownership layer inside the ownership model.

## Core Model

- `RECORD_COMMENT` is the comment this instance writes to DNS records that it creates or updates.
- `MANAGED_RECORDS_COMMENT_REGEX` is the attribute-based selector used to decide which DNS records are managed by this instance.
- These settings are intentionally separate: one controls what this instance writes, and the other controls what it may mutate.

Within the ownership model:

- resource ownership is defined elsewhere
- IP-family ownership is defined in [Ownership Model](ownership-model.markdown)
- this note defines DNS attribute-based ownership
- reconciliation semantics are defined in [Reconciliation Algorithm](reconciliation-algorithm.markdown)

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
- target satisfaction checks
- stale-record detection
- metadata derivation for new creates
- `DELETE_ON_STOP`

Unmatched records are invisible to DNS mutation logic, so the updater may create a new managed record even if an unmanaged record already has the desired IP address.

### DNS Instantiation

DNS instantiates the reconciliation algorithm with these resource-specific rules:

- the resource unit is `(domain, IP family)`
- a managed record satisfies a desired target when its record IP equals that desired target IP
- matching duplicate managed records may remain
- duplicate multiplicity is tolerated residue, not desired state
- already-satisfying record metadata is soft unless another DNS-specific contract overrides it

### Metadata for New Creates

When DNS reconciliation needs to satisfy uncovered targets, metadata is resolved per `(domain, record type)` unit from recyclable managed records only.

Recycling is only an optimization of delete-and-create to reduce disruption; the target metadata always comes from reconciled recyclable sources, not from already-matching records.

- Scalar fields (`TTL`, `PROXIED`, `RECORD_COMMENT`):
  - empty source set: use configured value
  - unanimous source value: inherit source value
  - non-unanimous source values: use configured value and emit one ambiguity warning per field
- Tag field (`TAGS`):
  - tag name is compared case-insensitively
  - tag value is compared case-sensitively
  - configured-default tags are sticky unless all sources omit them
  - non-default tags require unanimity across sources to be inherited

### Interruption-Aware Priority

DNS reconciliation should minimize residual risk under ambiguous partial execution.

The intended DNS risk tiers are:

- `R0`: missing desired target satisfaction
- `R1`: stale managed records still pointing to non-desired targets
- `R2a`: proxied mismatch (expected `PROXIED=false`, actual `true`)
- `R2b`: proxied mismatch (expected `PROXIED=true`, actual `false`)
- `R2c`: TTL drift
- `R2d`: comment/tags drift
- `R3`: duplicate or hygiene residue

Any implementation should order work so higher-risk residual states are reduced before lower-risk ones.

This note intentionally records risk order, not one exact stage decomposition.

### Failure and Shutdown Semantics

When the shared IP-family ownership semantics from [Ownership Model](ownership-model.markdown) are applied to DNS:

- Out-of-scope family intent preserves existing managed records of that family.
- Explicit-empty family intent reconciles that family to no managed records.
- Temporary target-set unavailability preserves existing managed records because desired targets are unknown.

For DNS, the deletion target is an individual managed record, not a broader DNS root. DNS shutdown may therefore delete only managed records. This is an inferred consequence of the ownership model: non-owned coexisting DNS content may always exist under the same domain and IP-family unit, so a broader DNS root is never eligible for deletion.

### API Contract Boundary

`setter` and `api.Handle` use the following DNS mutation contract:

- `UpdateRecord` reconciles one managed record to desired state for both:
  - content/IP
  - metadata in scope (`TTL`, `PROXIED`, `RECORD_COMMENT`, `TAGS`)
- desired-state mutation source is `desiredParams`

This contract is intentionally explicit. Any future contract change here should update interface comments, implementation comments, and API write tests together.

## Caching Contract

Record-list caches store already-filtered managed records.

This requires one handle and its bound setter to use one stable managed-record filter for their lifetime. The current cache key does not include filter identity.

## Tradeoffs

- The design prefers strict ownership isolation over reusing foreign records. This may leave parallel records with the same IP address, but it avoids mutating another deployment's records.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `RECORD_COMMENT` to reduce operator confusion.

## Scope Boundary

This design applies only to DNS record ownership based on DNS record comments.

## Extension Points

- If one process ever needs multiple ownership scopes for the same domain and IP family, the cache design must change so filter identity becomes part of the caching model.
- Future configuration and UI work should continue to keep ownership selection separate from the parameters written to DNS records.
- If future work changes the broader ownership model, this note should continue to own only the DNS attribute-based ownership layer instead of absorbing unrelated ownership rules.
