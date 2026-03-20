# Design Note: DNS Ownership Instantiation

Read when: changing DNS ownership, managed-record filtering, or DNS reconciliation semantics tied to DNS record ownership.

Defines: the DNS instantiation of the ownership model, including DNS attribute-based ownership via `MANAGED_RECORDS_COMMENT_REGEX` and `RECORD_COMMENT`, plus ownership-aware DNS reconciliation.

Does not define: exact Cloudflare request payload shapes or local warning text.

`MANAGED_RECORDS_COMMENT_REGEX` lets one updater instance decide which DNS records it recognizes as its own.

## Goal

Safely isolate DNS record ownership when multiple updater instances may touch overlapping DNS names. This note defines the DNS attribute-based ownership layer inside the ownership model.

## Core Model

- `RECORD_COMMENT` is the fallback comment this instance uses when reconciling DNS records.
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

When DNS reconciliation needs create metadata, it resolves that metadata per `(domain, record type)` unit from recyclable managed records only.

For scalar DNS metadata fields (`TTL`, `PROXIED`, `RECORD_COMMENT`), DNS uses the shared reconciliation rule from [Reconciliation Algorithm](reconciliation-algorithm.markdown).

`TAGS` uses the same rule per individual tag instead of per whole field:

- tag names are compared case-insensitively
- tag values are compared case-sensitively
- a tag is inherited only if every recyclable managed record has that canonical tag
- otherwise the fallback for that tag is used

With today's exposed config surface, the fallback tag set is empty, so DNS tag reconciliation reduces to the canonical intersection/common subset of recyclable managed records.

### Interruption-Aware Priority

DNS refines the shared residual-risk policy with these tiers:

- `R0`: missing desired target satisfaction
- `R1`: stale managed records still pointing to non-desired targets
- `R2a`: proxied mismatch (expected `PROXIED=false`, actual `true`)
- `R2b`: proxied mismatch (expected `PROXIED=true`, actual `false`)
- `R2c`: TTL drift
- `R2d`: comment/tags drift
- `R3`: duplicate or hygiene residue

### Failure and Shutdown Semantics

DNS uses the shared family-intent semantics from [Lifecycle Model](lifecycle-model.markdown).

For DNS, the deletion target is an individual managed record, not a broader DNS root. DNS shutdown may therefore delete only managed records.

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
