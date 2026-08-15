# Design Note: Domain Input Normalization

Read when: changing accepted domain spellings, domain-constructor validation, structured-domain diagnostics, or domain-expression diagnostics.

Defines: the 1.x compatibility and rejection contract for domain-like configuration input, its 2.0.0 transition, and the package ownership of canonical values and diagnostics.

Does not define: Cloudflare API behavior, trusted direct domain conversions, reconciliation, zone walking, API retries, or unrelated IDNA policy.

## 1.x Input Contract and 2.0.0 Transition

The constructors first apply the repository's existing IDNA lookup mapping, then classify the mapped dot structure and return a canonical value plus `domain.Normalization` metadata where the spelling is accepted for compatibility. The metadata records `RemovedLeadingDots` and `RemovedExtraTrailingDots`; it is not a separate configuration value.

| input shape after IDNA mapping     | 1.x result                                               | 2.0.0 transition                      |
| ---------------------------------- | -------------------------------------------------------- | ------------------------------------- |
| one final root dot                 | accepted and removed silently                            | remains accepted and removed silently |
| one or more leading dots           | accepted after removal, with leading-dot metadata        | reject the non-canonical spelling     |
| two or more final dots             | accepted after removal, with extra-trailing-dot metadata | reject the non-canonical spelling     |
| any remaining empty interior label | reject as `domain.ErrEmptyInteriorLabel`                 | remains rejected                      |

The root suffix is a special boundary case: `""` and `"."` are accepted silently, while all-dot suffix spellings with two or more dots are accepted with extra-trailing-dot metadata. `New` still rejects root and one-label target domains with `domain.ErrTooFewLabels`; `NewSuffix` permits root and one-label suffixes.

`*.example.org` and `*.example.org.` are accepted wildcard domains. A zero-length label in the wildcard suffix is fatal: `*..example.org`, `*...example.org`, and `*.a..example.org` return `domain.ErrEmptyInteriorLabel`. A wildcard suffix remains a wildcard-specific `NewSuffix` rejection only after its suffix has passed the empty-label check.

Unicode full stop (`U+3002`), fullwidth full stop (`U+FF0E`), and halfwidth ideographic full stop (`U+FF61`) are mapped by the existing IDNA path before this classification. Consequently, their leading, trailing, and interior forms follow the same contract as ASCII dots.

## Ownership and Diagnostic Routing

`internal/domain` owns construction, canonical ASCII/Unicode storage, boundary-normalization metadata, `ErrEmptyInteriorLabel`, and the wildcard-marker detail attached to that error. `New` and `NewSuffix` are the only dot-shape parsers for untrusted text in this flow. `StringToASCII` remains best-effort diagnostic rendering, not a validation API.

`internal/domainentry` owns the structured-entry parser and its source spans. It preserves constructor errors in `Diagnostic.Detail`; for an accepted normalized entry, it emits one `KindDomainBoundaryNormalization` diagnostic with the exact source span, nonzero metadata, and canonical effective domain. It emits no normalization diagnostic when the domain or a later structured field is rejected.

`internal/domainexp` owns parse-and-report domain lists and `is(...)`/`sub(...)` expressions. It retains each accepted normalization occurrence in encounter order, renders it in the syntax the operator supplied, and keeps canonical constructor values for later expression evaluation. `internal/config` owns environment-setting routing: it consumes structured entry diagnostics for `DOMAINS`, `IP4_DOMAINS`, and `IP6_DOMAINS`, and it preserves the setting-specific context when reporting them.

Fatal input never yields a usable value on the calling path. `domainexp.ParseList` and `domainexp.ParseExpression` return no partial result on a fatal domain error. The structured parser may recover at later top-level commas to report more diagnostics, but `config.readDomains` fails the setting and leaves its destination field unchanged when any fatal diagnostic is present. Accepted normalization metadata is emitted only for an atom that produced an accepted value.

## Standards Evidence Boundary

[RFC 9499](https://www.rfc-editor.org/rfc/rfc9499.html#section-2) is the standards basis for the local empty-label policy: in the global DNS, the final root label of a fully qualified name is the only label permitted to have zero length. This repository validates local input against that rule; Cloudflare's live response to malformed input has not been verified, and no Cloudflare behavior claim is needed for this policy.

## Non-Goals

This contract does not validate trusted direct `domain.FQDN(...)` or `domain.Wildcard(...)` conversions. It does not change reconciliation, zone walking, Cloudflare API retries, or any other provider behavior. It also does not define broader IDNA acceptance policy beyond using the existing IDNA mapping before the local dot-shape classification.
