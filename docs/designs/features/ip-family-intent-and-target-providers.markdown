# Design Note: IP Family Intent and Target Providers

Read when: changing `IP4_PROVIDER` / `IP6_PROVIDER` semantics, family scope, desired-target intent, target-provider structure, or shutdown behavior derived from IP-family intent.

Defines: the semantic states that public IP-family configuration must express, independent of the exact keyword spelling used in the public interface.

Does not define: exact package-local data structures.

## Goal

Define IP-family semantics from first principles so DNS, WAF, and shutdown behavior share one user model.

This note also explains the public-interface role of `IP4_PROVIDER` / `IP6_PROVIDER`.

## Public Interface Framing

`IP4_PROVIDER` and `IP6_PROVIDER` are coherent public names if "provider" is read as:

- a family-specific provider of desired targets
- not merely a raw source URL or source machine
- not a DNS-service provider (Cloudflare is fixed for this project)

The distinction matters:

- a source sounds passive, as if the updater only points at where targets come from
- a provider sounds active, as if the updater selects a strategy or mode that supplies targets

This project needs the latter framing. Provider values can carry behavior and attributes, not just origin. For example, `local.iface:<iface>` carries an interface parameter, `url.via4:<url>` carries transport behavior, and `none` works naturally as a sentinel provider mode even though it is not a meaningful "source".

Under that framing, a provider value may represent:

- a dynamic observation strategy such as `cloudflare.trace`, `cloudflare.doh`, or `local`
- a parameterized target-provider mode such as `local.iface:<iface>` or `url:<url>`
- an explicit target-provider mode that supplies a fixed target set directly
- a sentinel provider value such as `none`

This framing is intentional because target providers may grow richer attributes over time. For example, one provider family may later carry interface names, transport overrides, source-address binding, or other provider-specific parameters without forcing the public model to split prematurely into many separate knobs.

The semantic model below is therefore independent of whether the public interface stays as one `IP*_PROVIDER` knob or is later split into separate public knobs.

## Core Model

For each IP family, the updater must distinguish:

- family scope: whether this updater is responsible for that family at all
- desired-target intent: what target set the updater wants for that family when it is in scope
- target-set availability: whether the updater has a usable desired target set for this run

The first two are configuration-level semantics and are expected to remain stable until configuration changes.

The third is a runtime, per-run, temporary state. It may vary from one update cycle to the next even when configuration is unchanged.

These are separate concepts even when the public interface compresses them into one configuration surface such as `IP*_PROVIDER`.

## Semantic States

The semantic model needs these user-visible states:

- Out-of-scope family intent:
  - this updater is not responsible for that family
  - steady-state reconciliation preserves existing managed content of that family
  - shutdown cleanup does not claim authority over that family
- Explicit-empty family intent:
  - this updater is responsible for that family
  - the desired target set for that family is empty
  - steady-state reconciliation drives managed content of that family to empty
  - shutdown cleanup may delete managed content of that family
- Non-empty desired-target intent:
  - this updater is responsible for that family
  - the desired target set comes from some configured target provider mode
- Target-set unavailable for this run:
  - this updater is responsible for that family
  - the desired target set for this run is unknown
  - steady-state reconciliation preserves existing managed content because desired targets are unknown
  - shutdown authority still follows family scope, not temporary target unavailability

## Target Provider Modes

Any public configuration value or provider mode that supplies desired targets belongs to one of these categories:

- dynamic observation provider mode
- explicit static non-empty target provider mode
- explicit static empty target provider mode
- out-of-scope sentinel

This note intentionally does not require exact public spellings for explicit static provider modes. Keyword choices such as the non-empty and explicit-empty static spellings belong to public-interface documentation, not to this semantic note.

## Shared Consequences

DNS and WAF should consume these states consistently.

- Out-of-scope family intent means preserve that family.
- Explicit-empty family intent means reconcile that family to empty.
- Target-set unavailability means preserve because the updater lacks the desired target set for this run.
- Cleanup authority comes from responsibility for the family, not from whether observation succeeded.

## Extension Points

- If the public interface later separates family enablement from target-provider selection, this semantic model remains the same.
- If future work adds more provider families or provider attributes, they should map into this note's semantic states instead of inventing new cleanup rules ad hoc.
