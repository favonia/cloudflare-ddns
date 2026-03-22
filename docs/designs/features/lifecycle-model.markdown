# Design Note: Lifecycle Model

Read when: changing how one updater run starts, detects raw data, derives resource-specific targets, reconciles managed state, or cleans up on shutdown.

Defines: the umbrella phase model for one updater process and one update round.

Does not define: exact package-local call graphs or the resource-specific reconciliation rules.

## Goal

Give the project one top-level view of how updater work flows over time.

## Scope

This note covers two related timelines:

- process lifecycle: startup, scheduled waiting, shutdown
- update-round lifecycle: detection, derivation, reconciliation

The same process may execute many update rounds before one shutdown cleanup.

## Core Model

The updater is organized as these phases:

1. startup
2. waiting and triggering
3. detection
4. derivation
5. reconciliation
6. cleanup

Some phases happen once per process, some happen once per update round, and some happen only during shutdown.

## Startup

Startup prepares the runtime so later update rounds can use one stable set of validated dependencies.

Startup includes:

- output setup
- reporter setup
- raw-config reading and validation
- runtime-config construction
- API handle construction
- setter construction
- API-token usability verification

The main startup boundary is owned by [Codebase Architecture](../core/codebase-architecture.markdown). Ownership and reconciliation notes assume startup has already produced a valid runtime configuration.

## Waiting and Triggering

Between update rounds, the process waits for the next trigger from startup policy or cron scheduling.

This phase covers immediate start, future scheduling, and shutdown requests.

## Detection

Detection obtains the raw per-family data for the current round.

Detection lands family intent for the current round in raw form.

If a family is out of scope, there is only one intent: preserve existing managed content of that family.

If a family is in scope, detection yields one of three raw-data states:

- raw data unavailable for this run
- known empty raw data
- known non-empty raw data

The concrete raw-data representation is an implementation choice, not an axiom of this lifecycle model.

Detection semantics and provider contracts are owned by [Provider Raw-Data Contract](provider-raw-data-contract.markdown). Detection trust assumptions are owned by [Network Security Model](network-security-model.markdown).

## Derivation

Derivation transforms raw data into the resource-specific target shape consumed by reconciliation.

Today, the semantic raw data is a CIDR set.

DNS derivation turns each raw CIDR into a DNS address target by forgetting the prefix length.

WAF derivation turns each raw CIDR into a WAF prefix target by taking its subnet.

The default interpretation of bare IPv6 observations is owned by [IPv6 Default Prefix Policy](ipv6-default-prefix-policy.markdown).

The current code realizes only the canonical singleton special case of this model, so those derivations are currently implemented through an address-only representation instead of an explicit CIDR representation.

Future work may insert non-identity derivation without changing the surrounding lifecycle.

## Reconciliation

Reconciliation consumes the derived resource-specific target state plus the effective ownership result and mutates managed remote state toward the desired result.

The shared reconciliation algorithm is owned by [Reconciliation Algorithm](reconciliation-algorithm.markdown). Resource-specific ownership filters and instantiations are owned by [Ownership Model](ownership-model.markdown), [DNS Ownership Instantiation](managed-record-ownership.markdown), and [WAF Ownership Instantiation](managed-waf-item-ownership.markdown).

## Cleanup

Cleanup is shutdown-time mutation after the process has decided to stop.

Cleanup deletes the targets that are eligible for shutdown deletion.

Deletion eligibility is owned by [Ownership Model](ownership-model.markdown) and instantiated for WAF in [WAF Ownership Instantiation](managed-waf-item-ownership.markdown).

## Phase Boundaries

These boundaries should remain explicit:

- detection discovers raw data; it does not decide mutation authority
- derivation changes raw data into resource-specific targets; it does not mutate remote state
- reconciliation mutates toward the desired steady state for this round
- cleanup mutates under shutdown authority, not ordinary steady-state authority

## Extension Points

- If future work adds more resource kinds, they should plug into this lifecycle by defining their derivation needs, reconciliation instantiation, and cleanup eligibility instead of inventing a parallel lifecycle.
- If target derivation stops being identity, update this note and the resource notes so the derivation boundary stays explicit.
- If scheduling or startup ever become materially more complex, extend this note by refining phase boundaries instead of folding process-lifecycle rules into reconciliation notes.
