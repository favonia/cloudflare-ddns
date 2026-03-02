# Design Note: Managed DNS Record Ownership

`MANAGED_RECORDS_COMMENT_REGEX` lets one updater instance decide which existing DNS records it owns.

## Goal

Safely isolate ownership when multiple updater instances may touch overlapping DNS names. Ownership affects record discovery, updates, duplicate cleanup, and deletion.

## Core Model

- `RECORD_COMMENT` is the comment this instance writes to DNS records that it creates or updates.
- `MANAGED_RECORDS_COMMENT_REGEX` is the selector used to decide which existing DNS records are managed by this instance.
- These settings are intentionally separate: one controls what this instance writes, and the other controls what it may mutate.

The selector uses Go `regexp` RE2 syntax with `MatchString` semantics. It is not an implicit full-match pattern.

The empty default matches all comments, preserving pre-feature behavior. Ownership isolation is opt-in.

## Required Invariants

- `MANAGED_RECORDS_COMMENT_REGEX` is compiled during configuration normalization and stored in canonical form.
- After successful normalization, the compiled regex is always non-nil, including the default empty template.
- `RECORD_COMMENT` must match `MANAGED_RECORDS_COMMENT_REGEX`.

The last rule prevents self-orphaning.

## Reconciliation Semantics

Managed-record filtering happens immediately after listing DNS records from Cloudflare.

Only matched records participate in:

- IP parsing
- TTL, proxied, and comment drift warnings
- stale-record detection
- duplicate cleanup

Unmatched records are invisible to DNS mutation logic. As a result, the updater may create a new managed record even if an unmanaged record already has the desired IP address.

## Shutdown Deletion Semantics

With `DELETE_ON_STOP`, shutdown cleanup deletes only managed DNS records matched by `MANAGED_RECORDS_COMMENT_REGEX`.

The updater does not assume whole-domain ownership. Unmatched DNS records remain outside the deletion scope.

## Caching Contract

Record-list caches store already-filtered managed records.

This relies on one handle and its bound setter using one stable managed-record filter for their lifetime. The current cache key does not include filter identity.

## Tradeoffs

- The design prefers strict ownership isolation over reusing foreign records. This may leave parallel records with the same IP address, but it avoids mutating another deployment's records.
- Regex selectors allow flexible grouping, but exact ownership boundaries require explicit anchors such as `^managed-by-a$`.
- The selector name is intentionally distinct from `RECORD_COMMENT` to reduce operator confusion.

## Naming Notes

`MANAGED_RECORDS_COMMENT_REGEX` was kept instead of a name closer to `RECORD_COMMENT`.

The main reason is operator safety: the selector is about management scope, not the default comment for newly written records. A name that is too close to `RECORD_COMMENT` increases the risk of copy-paste and scanning mistakes in environment-variable-heavy setups.

## Scope Boundary

This design applies only to DNS record ownership based on DNS record comments.

It is not a general ownership abstraction for all managed resources. WAF list item ownership remains separate, and DNS-less or WAF-only runs do not use this selector.

## Future Development Notes

- If one process ever needs multiple ownership scopes for the same domain and IP family, the cache design must change so filter identity becomes part of the caching model.
- Future configuration and UI work should continue to keep ownership selection separate from the parameters of newly created DNS records.
- If future work needs ownership semantics beyond DNS comments, or shared ownership rules across DNS and WAF resources, that should be designed as a new abstraction instead of extending this selector implicitly.
