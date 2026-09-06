# Design Note: Domain Input Normalization

Read when: changing accepted domain spellings, domain validation, structured-domain diagnostics, or domain-expression diagnostics.

Defines: the 1.x compatibility, rejection, normalization, and diagnostic contracts for domain-like configuration input and their 2.0.0 transition.

This contract applies to untrusted domain text from configuration. Trusted direct domain or wildcard conversions are outside its scope; the existing IDNA mapping policy is unchanged.

## 1.x Input Contract and 2.0.0 Transition

Domain-like input first passes through the repository's existing IDNA lookup mapping. The mapped dot structure is then classified. Accepted input produces one canonical value.

| input shape after IDNA mapping     | 1.x result                                | 2.0.0 transition                      |
| ---------------------------------- | ----------------------------------------- | ------------------------------------- |
| one final root dot                 | accepted and removed silently             | remains accepted and removed silently |
| one or more leading dots           | accepted after removal, with a warning    | reject the non-canonical spelling     |
| two or more final dots             | accepted after removal, with a warning    | reject the non-canonical spelling     |
| any remaining empty interior label | reject with an empty-interior-label error | remains rejected                      |

For root suffixes, `""` and `"."` are accepted silently; spellings consisting of two or more dots are accepted with an extra-trailing-dot warning. Root and one-label suffixes are permitted, while root and one-label target domains are rejected as having too few labels.

`*.example.org` and `*.example.org.` are accepted wildcard domains. A zero-length label in the wildcard suffix is fatal: `*..example.org`, `*...example.org`, and `*.a..example.org` produce an empty-interior-label error. When wildcard syntax is supplied where a suffix is required, it is rejected as a wildcard suffix only after its domain suffix has passed the empty-label check.

Unicode full stop (`U+3002`), fullwidth full stop (`U+FF0E`), and halfwidth ideographic full stop (`U+FF61`) are mapped by the existing IDNA lookup mapping before this classification. Consequently, their leading, trailing, and interior forms follow the same contract as ASCII dots.

## Domain Parsing and Diagnostics

Domain parsing follows the shared [Configuration Design](../core/configuration-design.markdown) contract.

Diagnostic rendering must preserve the parser's classification, even when it uses best-effort ASCII conversion. Accepted values retain both canonical ASCII and Unicode forms.

### Information Needed for Diagnostics

For each accepted compatibility normalization, record whether leading dots or extra trailing dots were removed. Keep this metadata separate from the canonical configuration value. When the setting is accepted, report one diagnostic per normalization occurrence. When fatal errors are present, apply the warning tradeoff in [Configuration Design](../core/configuration-design.markdown#diagnostics).

Preserve the information needed to explain each accepted normalization:

- For structured domain entries, preserve the original source span and the canonical domain.
- For domain lists and expressions, preserve the supplied syntax and report occurrences in input order. Keep the canonical values for evaluation, including in `is(...)` and `sub(...)` expressions.

Structured-entry diagnostics preserve the underlying domain rejection reason, including whether an empty label immediately follows a wildcard marker.

### Rejection and Recovery

A fatal domain error produces no partial usable list or expression. A setting with any fatal diagnostic leaves its destination unchanged. Structured parsing may resume at later top-level commas solely to collect additional diagnostics.

## Standards Basis

[RFC 9499](https://www.rfc-editor.org/rfc/rfc9499.html#section-2) permits an empty label only for the final root label of a fully qualified name in the global DNS. This is the basis for rejecting empty interior labels.
