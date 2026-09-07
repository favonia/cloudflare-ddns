# Design Note: Go Lint Suppressions

Read when: adding or reviewing Go inline `//nolint`.

Defines: repository convention for Go inline `//nolint`, derived from [Project Principles](../core/project-principles.markdown).

Repository-wide lint decisions, stable false positives, and path-based exclusions belong in [`.golangci.yaml`](../../../.golangci.yaml).

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
- Do not add semantically empty or misleading code solely to silence a linter; when the code is correct by an invariant the linter cannot see, use a precise suppression with a reason.
- Do not use bare `//nolint`, `//nolint:all`, or file-wide suppression as normal practice.

## Durable Recurring Judgments

### `exhaustruct_v5`

Use `//nolint:exhaustruct_v5` only for intentionally partial literals whose omitted fields are irrelevant at that site.

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
