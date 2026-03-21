# Design Note: Testing Boundaries

Read when: adding tests, moving tests, or deciding whether a test needs private access.

Defines: the repository convention for choosing between `package foo_test`, `package foo` in `*_internal_test.go`, and `export_test.go`.

Does not define: feature-specific coverage strategy or any new project-wide testing priorities.

This note applies [Project Principles](../core/project-principles.markdown) to one local question: how test code crosses Go package boundaries in this repository.

## Default Boundary

Use `package foo_test` for the normal test suite.

- keep behavior tests outside the package
- exercise the exported contract the same way callers use it
- keep helpers and expectations on the public side of the boundary

## Same-Package Tests

Use same-package tests only when the test is directly about private implementation behavior.

- use `package foo`
- put these tests in `*_internal_test.go`
- call unexported helpers directly
- keep the file focused on local helper behavior or implementation logic that would become less clear if driven only through exported APIs

Typical cases:

- small unit tests for private helper functions
- focused edge-case tests for local internal logic
- tests whose setup only makes sense inside the package

## `export_test.go`

Use `export_test.go` only when a `package foo_test` test still needs a small internal hook after the first two choices have been ruled out.

- keep `export_test.go` in `package foo`
- keep the alias or wrapper minimal and test-only
- use it when moving the test into `package foo` would blur the intended black-box boundary or create an import cycle
- document why the hook is needed
- do not add production exports to satisfy tests
- do not use it for small helper tests that fit cleanly in `*_internal_test.go`
- do not mirror broad implementation details through test-only exports

## Decision Order

When placing a test, choose the first shape that fits:

1. `package foo_test` for normal behavior tests
2. `package foo` in `*_internal_test.go` for direct tests of private helpers or local implementation behavior
3. `export_test.go` only when the test should stay in `package foo_test` and still needs a minimal internal hook

## Scope Boundary

This note defines repository-wide test boundary conventions.

It does not define:

- feature-specific scenario coverage
- package-specific test helpers beyond the boundary rules above
- general testing philosophy beyond the local conventions above
