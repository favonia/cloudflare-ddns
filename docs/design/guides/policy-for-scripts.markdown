# Design Note: Policy for Scripts

Read when: changing code, module boundaries, or `.golangci.yaml` in `scripts/github-actions/*`.

Defines: repository convention for standalone script modules under `scripts/github-actions/*`, including their module-local `golangci-lint` policy, derived from [Project Principles](../core/project-principles.markdown).

Does not define: repository-wide Go lint configuration, which belongs in [`.golangci.yaml`](../../../.golangci.yaml), or statement-local exceptions, which belong under [the inline suppression convention](go-lint-suppressions.markdown).

## Scope

Each standalone Go module under `scripts/github-actions/*` may define its own `.golangci.yaml`.

These modules exist for their own standalone script purpose, not as a general support layer for the rest of the repository.

Their `.golangci.yaml` files define module-wide lint policy for those script modules only.

## Policy Boundary

Module-local `.golangci.yaml` is the home for stable module-wide exceptions that follow from the script-module context rather than from one specific statement or declaration.

Repository-wide judgments belong in [`.golangci.yaml`](../../../.golangci.yaml).

Statement-local or declaration-local exceptions belong inline as `//nolint:<linter> // reason`.

## Required Standard

Script modules still follow the repository's correctness, security, resilience, and operator-clarity priorities from [Project Principles](../core/project-principles.markdown).

Do not relax a linter at module scope when it materially protects required behavior, security properties, failure handling, or operator-facing clarity.

Module-local lint policy must fit the script module's own standalone purpose. It must not treat `scripts/github-actions/*` as a general support layer for the rest of the repository.

## Accepted Module-Wide Exceptions

Script-module `.golangci.yaml` may relax linter rules whose cost is mainly ceremony or refactoring pressure tied to the small, standalone runner shape of these modules.

This includes scale-sensitive maintainability or style rules when enforcing them at module scope would not materially improve the priorities above.

Such relaxations are module-scoped exceptions, not a second repository-wide lint baseline.

## Scope Boundary

This note defines the repository boundary for standalone script modules and what module-local `golangci-lint` policy means within that boundary.

It does not define:

- how to review or edit `.golangci.yaml`
- one-off justifications that belong at a specific code site
- repository-wide linter enablement or disablement
- feature-specific correctness rules
