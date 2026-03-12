# Design Note: Project Principles

Read when: making design tradeoffs or choosing between competing implementations.

Defines: the project-wide priorities that should influence many tasks.

Does not define: feature-specific semantics, exact mutation ordering, or local implementation details.

## Priorities

1. Build the DDNS updater the maintainer wants to use.
2. Support the features the maintainer wants, including expressive output such as emojis.
3. Favor practical security:
   - prefer open design over obscurity
   - use code analysis, unit tests, fuzzing, and similar techniques to find bugs
   - detect common misconfigurations
4. Favor resilience against temporary failures such as network outages and Cloudflare downtime.
   - For mutation flows that can fail ambiguously (timeouts, transport errors), order operations so any executed prefix leaves the system in the best known state.
   - Prefer designs whose useful prefix remains correct or recoverable after interruption.
5. Favor efficiency in network usage, CPU usage, memory usage, and operational churn.
6. Favor features that remain maintainable.
7. Favor user models that track observable outcomes rather than internal mechanisms.
   - User-facing guidance should explain what the updater will do, what it will not do, and what the operator can do next.
   - Mention internal mechanisms only when they materially change operator decisions.
