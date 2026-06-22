# Design Note: Formal Verification Methodology

Read when: adding or maintaining Lean proofs or differential tests that verify a kernel against its documented contract.

Defines: the multi-leg methodology used to prove that a Go function's documented contract actually holds of its implementation.

Does not define: which kernels to verify, coverage strategy for ordinary unit tests, or CI infrastructure beyond the verification-specific jobs.

This note applies [Project Principles](../core/project-principles.markdown) §1 (Pragmatic Security — "detect bugs through analysis, tests, fuzzing, or formal verification") to the question of how this repository uses a proof assistant and differential testing together to verify a kernel's documented contract end-to-end.

The pilot is `internal/hostid6.Derive` with its Lean project in `proofs/`.

## What Verification Means Here

Verification proves that the *documented contract* is true — that the claims written in code comments and design notes actually hold of the code.

- a documented claim that is false makes a proof fail
- undocumented behavior is not claimed and therefore not verified
- the verified contract equals the documented contract: no more, no less

## The Three Legs

All three legs are required; each closes a gap the others cannot.

**Leg 1 — Lean 4 proof of the model.**
A Lean 4 project contains one theorem per documented claim. Each theorem formalizes exactly one specific documented sentence and proves it holds of a *model* of the function (a pure Lean function mirroring the kernel's logic).

**Leg 2 — Differential testing: the code↔model link.**
The Lean model is compiled to a persistent stdin/stdout *oracle* subprocess. A build-tagged Go test runs the real Go function and the oracle on the same inputs — near-exhaustive structural enumeration of interesting cases combined with randomized fill via `pgregory.net/rapid` — and asserts equal outputs. This checks *full function equality*, not just the named properties, and requires no characterization theorem. The oracle runs as a test-time subprocess; no cgo, no production dependency.

**Leg 3 — Correspondence audit.**
A human or AI check confirms that each theorem faithfully states the documented prose. A small audit table (e.g. `proofs/CORRESPONDENCE.markdown`) maps each theorem name to the exact documented sentence it encodes.

Together, the three legs establish: the Go code provably obeys its documented contract.

## Always-On Regression Tests

A small set of Lean-free property tests runs in normal `go test` as an everyday regression tripwire for contributors who do not have the prover installed.

In this pilot, these tests were kept to invariants checkable against an *input* or a *fixed constant* via derivation-agnostic helpers, and avoid re-implementing any private part of the function under test — that would be "code tested against a hand-copy of itself" (circular).

Computation-fidelity (exact byte layouts, boundary arithmetic derived from private helpers) is owned by the differential test against the independent model, not by these tests.

## External, Not Embedded

The prover output is never linked into the production binary.

- production stays `CGO_ENABLED=0`, pure Go
- the oracle is a test-time subprocess only
- kernels remain ordinary Go; no extraction or embedding required

## Property-vs-Fuzz Tiering

- **Near-exhaustive enumeration + random fill** (current approach): sufficient for shallow, structurally-enumerable kernels where the interesting input space can be covered without guidance.
- **Combined-coverage fuzzing** (deferred): prover compiled to C with sanitizer coverage, the Go function compiled as a c-archive under libFuzzer, linked into one harness. Reserved for deep or heavily-branched kernels where structural enumeration is infeasible. The toolchain cost is not justified on shallow kernels.

## Toolchain Pinning and CI

- Pin the exact prover version in `proofs/lean-toolchain`. Upgrades are deliberate.
- **Verification CI job** (path-filtered): installs the pinned prover, builds all proofs (checking every theorem), enforces a zero-`sorry` gate, builds the oracle, and runs the build-tagged differential test.
- **Normal `go test` job**: runs always-on tests with no prover dependency.

## Alternatives Considered

- **Property-contract only** (prove properties + a characterization lemma; no runtime oracle): rejected as the primary approach — depends on a characterization theorem that may not exist for every kernel. Differential testing requires none.
- **Embedded/extraction** (author the kernel in the prover, extract to Go, link via cgo): rejected — breaks the `CGO_ENABLED=0` static binary and the kernel would no longer be Go.
- **Committed differential vectors** (golden-file corpus): rejected — large committed diffs with high maintenance cost.
- **Combined-coverage differential fuzz as primary**: deferred, not rejected — correct approach for deep-branch kernels; low return on shallow ones.

## Future Work

- Apply the methodology to further kernels (e.g. IP filtering, detection-filter expression evaluation, syntax tokenizer/parser).
- A fail-safe flagship property ("never destroy DNS on failure"), of which the pilot's prefix-preservation and refuse-rather-than-write-garbage behavior is the local pure-function warm-up.
- Adopt combined-coverage fuzzing when a deep-branch kernel justifies the toolchain cost.

## Scope Boundary

This note defines the formal-verification methodology for pure-function kernels in this repository.

It does not define:

- which specific kernels are candidates for verification
- ordinary unit or integration test strategy beyond the always-on regression tests described above
- CI infrastructure unrelated to verification jobs
