# Correspondence Map: Documented Claims ↔ Lean Theorems

This table is the **manual/AI audit surface for leg (3)** of the verification:
confirm that each Lean theorem in `proofs/Hostid6/Proofs.lean` faithfully states
the same claim as the prose in `derive.go` or the `Incompatibility` constant
comments.

The **code↔model link is the differential test** (`derive_diff_test.go`, build
tag `lean_oracle`), not this table. This table only tracks whether the theorem
statements match the English prose; it cannot substitute for the differential test.

## Claim-to-theorem map

| Documented claim (prose location) | Lean theorem (`proofs/Hostid6/Proofs.lean`) | Always-on Go regression (`derive_props_test.go`), if any |
|---|---|---|
| "Derive only ever changes host bits: on success the result equals the observed address on the top PrefixLen bits" (`derive.go` `Derive` comment, T1) | `t1_prefix_preserved` | `TestProp_T1_PrefixPreserved` |
| "a literal supplies the host bits" (`derive.go` `Derive` comment, T2) | `t2_literal_host` | `TestProp_T2_LiteralHost` |
| "a MAC supplies the Modified EUI-64 interface identifier, which requires exactly a /64" (`derive.go` `Derive` comment, T2; `macHost` comment) | `t2_mac_host` | Partially `TestProp_T3_MACBoundary` (the /64 requirement only; EUI-64 content is covered by the differential test — see rationale below) |
| "LiteralPrefixTooLong: the observed prefix is longer than the literal allows" (`LiteralPrefixTooLong` const comment, T3) | `t3_literal` | (covered by the differential test; not re-asserted on the Go side — see rationale below) |
| "MACPrefixTooLong: the observed prefix is longer than /64" (`MACPrefixTooLong` const comment, T3) | `t3_mac_long` | `TestProp_T3_MACBoundary` |
| "MACPrefixTooShort: the observed prefix is shorter than /64" (`MACPrefixTooShort` const comment, T3) | `t3_mac_short` | `TestProp_T3_MACBoundary` |

## Rationale: why the Go regression column is intentionally sparse

The always-on Go tests (`TestProp_*`) are a cheap, Lean-free everyday regression
tripwire. They are deliberately limited to invariants that can be checked against
an input or a fixed constant: T1 prefix preservation, T2 literal host bits
(checked against the input literal), and the T3 MAC /64 boundary (checked
against the constant 64).

Two invariants are intentionally omitted:

- **EUI-64 byte content** (`t2_mac_host`): asserting the exact layout on the Go
  side would re-implement `macHost` in test code — Go tested against a hand-copy
  of Go is circular. The differential test covers this against the independent
  Lean model.
- **`LiteralPrefixTooLong` boundary** (`t3_literal`): asserting this would
  re-implement `literalMaxPrefixLen` in test code — same circularity concern. The
  differential test covers it against the Lean model.

## Go API surface (grounding)

Confirmed signatures at the time this correspondence map was established.

**`internal/hostid6` package:**

- `hostid6.Derive(raw ipnet.RawEntry, derivation Derivation) (netip.Addr, *Incompatibility)`
- `hostid6.Preserve() Derivation`
- `hostid6.Literal(addr netip.Addr) (Derivation, error)`
- `hostid6.MAC(addr [6]byte) Derivation`
- `hostid6.Incompatibility{ Kind IncompatibilityKind; Derivation Derivation; ObservedPrefix ipnet.RawEntry; PrefixLenBound int }`
- Constants: `LiteralPrefixTooLong`, `MACPrefixTooLong`, `MACPrefixTooShort` (type `IncompatibilityKind`)

**`internal/ipnet` package (used as oracle input/output):**

- `ipnet.RawEntryFrom(addr netip.Addr, prefixLen int) RawEntry`
- `(RawEntry).Addr() netip.Addr`
- `(RawEntry).PrefixLen() int`
