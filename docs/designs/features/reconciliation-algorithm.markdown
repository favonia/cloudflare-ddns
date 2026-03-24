# Design Note: Reconciliation Algorithm

Read when: changing reconciliation semantics or interruption-risk policy across managed resources.

Defines: the cross-resource reconciliation algorithm and residual-risk minimization policy.

Does not define: ownership selection, shutdown authority, exact public keyword spelling, or resource-specific metadata fields.

## Goal

Give DNS and WAF one reconciliation algorithm so resource-specific notes only need to define how that algorithm is instantiated.

## Inputs

Every reconciler run is defined by these inputs:

- the reconciliation intent from [Lifecycle Model](lifecycle-model.markdown)
- the effective ownership result from [Ownership Model](ownership-model.markdown)
- resource-specific matching semantics

## Intent Handling

The reconciliation intent determines the overall action for one managed resource unit:

- `preserve`: keep all existing managed content unchanged (the resource is out of scope)
- `abort`: keep all existing managed content unchanged (the resource is in scope but raw data was unavailable for this run)
- `clear`: proceed to the core model with an empty desired target set
- `update`: proceed to the core model with the derived desired target set

This algorithm treats `preserve` and `abort` identically, but they are kept apart because a different algorithm could reasonably handle them differently (e.g., clearing stale content on `abort` after a timeout).

## Core Model

For one managed resource unit:

1. keep all owned objects that already satisfy at least one desired target,
2. use remaining owned objects as recyclable material for uncovered desired targets,
3. create new objects only when recycling cannot satisfy uncovered desired targets,
4. delete leftover owned objects that satisfy no desired target.

This is a satisfier, not a full canonicalizer.

- Already-satisfying objects are preserved unless a resource-specific note says otherwise.
- Metadata drift on otherwise-satisfying objects stays soft unless a resource-specific note says otherwise.
- Metadata for new creates is resolved from recyclable owned objects, not from already-satisfying ones.
- Duplicate or overlapping residue may remain when it still satisfies desired targets.

Resource-specific notes define:

- the resource unit
- what it means for an object to satisfy a desired target
- which metadata fields exist
- any stricter contracts that intentionally override the default softness above

## Metadata Resolution for New Creates

When reconciliation needs metadata for newly created objects, it resolves that metadata from recyclable owned objects only.

- An empty source set uses the configured fallback value.
- A unanimous source value is inherited.
- A non-unanimous source value uses the configured fallback value and emits one ambiguity warning for that field.

Set-valued metadata fields may apply the same rule per element instead of per whole field. In that case, each element is inherited only when its source values agree under the resource-specific equality rule; otherwise the fallback for that element is used.

## Residual-Risk Policy

Under ambiguous partial execution, implementations should minimize residual risk rather than follow one fixed mutation script.

The priority order is:

- missing desired satisfaction or coverage
- stale owned content still affecting non-desired targets
- metadata drift on otherwise-satisfying owned content
- duplicate or hygiene residue

Resource-specific notes may refine the tiers, but should not invert this order without explicit justification.

## Extension Points

- If future resources beyond DNS and WAF are added, they should instantiate this algorithm rather than inventing separate reconciliation rules ad hoc.
- If the ownership model later grows new filter layers, this reconciliation algorithm still consumes only the final effective ownership result.
