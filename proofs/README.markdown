# `proofs/` — Lean 4 Formalization

This directory contains a Lean 4 formalization of the documented claims of `internal/hostid6.Derive`, plus a standalone oracle binary used for differential testing. Nothing here is linked into the production binary; `CGO_ENABLED=0` is unaffected.

## Who needs Lean

Only contributors who modify `internal/hostid6.Derive` (or its documented claims) or the `proofs/` directory. Everyone else can ignore it — `go test ./...` runs the always-on regression tests with no Lean dependency.

## Toolchain

The pinned toolchain version is in `proofs/lean-toolchain`. You need `elan`, `lake`, and `lean` on your PATH; with `elan` present, the pinned toolchain auto-installs on the first `lake` invocation. There is no Mathlib dependency — only Lean core and `Std` (`bv_decide` via `import Std.Tactic.BVDecide`).

## Build and check the proofs

```sh
cd proofs && lake build Hostid6
```

This type-checks every theorem; a `sorry` shows up as a build warning. CI enforces a no-`sorry` gate independently with the nanoda kernel (`nanoda-allow-sorry: false`), which rejects any proof depending on `sorryAx`, and with `lake build --wfail`.

## Build and run the oracle

```sh
cd proofs && lake build oracle
```

The binary at `proofs/.lake/build/bin/oracle` is a persistent process: one request line in, one response line out. Request: `<kind> <prefixLen> <addr32> <payload>`, where `<kind>` is `preserve`/`literal`/`mac`, `<prefixLen>` is decimal, `<addr32>` is the 128-bit address as 32 lowercase hex chars, and `<payload>` is a 32-hex address for `literal`, a 12-hex MAC for `mac`, or absent for `preserve`. Response: `ok <addr32>`, `err literalPrefixTooLong <bound>`, `err macPrefixTooLong`, or `err macPrefixTooShort`.

## Run the differential test

```sh
cd proofs && lake build oracle && cd ..
HOSTID6_LEAN_ORACLE="$PWD/proofs/.lake/build/bin/oracle" go test -tags lean_oracle ./internal/hostid6/...
```

The differential test (`derive_diff_test.go`) compares `hostid6.Derive` against the oracle over a near-exhaustive enumeration plus random inputs. This is the code↔model link.

## Correspondence map

`proofs/CORRESPONDENCE.markdown` aligns each documented claim in `derive.go` and the `Incompatibility` constants to its Lean theorem.

## Maintenance rule

Changing `Derive` or any of its documented claims requires updating **all** of the following together:

1. The prose in `derive.go` (the `Derive` doc comment and `Incompatibility` constant comments).
2. `proofs/Hostid6/Model.lean` — the Lean model.
3. `proofs/Hostid6/Proofs.lean` — the theorems.
4. The Go tests in `internal/hostid6/`.

Then verify:

```sh
cd proofs && lake build Hostid6   # type-checks every theorem; warns on any sorry
HOSTID6_LEAN_ORACLE="$PWD/proofs/.lake/build/bin/oracle" go test -tags lean_oracle ./internal/hostid6/...
```
