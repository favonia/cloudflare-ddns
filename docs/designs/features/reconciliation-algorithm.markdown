# Design Note: Reconciliation Algorithm

Read when: changing reconciliation semantics or interruption-risk policy across managed resources.

Defines: the cross-resource reconciliation algorithm and residual-risk minimization policy.

Does not define: ownership selection, shutdown authority, exact public keyword spelling, or resource-specific metadata fields.

## Goal

Give DNS and WAF one reconciliation algorithm so resource-specific notes only need to define how that algorithm is instantiated.

## Inputs

Every reconciler run is defined by these inputs:

- the effective ownership result from [Ownership Model](ownership-model.markdown)
- resource-specific matching semantics

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
