# Design Note: Ownership Model

Read when: changing what this updater may manage, preserve, mutate, or delete across DNS and WAF, including target-provider semantics, attribute-based ownership selectors, or deletion eligibility.

Defines: resource ownership, IP-family ownership, attribute-based ownership, and the deletion-eligibility inference.

Does not define: exact package-local data structures, exact public keyword spelling, or reconciliation step ordering.

## Goal

Give the project one first-principles answer to the question: what may this updater touch, and why?

## Core Model

For one updater run, effective mutation authority is the intersection of three ownership layers:

1. resource ownership
2. IP-family ownership
3. attribute-based ownership

Each layer answers a different question:

- resource ownership:
  - which DNS names or WAF lists are selected as managed resource roots at all
- IP-family ownership:
  - which managed content of each resource root is in scope by IP family
  - what desired-target intent exists for each in-scope family
- attribute-based ownership:
  - which remote DNS records or WAF list items this updater recognizes as its own based on selected attributes

Write-side values such as `RECORD_COMMENT` and `WAF_LIST_ITEM_COMMENT` are not ownership filters. They are self-identifying values written to new or updated objects, and they must satisfy the corresponding attribute-based ownership selectors so the updater does not orphan its own objects.

## Resource Ownership

Resource ownership starts from updater configuration itself.

- configured DNS domains select DNS resource roots
- configured WAF lists select WAF resource roots

If a resource root is not configured, it is out of scope for mutation regardless of any more specific ownership filter.

## IP-Family Ownership

`IP4_PROVIDER` and `IP6_PROVIDER` are coherent public names if `provider` is read as a family-specific provider of desired targets:

- a provider is active, not merely a passive source
- provider values may carry behavior and attributes
- `none` is a coherent sentinel provider mode even though it is not a meaningful source

IP-family ownership must distinguish:

- family scope:
  - whether this updater is responsible for managed content of that family at all
- desired-target intent:
  - what target set the updater wants when that family is in scope
- target-set availability:
  - whether the updater has a usable desired target set for this run

The semantic states are:

- Out-of-scope family intent:
  - this updater is not responsible for that family
  - steady-state reconciliation preserves existing owned content of that family
  - shutdown cleanup does not claim authority over that family
- Explicit-empty family intent:
  - this updater is responsible for that family
  - the desired target set for that family is empty
  - steady-state reconciliation drives owned content of that family to empty
  - shutdown cleanup may delete owned content of that family
- Non-empty desired-target intent:
  - this updater is responsible for that family
  - the desired target set comes from some configured target-provider mode
- Target-set unavailable for this run:
  - this updater is responsible for that family
  - the desired target set for this run is unknown
  - steady-state reconciliation preserves existing owned content because desired targets are unknown
  - shutdown authority still follows family scope, not temporary target unavailability

Any public provider mode belongs to one of these categories:

- dynamic observation provider mode
- explicit static non-empty target provider mode
- explicit static empty target provider mode
- out-of-scope sentinel

This note does not require exact public spellings for explicit static modes.

IP-family ownership is about managed content, not about ownership of the provider mechanism itself. The provider is only the public knob that expresses family scope and desired-target intent.

The provider-facing landing of the target-set portion of this model is defined in [Provider Target Validation](provider-target-validation.markdown). That note covers only families that are in scope for provider evaluation; out-of-scope remains an ownership-layer concept.

## Attribute-Based Ownership

Attribute-based ownership answers:

Which remote objects does this updater recognize as its own from selected remote attributes?

These selectors are resource-specific:

- DNS ownership is instantiated in [DNS Ownership Instantiation](managed-record-ownership.markdown)
- WAF ownership is instantiated in [WAF Ownership Instantiation](managed-waf-item-ownership.markdown)

## Deletion Eligibility

Shutdown deletion authority is a logical inference for each run from the ownership layers above plus recreatability.

A deletion target is eligible for deletion only if the updater can recreate the fully reconciled state of that same target from configuration alone.

This distinction is resource-specific:

- an individual owned DNS record may be eligible for deletion, but a broader DNS root is not, because non-owned coexisting content may always exist under the same DNS name and IP family unit.
- a WAF list may be eligible for deletion when all relevant ownership filters are widened to full coverage for the configured list and the updater can recreate the fully reconciled state of that list from configuration alone.

## Consequences

DNS and WAF should consume this model consistently.

- Out-of-scope family intent means preserve that family.
- Explicit-empty family intent means reconcile that family to empty.
- Target-set unavailability means preserve because the updater lacks the desired target set for this run.
- Deletion eligibility comes from ownership, not from whether observation succeeded.

The reconciliation algorithm that consumes this ownership result is defined in [Reconciliation Algorithm](reconciliation-algorithm.markdown).

## Extension Points

- If the public interface later separates family enablement from target-provider selection, the ownership model remains the same.
- If future work adds more provider families or provider attributes, they should map into this note's semantic states instead of inventing new cleanup rules ad hoc.
- If future resources beyond DNS and WAF are added, they should define:
  - their resource ownership unit
  - their IP-family ownership consequences, if any
  - their attribute-based ownership selectors, if any
  - which deletion targets are eligible by recreatability instead of inventing separate scope rules ad hoc.
