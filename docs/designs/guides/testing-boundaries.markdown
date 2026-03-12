# Design Note: Testing Boundaries

Read when: adding tests, moving tests, or deciding whether a test needs private access.

Defines: the repository's testing boundary convention for `package foo_test`, `*_internal_test.go`, and `export_test.go`.

Does not define: feature-specific testing strategy or assertion style.

The goal is to keep production package surfaces honest without making small white-box tests awkward.

## Default Test Shape

Prefer black-box tests in the external test package.

- use `package foo_test` for the normal package test suite
- test the exported contract the same way callers use it
- keep helpers and expectations aligned with the intended public package boundary

## White-Box Tests

Use same-package tests only when the test is directly about private implementation behavior.

- use `package foo`
- name the file `*_internal_test.go`
- call unexported helpers directly

Typical cases:

- small unit tests for private helper functions
- focused edge-case tests for local internal logic
- tests whose setup would become more awkward if routed through a larger exported API

## `export_test.go`

Use `export_test.go` only as a narrow escape hatch for black-box tests.

- a `package foo_test` test genuinely needs a small internal hook
- moving that test to `package foo` would lose the desired black-box perspective or create an import cycle
- the wrapper or alias can stay minimal and clearly test-only

- keep `export_test.go` in `package foo`
- expose the smallest possible alias or wrapper
- document why the hook is needed
- do not add production exports to satisfy tests

## What Not To Do

Do not use `export_test.go` for:

- small white-box tests of private helpers
- convenience access when a `*_internal_test.go` file would be clearer
- broad test-only mirrors of implementation details

## Practical Repository Rule

Prefer this order:

1. `package foo_test` for normal behavior tests
2. `package foo` in `*_internal_test.go` for small private-helper tests
3. `export_test.go` only when the first two options would make the test materially worse

This keeps test structure predictable and prevents accidental growth of test-only escape hatches across packages.
