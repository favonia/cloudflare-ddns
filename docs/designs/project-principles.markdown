# Design Note: Project Principles

This document records the project-wide priorities that guide design decisions.

## Priorities

1. Build the DDNS updater the maintainer wants to use.
2. Support the features the maintainer wants, including expressive output such as emojis.
3. Favor practical security:
   - prefer open security over security through obscurity
   - use code analysis, unit tests, fuzzing, and similar techniques to find bugs
   - detect common misconfigurations
4. Favor resilience by recovering from temporary failures such as network outages and Cloudflare downtime.
5. Favor efficiency in network usage, CPU usage, memory usage, and operational churn.
6. Favor features that remain maintainable.
