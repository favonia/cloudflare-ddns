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

The main startup boundary is owned by [Codebase Architecture](../core/codebase-architecture.markdown). Ownership and reconciliation notes assume startup has already produced a valid runtime configuration.

## Waiting and Triggering

Between update rounds, the process waits for the next trigger from startup policy or cron scheduling.

This phase covers immediate start, future scheduling, and shutdown requests.

## Detection

Detection yields per-resource reconciliation intents for the current round. A reconciliation intent expresses what ownership and admissible observation together lead to; the actual action is decided by the reconciliation algorithm.

If a resource is out of scope, there is only one intent:

- `preserve`: preserve existing content

If a resource is in scope, detection yields one of these three intents:

- `abort`: admissible raw data is unavailable for this run; normal update is aborted and further handling is up to reconciliation
- `clear`: known empty admissible raw data
- `update`: known non-empty admissible raw data

The current concrete raw-data representation is a set of IP addresses with prefix lengths. Known raw data must be admissible for all in-scope resources for that family and round. Bare observations are lifted using the effective default prefix lengths: 32 for IPv4 and 64 for IPv6 unless set otherwise. Problems provable from configuration-time known raw data, including static-provider incompatibilities, make startup invalid. A valid configuration may still encounter runtime-dependent observations that are inadmissible for a particular round; the affected family then yields `abort`. The default interpretation of bare IPv6 observations is owned by [IPv6 Default Prefix Length Policy](ipv6-default-prefix-length-policy.markdown).

Concrete detection and provider contracts are owned by [Provider Raw-Data Contract](provider-raw-data-contract.markdown). Detection security is owned by [Network Security Model](network-security-model.markdown).

## Derivation

Admissibility is semantically prior to target derivation: the raw data must be suitable for every in-scope resource before any derived target may authorize mutation. A pure operation may decide admissibility and produce candidate targets together, provided it discards those targets when the raw data is inadmissible.

- IPv4 DNS derivation turns each raw entry into a DNS address target by forgetting the prefix length.
- IPv6 DNS derivation uses each observed prefix and each domain's effective `hostid6` set under the compatibility and target-set rules in [DNS Ownership Instantiation](managed-record-ownership.markdown).
- WAF derivation turns each raw IP address with prefix length into a WAF prefix target by taking its subnet.

Admissibility preflight for one IP family must finish before any mutation for that family. If any raw entry is inadmissible for any in-scope resource, that family yields `abort`: existing DNS and WAF content for the family is preserved for the round. The other IP family remains independent. This preflight rule does not claim transactional mutation after preflight succeeds.

## Reconciliation

Reconciliation consumes reconciliation intent and derived resource-specific target state, and mutates managed remote state toward the desired result.

The shared reconciliation algorithm is owned by [Reconciliation Algorithm](reconciliation-algorithm.markdown). Resource-specific ownership filters and instantiations are owned by [Ownership Model](ownership-model.markdown), [DNS Ownership Instantiation](managed-record-ownership.markdown), and [WAF Ownership Instantiation](managed-waf-item-ownership.markdown).

## Cleanup

Cleanup is shutdown-time mutation after the process has decided to stop.

Cleanup deletes the resources that are eligible for shutdown deletion.

Deletion eligibility is owned by [Ownership Model](ownership-model.markdown) and instantiated for WAF in [WAF Ownership Instantiation](managed-waf-item-ownership.markdown).

## Phase Boundaries

These boundaries should remain explicit:

- detection and derivation together yield only raw data and targets admissible for all in-scope resources; they do not decide mutation authority
- admissibility preflight completes before mutation for the affected IP family
- derivation changes admissible raw data into resource-specific targets; it does not mutate remote state
- reconciliation mutates toward the desired steady state for this round
- cleanup mutates under shutdown authority, not ordinary steady-state authority

## Extension Points

- If future work adds more resource kinds, they should plug into this lifecycle by defining their admissibility requirements, derivation needs, reconciliation instantiation, and cleanup eligibility instead of inventing a parallel lifecycle.
- If target derivation stops being identity, update this note and the resource notes so the derivation boundary stays explicit.
- If scheduling or startup ever become materially more complex, extend this note by refining phase boundaries instead of folding process-lifecycle rules into reconciliation notes.
