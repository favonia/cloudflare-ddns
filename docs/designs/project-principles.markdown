# Design Note: Project Principles

This document records the project-wide priorities that guide design tradeoffs.

## Priorities

1. Build the DDNS updater the maintainer wants to use.
2. Support the features the maintainer wants, including expressive output such as emojis.
3. Favor practical security:
   - prefer open design over obscurity
   - use code analysis, unit tests, fuzzing, and similar techniques to find bugs
   - detect common misconfigurations
4. Favor resilience against temporary failures such as network outages and Cloudflare downtime.
   - For mutation flows that can fail ambiguously (timeouts, transport errors), order operations so any executed prefix leaves the system in the best known state.
   - Define explicit risk tiers for each mutation flow, and prioritize operations that reduce higher tiers first.
   - In DNS reconciliation, this is modeled as `R0` (missing target coverage), `R1` (wrong-IP exposure), then `R2*` (metadata drift), then `R3` (duplicate hygiene).
5. Favor efficiency in network usage, CPU usage, memory usage, and operational churn.
6. Favor features that remain maintainable.
