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

Current startup includes:

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

Detection yields per-resource reconciliation intents for the current round. A reconciliation intent expresses what ownership and observation together lead to; the actual action is decided by the reconciliation algorithm.

If a resource is out of scope, there is only one intent:

- `preserve`: preserve existing content

If a resource is in scope, detection yields one of these three intents:

- `abort`: raw data is unavailable for this run; normal update is aborted and further handling is up to reconciliation
- `clear`: known empty raw data
- `update`: known non-empty raw data

The current concrete raw-data representation is a set of IP addresses with prefix lengths. The default prefix lengths are 32 for bare IPv4 observations and 64 for bare IPv6 observations. The default interpretation of bare IPv6 observations is owned by [IPv6 Default Prefix Length Policy](ipv6-default-prefix-length-policy.markdown).

Concrete detection and provider contracts are owned by [Provider Raw-Data Contract](provider-raw-data-contract.markdown). Detection security is owned by [Network Security Model](network-security-model.markdown).

## Derivation

Derivation transforms raw data into the resource-specific target state consumed by reconciliation.

- DNS derivation turns each raw IP address with prefix length into a DNS address target by forgetting the prefix length.
- WAF derivation turns each raw IP address with prefix length into a WAF prefix target by taking its subnet.

## Reconciliation

Reconciliation consumes reconciliation intent and derived resource-specific target state, and mutates managed remote state toward the desired result.

The shared reconciliation algorithm is owned by [Reconciliation Algorithm](reconciliation-algorithm.markdown). Resource-specific ownership filters and instantiations are owned by [Ownership Model](ownership-model.markdown), [DNS Ownership Instantiation](managed-record-ownership.markdown), and [WAF Ownership Instantiation](managed-waf-item-ownership.markdown).

## Cleanup

Cleanup is shutdown-time mutation after the process has decided to stop.

Cleanup deletes the resources that are eligible for shutdown deletion.

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
