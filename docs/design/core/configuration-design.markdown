# Design Note: Configuration Design

Read when: adding or changing configuration inputs, defaults, validation, normalization, diagnostics, or runtime admission.

Defines: the project-wide semantic contract for turning operator configuration into runtime configuration.

Feature notes define individual settings and their syntax, defaults, normalization, diagnostics, and local invariants. [Codebase Architecture](codebase-architecture.markdown#configuration-lifecycle) maps this contract to packages and startup code.

## Operator Intent and Defaults

Treat every explicit configuration as meaningful operator intent. Honor it or report why it cannot hold; never silently discard it.

Every implicit default must be semantically identical to an explicit value, so the operator can restate the default choice without changing behavior. The explicit value may name a mode such as `auto` or `inherit`; it need not spell the concrete result later derived from that mode.

## Validation Boundaries

Detect operator misconfigurations at the earliest boundary with enough information to establish the error. Parse and type errors belong at the input boundary, setting-specific semantic errors belong where that setting is assembled, cross-setting errors belong before runtime configuration is admitted, and conditions that depend on runtime observations belong where those observations first become available.

Normalization accepts spellings allowed by the feature contract and converts them to a canonical value. It must not repair rejected input into accepted configuration.

## Runtime Admission

The complete configuration being validated is a candidate. Runtime code may use it only after all configuration-time checks succeed, including setting-specific and cross-setting invariants. Any configuration error rejects the whole candidate.

Parsing may recover after an error to collect additional diagnostics, but the recovered results must not become a partial runtime configuration.

## Diagnostics

Diagnostics must identify which value or setting they describe. A warning about an accepted value must not imply that the complete candidate has passed validation.
