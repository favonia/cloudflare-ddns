# Design Note: Testing Boundaries

Read when: adding tests, moving tests, or deciding whether a test needs private access.

Defines: the repository convention for choosing between `package foo_test`, `package foo` in `*_internal_test.go`, and `export_test.go`.

This note applies [Project Principles](../core/project-principles.markdown) to Go test boundaries. Feature-specific coverage and package-specific helpers are outside its scope.

## Contract Boundary

Derive assertions and observation points from the tested unit's contract, including when testing a private unit. If a test relies on unpromised implementation details, document alongside it the verification value and why that dependency is necessary.

## Decision Order

When placing a test, choose the first shape that fits:

1. Use `package foo_test` for normal behavior tests. Exercise the exported contract as callers do, keeping helpers and expectations on the public side of the boundary.
2. Use `package foo` in `*_internal_test.go` for direct tests of private units' own contracts. Call unexported helpers directly and keep the file focused on those units.
3. Use `export_test.go` in `package foo` only when a test must remain in `package foo_test` and needs a minimal internal hook: moving it would blur the intended black-box boundary or create an import cycle. Keep aliases and wrappers minimal and test-only. Document the hook's behavior and caller obligations at the hook. Do not use this path for helper tests that fit the second choice or to mirror broad implementation details.

Do not add production exports to satisfy tests.

Reasons for test placement and package boundaries belong with the maintained design for those decisions. Reference this guide when its rules already explain the choice; record additional rationale only when future changes need it. Hook comments should describe how to use the hook and any reasons for its own constraints.
