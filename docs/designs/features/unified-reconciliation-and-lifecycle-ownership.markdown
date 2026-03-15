# Design Note: Unified Reconciliation and Lifecycle Ownership

Read when: changing shared DNS/WAF reconciliation semantics, interruption-risk policy, or shutdown authority rules across managed resources.

Defines: the cross-resource semantic model for ownership scope, desired-target reconciliation, residual-risk minimization, and lifecycle ownership.

Does not define: exact public keyword spelling, resource-specific metadata fields, or package-local mutation stages.

## Goal

Give DNS and WAF one shared semantic model so resource-specific notes only need to define how that model is instantiated.

## Shared Inputs

Every reconciler run is defined by these inputs:

- ownership scope: which existing objects this updater may touch
- family-scope and desired-target semantics from [IP Family Intent and Target Providers](ip-family-intent-and-target-providers.markdown)
- resource-specific matching semantics
- lifecycle ownership of the managed resource root

## Unified Reconciliation Model

For one managed resource unit:

1. keep all owned objects that already satisfy at least one desired target,
2. use remaining owned objects as recyclable material for uncovered desired targets,
3. create new objects only when recycling cannot satisfy uncovered desired targets,
4. delete leftover owned objects that satisfy no desired target.

This is a satisfier, not a full canonicalizer.

- Already-satisfying objects are preserved unless a resource-specific note says otherwise.
- Metadata for new creates is resolved from recyclable owned objects, not from already-satisfying ones.
- Duplicate or overlapping residue may remain when it still satisfies desired targets.

Resource-specific notes define:

- the resource unit
- what it means for an object to satisfy a desired target
- which metadata fields exist
- any stricter contracts that intentionally override the default softness above

## Residual-Risk Policy

Under ambiguous partial execution, implementations should minimize residual risk rather than follow one fixed mutation script.

The shared priority order is:

- missing desired satisfaction or coverage
- stale owned content still affecting non-desired targets
- metadata drift on otherwise-satisfying owned content
- duplicate or hygiene residue

Resource-specific notes may refine the tiers, but should not invert this order without explicit justification.

## Lifecycle Ownership

Shutdown authority is defined by lifecycle ownership of the resource root.

- Member-owned resources may delete only owned members in active family scope.
- Root-owned resources may delete the whole resource root.

A resource root is only root-owned for a run when:

- the updater can recreate that root from updater configuration alone, and
- the current cleanup scope covers all semantic content the updater owns within that root.

If either condition fails, the resource is member-owned for that run.

## Extension Points

- If future resources beyond DNS and WAF are added, they should instantiate this model rather than inventing separate reconciliation rules ad hoc.
- If the public interface later separates family enablement from target-provider selection, the shared model here remains unchanged.
