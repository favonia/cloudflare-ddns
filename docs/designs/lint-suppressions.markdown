# Design Note: Lint Suppressions

This document defines the repository's policy for inline `//nolint` suppressions.

It complements [`.golangci.yaml`](../../.golangci.yaml): repository-wide lint policy belongs in the linter configuration, while `//nolint` is reserved for code-local exceptions tied to a specific declaration, statement, literal, or test.

## Policy

Use inline `//nolint` only when all of the following are true:

- the exception is genuinely local to one code site
- a global exclusion in [`.golangci.yaml`](../../.golangci.yaml) would be too broad
- a small refactor or helper would not remove the suppression more cleanly
- the suppressed linter is named explicitly

Inline suppressions should follow these rules:

- Prefer the form `//nolint:<linter> // reason`.
- Suppress exactly one linter at a time unless multiple suppressions are inseparable at the same site.
- Keep the scope tight.
  - Attach the suppression to the smallest declaration, statement, or literal that needs it.
- Add a short local reason whenever the exception is not obvious from the surrounding code.
- Treat repeated copy-pasted suppressions as design feedback.
  - If the same pattern recurs, reconsider the helper API, the surrounding code shape, or the global lint configuration.

The repository does not use bare `//nolint`, `//nolint:all`, or file-wide suppression as normal practice.

## Where Policy Belongs

Choose the narrowest durable home for the rule:

- Put repository-wide decisions in [`.golangci.yaml`](../../.golangci.yaml).
  - This includes globally disabled linters, path-based exclusions, and stable tool false positives.
- Put package- or call-site-specific exceptions inline with `//nolint`.
- Prefer changing code over suppressing lint when the warning points to a real readability, correctness, or maintenance problem.

## Accepted Recurring Categories

The current tree shows a few recurring categories that are consistent with this policy.

### `exhaustruct`

`exhaustruct` is mainly suppressed for intentionally partial struct literals.

Accepted uses include:

- focused test fixtures that only set fields relevant to the scenario
- third-party or standard-library literals where omitted fields are intentionally left at zero values
- compact expectation structs in tests where exhaustiveness would add noise rather than clarity

Guidance:

- In tests, omitting irrelevant fields is usually acceptable.
- In non-test code, prefer a short reason when the intentional omission is not obvious.

### `paralleltest`

`paralleltest` is mainly suppressed for tests that cannot safely run in parallel because they touch process-global state.

Accepted uses include tests that read or mutate:

- environment variables
- the timezone
- package-level shared handles
- signal handlers or signal masks

Guidance:

- Always include the shared-state reason locally.
- Prefer naming the concrete shared state, such as “environment vars are global” or “changing global var file.FS”.

### `lll`

`lll` is acceptable for long fixed user-facing text when line-wrapping the source would make the final message harder to read or maintain.

Accepted uses include:

- fixed guidance shown directly to users
- mismatch explanations that are clearer as one sentence
- deprecation or migration hints with concrete setting names

Guidance:

- Prefer ordinary wrapping or helper functions first.
- Use `//nolint:lll` when preserving the message as one unit is genuinely clearer.

### `gochecknoglobals`

`gochecknoglobals` is acceptable for intentional process-wide values.

Accepted uses include:

- build-time version injection
- shared lookup tables
- shared filesystem handles
- shared signal lists
- shared network clients or conversion profiles

Guidance:

- The global must reflect intentional shared state, not convenience.
- Prefer a short reason when the shared purpose is not obvious from the declaration and nearby comments.

### Narrow Interoperability and Test Exceptions

Some linters appear only in narrow one-off cases. These are acceptable when the local reason is specific and externally constrained.

Examples include:

- `tagliatelle` for externally defined JSON field names such as `operation_id`
- `testifylint` when `require` would be unsafe inside an HTTP handler goroutine
- `embeddedstructfieldcheck` when embedding preserves the intended public model shape
- `unparam` when a helper signature stays intentionally symmetric with its callers
- `gosec` when a weak fallback is used only after a stronger mechanism fails and the risk is understood
- `forcetypeassert` after earlier checks establish the expected concrete type
- `noctx` when a test helper must call an API that has no context-aware alternative

These should remain rare and should usually carry an explicit local reason.

## Observed Repository Shape

The present tree is consistent with this policy in broad strokes:

- most inline suppressions are in tests
- most suppressions fall into a small set of recurring categories
- `exhaustruct` and `paralleltest` dominate the current footprint
- the remaining linters are sparse, local exceptions

That observed shape is useful as a sanity check. A new suppression style that does not resemble these patterns should be treated as unusual and justified explicitly.

## Review Heuristics

When reviewing a new `//nolint`, ask:

- Could this be solved better in [`.golangci.yaml`](../../.golangci.yaml)?
- Could a helper or small refactor eliminate the need for suppression?
- Is the scope as small as possible?
- Is the suppressed linter named precisely?
- Is the reason obvious from nearby code?
- If not, does the comment say why this exception is acceptable here?

If the answer to those questions is weak, the suppression should usually be rewritten or removed rather than normalized.
