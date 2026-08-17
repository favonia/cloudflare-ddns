# Design Note: Domain Input Normalization

Read when: changing accepted domain spellings, domain validation, structured-domain diagnostics, or domain-expression diagnostics.

Defines: the 1.x compatibility, rejection, normalization, and diagnostic contracts for domain-like configuration input and their 2.0.0 transition.

Does not define: Cloudflare API behavior, trusted direct domain conversions, reconciliation, zone walking, API retries, or unrelated IDNA policy.

## 1.x Input Contract and 2.0.0 Transition

Domain-like input first passes through the repository's existing IDNA lookup mapping. The mapped dot structure is then classified. Accepted input produces one canonical value; input accepted through compatibility normalization also produces occurrence metadata that records whether leading dots or extra trailing dots were removed. The metadata is not a separate configuration value.

| input shape after IDNA mapping     | 1.x result                                               | 2.0.0 transition                      |
| ---------------------------------- | -------------------------------------------------------- | ------------------------------------- |
| one final root dot                 | accepted and removed silently                            | remains accepted and removed silently |
| one or more leading dots           | accepted after removal, with leading-dot metadata        | reject the non-canonical spelling     |
| two or more final dots             | accepted after removal, with extra-trailing-dot metadata | reject the non-canonical spelling     |
| any remaining empty interior label | reject with an empty-interior-label error                | remains rejected                      |

The root suffix is a special boundary case: `""` and `"."` are accepted silently, while all-dot suffix spellings with two or more dots are accepted with extra-trailing-dot metadata. Root and one-label target domains remain rejected as having too few labels; root and one-label suffixes remain permitted.

`*.example.org` and `*.example.org.` are accepted wildcard domains. A zero-length label in the wildcard suffix is fatal: `*..example.org`, `*...example.org`, and `*.a..example.org` produce an empty-interior-label error. When wildcard syntax is supplied where a suffix is required, it is rejected as a wildcard suffix only after its domain suffix has passed the empty-label check.

Unicode full stop (`U+3002`), fullwidth full stop (`U+FF0E`), and halfwidth ideographic full stop (`U+FF61`) are mapped by the existing IDNA lookup mapping before this classification. Consequently, their leading, trailing, and interior forms follow the same contract as ASCII dots.

## Normalization and Diagnostic Model

The model follows these rules:

1. Apply the existing IDNA lookup mapping before dot-structure classification.
2. Produce one canonical value for accepted input and occurrence metadata only for compatibility normalization.
3. Preserve the original source span and canonical effective domain when reporting accepted structured-entry occurrences; preserve the supplied syntax and encounter order when reporting accepted list or expression occurrences.
4. Add configuration-setting context only at configuration reporting time.
5. Emit no accepted-normalization diagnostic when that atom or a later field in the same structured entry is rejected.
6. Return no partial usable list or expression after a fatal domain error; structured parsing may recover only to collect later diagnostics, and a setting with any fatal diagnostic leaves its destination unchanged.

Dot-structure classification is authoritative for untrusted text in this flow. Best-effort ASCII diagnostic rendering does not validate or reinterpret input. Accepted values retain their canonical ASCII and Unicode forms for later use.

Structured-entry diagnostics preserve the underlying domain rejection reason, including whether an empty label immediately follows a wildcard marker. Each accepted compatibility normalization produces one diagnostic occurrence with nonzero metadata. Domain lists and `is(...)`/`sub(...)` expressions keep accepted canonical values for evaluation while preserving each occurrence for reporting. Configuration reporting for `DOMAINS`, `IP4_DOMAINS`, and `IP6_DOMAINS` adds the setting-specific context.

When parsing structured entries, recovery may resume at later top-level commas solely to collect more diagnostics.

## Standards Evidence Boundary

[RFC 9499](https://www.rfc-editor.org/rfc/rfc9499.html#section-2) is the standards basis for the local empty-label policy: in the global DNS, the final root label of a fully qualified name is the only label permitted to have zero length. This repository validates local input against that rule; Cloudflare's live response to malformed input has not been verified, and no Cloudflare behavior claim is needed for this policy.

## Non-Goals

This contract does not validate trusted direct conversions into domain or wildcard values. It does not change reconciliation, zone walking, Cloudflare API retries, or any other provider behavior. It also does not define broader IDNA acceptance policy beyond using the existing IDNA mapping before the local dot-shape classification.
