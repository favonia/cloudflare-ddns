# Design Note: Policy for Scripts

Read when: changing code, module boundaries, or `.golangci.yaml` in `scripts/github-actions/*`.

Defines: repository convention for standalone script modules under `scripts/github-actions/*`, including their module-local `golangci-lint` policy, derived from [Project Principles](../core/project-principles.markdown).

## Scope

Each standalone Go module under `scripts/github-actions/*` may define its own `.golangci.yaml`. These modules and their lint policies serve their own standalone script purposes, not a general support layer for the rest of the repository.

## Policy Boundary

- Repository-wide judgments belong in [`.golangci.yaml`](../../../.golangci.yaml).
- Stable exceptions arising from a standalone script module's context belong in that module's `.golangci.yaml` and apply only to that module.
- Statement-local or declaration-local exceptions belong inline as `//nolint:<linter> // reason`; follow [Go Lint Suppressions](go-lint-suppressions.markdown).

## Module-Wide Exceptions

Script modules still follow the repository's correctness, security, resilience, and operator-clarity priorities from [Project Principles](../core/project-principles.markdown).

Do not relax a linter at module scope when it materially protects required behavior, security properties, failure handling, or operator-facing clarity.

Module-local policy may relax rules whose cost is mainly ceremony or refactoring pressure tied to the small, standalone runner shape, including scale-sensitive maintainability or style rules, when enforcing them would not materially improve the priorities above.
