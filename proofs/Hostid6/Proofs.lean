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

end Hostid6
