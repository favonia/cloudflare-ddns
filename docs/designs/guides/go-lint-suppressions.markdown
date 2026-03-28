# Design Note: Go Lint Suppressions

Read when: adding or reviewing Go inline `//nolint`.

Defines: repository convention for Go inline `//nolint`, derived from [Project Principles](../core/project-principles.markdown).

Does not define: repository-wide Go lint configuration, which belongs in [`.golangci.yaml`](../../../.golangci.yaml).

This note records how the decision tree is applied to Go inline suppressions. It does not independently authorize broader lint policy.

## Local Exception First

Use inline `//nolint` only when all of the following are true:

- the exception is local to one declaration, statement, literal, or test
- naming the specific linter keeps the exception precise
- putting the rule in [`.golangci.yaml`](../../../.golangci.yaml) would be too broad
- a small refactor, helper, or clearer code shape would not remove the warning more cleanly

Write suppressions in the local form `//nolint:<linter> // reason`.

- Keep the scope on the smallest code site that needs the exception.
- Suppress one linter unless the same site needs inseparable suppressions.
- Give a concrete local reason when the exception is not already obvious from nearby code.
- Do not use bare `//nolint`, `//nolint:all`, or file-wide suppression as normal practice.

## Narrowest Durable Home

Choose the narrowest durable home for the rule:

- Put repository-wide lint decisions, stable false positives, and path-based exclusions in [`.golangci.yaml`](../../../.golangci.yaml).
- Put one-off exceptions inline at the enforcement point.
- If the same suppression repeats because of a shared code shape, move the durable rule to the smallest correct shared home, such as a helper, code comment, test helper, or existing design note.
- Prefer changing code over suppressing a warning when the warning points to a real local readability, correctness, or maintenance problem.

## Durable Recurring Judgments

### `exhaustruct`

Use `//nolint:exhaustruct` only for intentionally partial literals whose omitted fields are irrelevant at that site.

- Focused test fixtures and expectation values may initialize only the fields the test reads.
- Non-mutating selector, query, or protocol literals may set only the fields the call path uses.
- Keep mutating request literals exhaustive so new upstream fields stay visible during review.
- When the intentional omission is not obvious, say what local shape the literal is preserving.

### `paralleltest`

Use `//nolint:paralleltest` only when the test touches process-global state.

- Name the shared state in the reason, such as environment variables, timezone, signals, or a package global.
- Do not suppress `paralleltest` just because a test is inconvenient to parallelize.

### `lll`

Use `//nolint:lll` only for fixed operator-facing text that is clearer as one source string.

- Prefer ordinary wrapping or a helper first.
- Keep the exception tied to the specific message text, not to a file or function.

### `gochecknoglobals`

Use `//nolint:gochecknoglobals` only for intentional process-wide values.

- Shared immutable lookup tables, linker-injected version strings, and shared handles can justify it.
- Do not use it for convenience globals when a narrower dependency shape would do.

### `unparam`

Do not address `unparam` mechanically by deleting a parameter just because one current call path passes the same value every time.

- First check whether the parameter is part of the helper's honest contract.
- If removing it would hard-code a real dependency into a generic-looking helper, prefer deleting the thin wrapper and calling a more explicit helper directly, or keep the parameter with a local suppression and reason.
- Avoid "fixing" `unparam` by turning an explicit dependency into hidden coupling.

## Review Checks

When reviewing a new `//nolint`, ask:

- Is the exception truly local?
- Is the linter named explicitly?
- Is the reason concrete and local?
- Is there a narrower durable home for the rule?
- Would a small code change remove the suppression more cleanly?

If those answers are weak, rewrite or remove the suppression instead of normalizing it.

## Scope Boundary

This note defines how to apply the project principles to Go inline `//nolint`.

It does not define:

- which linters are enabled repository-wide
- feature-specific correctness rules
- one-off justifications that belong at the suppression site instead of in this note
- general refactoring policy beyond recurring lint-driven judgments such as the cases above
