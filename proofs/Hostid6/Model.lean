namespace Hostid6

abbrev Addr := BitVec 128

structure Raw where
  addr : Addr
  prefixLen : Nat      -- intended invariant: prefixLen ≤ 128

inductive Deriv where
  | preserve
  | literal (lit : Addr)
  | mac (m : BitVec 48)

inductive Incompat where
  | literalPrefixTooLong (bound : Nat)
  | macPrefixTooLong
  | macPrefixTooShort
  deriving DecidableEq, Repr

/-- Top `p` bits set, low `128-p` bits clear. p=0 → 0; p=128 → allOnes. -/
def prefixMask (p : Nat) : Addr := BitVec.allOnes 128 <<< (128 - p)

/-- Keep `a`'s top `p` bits; take `host`'s low `128-p` bits. -/
def combine (a host : Addr) (p : Nat) : Addr :=
  (a &&& prefixMask p) ||| (host &&& ~~~ (prefixMask p))

/-- Modified EUI-64 interface identifier in the low 64 bits; high 64 bits zero. -/
def eui64 (m : BitVec 48) : Addr :=
  let b : Fin 6 → BitVec 8 := fun i => (m >>> (8 * (5 - i.val))).setWidth 8
  let u := (b 0) ^^^ (0x02 : BitVec 8)         -- flip the U/L bit
  let bytes : Array (BitVec 8) :=
    #[u, b 1, b 2, 0xff, 0xfe, b 3, b 4, b 5]
  bytes.foldl (init := (0 : Addr)) (fun acc x => (acc <<< 8) ||| (x.setWidth 128))

/-- Max prefix length that leaves all set bits of `lit` in the host region.
    `lit = 0 → 128`; else 127 minus the position of the highest set bit. -/
def literalMaxPrefixLen (lit : Addr) : Nat :=
  match (lit.toNat) with
  | 0 => 128
  | n => 127 - Nat.log2 n

def derive (raw : Raw) : Deriv → Except Incompat Addr
  | .preserve    => .ok raw.addr
  | .literal lit =>
      let m := literalMaxPrefixLen lit
      if raw.prefixLen > m then .error (.literalPrefixTooLong m)
      else .ok (combine raw.addr lit raw.prefixLen)
  | .mac mm =>
      if raw.prefixLen > 64 then .error .macPrefixTooLong
      else if raw.prefixLen < 64 then .error .macPrefixTooShort
      else .ok (combine raw.addr (eui64 mm) 64)

end Hostid6
