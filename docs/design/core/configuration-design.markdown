# Design Note: Configuration Design

Read when: adding or changing configuration inputs, defaults, validation, normalization, diagnostics, or runtime admission.

Defines: the project-wide semantic contract for turning operator configuration into runtime configuration.

Does not define: the syntax or default of an individual setting, package ownership, composition-root wiring, or failures that arise only after configuration has been accepted.

## Operator Intent and Defaults

Treat every explicit configuration as meaningful operator intent. Honor it or report why it cannot hold; never silently discard it.

Every implicit default must be semantically identical to an explicit value, so the operator can restate the default choice without changing behavior. The explicit value may name a mode such as `auto` or `inherit`; it need not spell the concrete result later derived from that mode.

## Validation Boundaries

Detect operator misconfigurations at the earliest boundary with enough information to establish the error. Parse and type errors belong at the input boundary, setting-specific semantic errors belong where that setting is assembled, cross-setting errors belong before runtime configuration is admitted, and conditions that depend on runtime observations belong where those observations first become available.

Normalization is an acceptance rule, not error recovery. Only spellings explicitly admitted by a feature contract may produce a canonical value.

## Diagnostics

Diagnostics must make acceptance status unambiguous. A warning may accompany accepted configuration; an error rejects the candidate. Do not report accepted normalization for a value or enclosing setting that is rejected.

## Runtime Admission

Configuration input is a candidate until all configuration-time validation succeeds. Runtime code receives configuration only after the complete candidate satisfies its setting-specific and cross-setting invariants. Any configuration error prevents runtime admission; no partial runtime configuration is admitted.

Parsing may recover after an error to collect additional diagnostics. Diagnostic recovery must not make a rejected value, setting, list, expression, or candidate available for runtime use.

## Feature Contracts

Feature design notes define their accepted syntax, defaults, normalization, diagnostics, and local invariants within this contract.
