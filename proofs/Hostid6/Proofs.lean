import Hostid6.Model
import Std.Tactic.BVDecide
namespace Hostid6

/-- Masking the result of `combine` with the same mask used inside `combine`
    recovers exactly the masked prefix of the first argument. This is a pure
    bitvector identity, true for any mask `M`; it is the core of T1. -/
private theorem combine_mask (X L M : BitVec 128) :
    ((X &&& M) ||| (L &&& ~~~M)) &&& M = X &&& M := by
  bv_decide

/-- T1: a successful derivation never changes the network prefix. -/
theorem t1_prefix_preserved (raw : Raw) (d : Deriv) (a : Addr)
    (h : derive raw d = .ok a) :
    a &&& prefixMask raw.prefixLen = raw.addr &&& prefixMask raw.prefixLen := by
  cases d with
  | preserve =>
      simp only [derive] at h
      cases h
      rfl
  | literal lit =>
      simp only [derive] at h
      split at h
      · exact absurd h (by simp)
      · simp only [Except.ok.injEq] at h
        subst h
        simp only [combine]
        exact combine_mask raw.addr lit (prefixMask raw.prefixLen)
  | mac mm =>
      simp only [derive] at h
      split at h
      · exact absurd h (by simp)
      · split at h
        · exact absurd h (by simp)
        · simp only [Except.ok.injEq] at h
          subst h
          have hp : raw.prefixLen = 64 := by omega
          rw [hp]
          simp only [combine]
          exact combine_mask raw.addr (eui64 mm) (prefixMask 64)

/-- Dual of `combine_mask`: masking the result of `combine` with the complement
    of the mask recovers exactly the masked host bits of the second argument.
    A pure bitvector identity for any mask `M`; it is the core of T2. -/
private theorem combine_host (X L M : BitVec 128) :
    ((X &&& M) ||| (L &&& ~~~M)) &&& ~~~M = L &&& ~~~M := by
  bv_decide

/-- T2-literal: a successful literal derivation copies the literal's host bits. -/
theorem t2_literal_host (raw : Raw) (lit a : Addr)
    (h : derive raw (.literal lit) = .ok a) :
    a &&& ~~~ prefixMask raw.prefixLen = lit &&& ~~~ prefixMask raw.prefixLen := by
  simp only [derive] at h
  split at h
  · exact absurd h (by simp)
  · simp only [Except.ok.injEq] at h
    subst h
    simp only [combine]
    exact combine_host raw.addr lit (prefixMask raw.prefixLen)

/-- T2-mac: a successful MAC derivation requires /64 and yields the EUI-64 IID. -/
theorem t2_mac_host (raw : Raw) (mm : BitVec 48) (a : Addr)
    (h : derive raw (.mac mm) = .ok a) :
    raw.prefixLen = 64 ∧
      a &&& ~~~ prefixMask 64 = eui64 mm &&& ~~~ prefixMask 64 := by
  simp only [derive] at h
  split at h
  · exact absurd h (by simp)
  · split at h
    · exact absurd h (by simp)
    · simp only [Except.ok.injEq] at h
      have hp : raw.prefixLen = 64 := by omega
      refine ⟨hp, ?_⟩
      subst h
      simp only [combine]
      exact combine_host raw.addr (eui64 mm) (prefixMask 64)

/-- T3-literal: literal incompatibility fires exactly when the prefix is too long. -/
theorem t3_literal (raw : Raw) (lit : Addr) :
    derive raw (.literal lit) = .error (.literalPrefixTooLong (literalMaxPrefixLen lit))
      ↔ raw.prefixLen > literalMaxPrefixLen lit := by
  simp only [derive]
  split <;> simp_all <;> omega

/-- T3-mac-long: MAC "too long" fires exactly when prefix > 64. -/
theorem t3_mac_long (raw : Raw) (mm : BitVec 48) :
    derive raw (.mac mm) = .error .macPrefixTooLong ↔ raw.prefixLen > 64 := by
  simp only [derive]
  split <;> first
    | (split <;> simp_all <;> omega)
    | (simp_all <;> omega)
    | simp_all

/-- T3-mac-short: MAC "too short" fires exactly when prefix < 64. -/
theorem t3_mac_short (raw : Raw) (mm : BitVec 48) :
    derive raw (.mac mm) = .error .macPrefixTooShort ↔ raw.prefixLen < 64 := by
  simp only [derive]
  split <;> first
    | (split <;> simp_all <;> omega)
    | (simp_all <;> omega)
    | simp_all

end Hostid6
