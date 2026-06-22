# `proofs/` — Lean 4 Formalization

This directory contains a Lean 4 formalization of the documented claims of
`internal/hostid6.Derive`, plus a standalone oracle binary used for differential
testing. Nothing here is linked into the production binary; `CGO_ENABLED=0` is
unaffected.

## Who needs Lean

Only contributors who modify `internal/hostid6.Derive` (or its documented
claims) or the `proofs/` directory. Everyone else can ignore this directory
entirely — `go test ./...` runs the always-on regression tests with no Lean
dependency.

## Structure

- `Hostid6/Model.lean` — the Lean model: `derive` over `BitVec 128`, with
  `combine`, `eui64`, `literalMaxPrefixLen`, `prefixMask`.
- `Hostid6/Proofs.lean` — six theorems (zero `sorry`): `t1_prefix_preserved`,
  `t2_literal_host`, `t2_mac_host`, `t3_literal`, `t3_mac_long`, `t3_mac_short`.
- `Oracle.lean` — a persistent stdin→stdout oracle process used by the
  differential test.
- `CORRESPONDENCE.markdown` — the manual/AI audit surface that aligns each
  documented claim to its Lean theorem.

## Toolchain

The pinned toolchain version is in `proofs/lean-toolchain`
(`leanprover/lean4:v4.31.0`). You need `elan`, `lake`, and `lean` on your PATH.
If `elan` is present, it auto-installs the pinned toolchain on the first `lake`
invocation — no manual `elan toolchain install` step is needed. There is no
Mathlib dependency; only Lean core and `Std` are used (`bv_decide` is imported
via `import Std.Tactic.BVDecide`).

## Build and check the proofs

```sh
cd proofs && lake build Hostid6
```

This type-checks every theorem. A passing build with no `sorry` is the
proof-validity gate. To confirm no `sorry` was introduced:

```sh
! grep -rn 'sorry' proofs/Hostid6/
```

## Build the oracle

```sh
cd proofs && lake build oracle
```

Binary is at `proofs/.lake/build/bin/oracle`.

**Wire codec:** The oracle is a persistent process. It reads one request per
line on stdin and writes one response per line on stdout.

Request format:

```
<kind> <prefixLen> <addr32> <payload>
```

where `<kind>` is `preserve`, `literal`, or `mac`; `<prefixLen>` is a decimal
integer; `<addr32>` is the 128-bit address as 32 lowercase hex chars; and
`<payload>` is the literal address (32 hex chars) for `literal`, the MAC (12 hex
chars) for `mac`, or absent for `preserve`.

Response format:

```
ok <addr32>
err literalPrefixTooLong <bound>
err macPrefixTooLong
err macPrefixTooShort
```

## Run the differential test

```sh
cd proofs && lake build oracle && cd ..
HOSTID6_LEAN_ORACLE="$PWD/proofs/.lake/build/bin/oracle" go test -tags lean_oracle ./internal/hostid6/...
```

The differential test (`derive_diff_test.go`) compares `hostid6.Derive` against
the oracle over a near-exhaustive enumeration plus random inputs. This is the
code↔model link.

## Run the always-on tests (no Lean required)

```sh
go test ./internal/hostid6/
```

These run as part of the normal `go test ./...` suite and require no Lean
installation.

## Correspondence map

`proofs/CORRESPONDENCE.markdown` aligns each documented claim in `derive.go` and
the `Incompatibility` constants to its Lean theorem, and notes which always-on Go
test (if any) provides a cheap everyday regression for that claim.

## Maintenance rule

Changing `Derive` or any of its documented claims requires updating **all** of
the following together:

1. The prose in `derive.go` (the `Derive` doc comment and `Incompatibility`
   constant comments).
2. `proofs/Hostid6/Model.lean` — the Lean model.
3. `proofs/Hostid6/Proofs.lean` — the theorems.
4. The Go tests in `internal/hostid6/`.

Then verify:

```sh
cd proofs && lake build Hostid6   # must pass, zero sorry
! grep -rn 'sorry' proofs/Hostid6/
HOSTID6_LEAN_ORACLE="$PWD/proofs/.lake/build/bin/oracle" go test -tags lean_oracle ./internal/hostid6/...
```
