# Design Note: Ownership Model

Read when: changing what this updater may manage, preserve, mutate, or delete across DNS and WAF, including provider raw-data semantics, attribute-based ownership selectors, or deletion eligibility.

Defines: the current ownership predicates and the deletion-eligibility inference.

Does not define: exact package-local data structures, exact public keyword spelling, or reconciliation step ordering.

## Goal

Give the project one first-principles answer to the question: what may this updater touch, and why?

## Core Model

Ownership is the intersection of static yes-or-no predicates.

This project currently defines three predicates:

- resource ownership
- IP-family ownership
- attribute-based ownership

## Resource Ownership

This predicate selects which DNS names or WAF lists are managed resource roots at all.

- configured DNS domains select DNS resource roots
- configured WAF lists select WAF resource roots

If a resource root is not configured, it is out of scope for mutation regardless of any more specific ownership filter.

## IP-Family Ownership

This predicate selects which managed content of each resource root is in scope by IP family.

`IP4_PROVIDER=none` or `IP6_PROVIDER=none` means that family is out of scope. Any other provider mode means that family is in scope.

## Attribute-Based Ownership

This predicate selects which remote DNS records or WAF list items the updater recognizes as its own from selected attributes.

Write-side values such as `RECORD_COMMENT` and `WAF_LIST_ITEM_COMMENT` must satisfy the corresponding attribute-based ownership selectors so the updater does not orphan its own objects.

Selectors are resource-specific:

- DNS ownership is instantiated in [DNS Ownership Instantiation](managed-record-ownership.markdown)
- WAF ownership is instantiated in [WAF Ownership Instantiation](managed-waf-item-ownership.markdown)

## Deletion Eligibility

A deletion target is eligible for shutdown deletion only if the updater can recreate the fully reconciled state of that same target from configuration alone.

## Extension Points

- If future work changes how family scope is configured, preserve the IP-family predicate instead of moving runtime target-state detail back into the ownership layer.
- If future resources beyond DNS and WAF are added, they should define:
  - their resource ownership unit
  - their attribute-based ownership selectors, if any
  - which deletion targets are eligible by recreatability instead of inventing separate scope rules ad hoc.
- If future work adds more ownership predicates, they should be added here as additional yes-or-no predicates instead of overloading the existing ones.
