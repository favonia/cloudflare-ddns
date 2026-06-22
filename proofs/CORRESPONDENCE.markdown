# Correspondence Map: Documented Claims ↔ Lean Theorems

Confirm that each theorem in `proofs/Hostid6/Proofs.lean` states the same claim as the prose it cites.

| Documented claim | Prose location | Lean theorem |
| --- | --- | --- |
| A successful derivation preserves the top `PrefixLen` bits. | `derive.go`, `Derive` doc | `t1_prefix_preserved` |
| A successful literal derivation copies the literal's host bits. | `derive.go`, `Derive` doc | `t2_literal_host` |
| A successful MAC derivation requires `/64` and yields the EUI-64 interface identifier. | `derive.go`, `Derive` doc; `macHost` doc | `t2_mac_host` |
| `LiteralPrefixTooLong` fires exactly when the prefix exceeds the literal's allowance. | `LiteralPrefixTooLong` doc | `t3_literal` |
| `MACPrefixTooLong` fires exactly when the prefix is longer than `/64`. | `MACPrefixTooLong` doc | `t3_mac_long` |
| `MACPrefixTooShort` fires exactly when the prefix is shorter than `/64`. | `MACPrefixTooShort` doc | `t3_mac_short` |
