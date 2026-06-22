import Hostid6.Model
open Hostid6

/-- Parse a single hex digit (lower- or upper-case). -/
def hexDigit? (c : Char) : Option Nat :=
  if '0' ≤ c ∧ c ≤ '9' then some (c.toNat - '0'.toNat)
  else if 'a' ≤ c ∧ c ≤ 'f' then some (10 + c.toNat - 'a'.toNat)
  else if 'A' ≤ c ∧ c ≤ 'F' then some (10 + c.toNat - 'A'.toNat)
  else none

/-- Parse a hex string into a Nat. Empty string yields 0; any non-hex char fails. -/
def hexToNat? (s : String) : Option Nat :=
  s.foldl (fun acc c => match acc, hexDigit? c with
    | some n, some d => some (n * 16 + d)
    | _, _ => none) (some 0)

/-- Parse a hex string of address bytes (big-endian) into an `Addr`. -/
def hexToAddr (s : String) : Option Addr := (hexToNat? s).map (BitVec.ofNat 128)

/-- A single lowercase hex digit for `n % 16`. -/
def nib (n : Nat) : Char :=
  let d := n % 16
  if d < 10 then Char.ofNat ('0'.toNat + d)
  else Char.ofNat ('a'.toNat + (d - 10))

/-- Format an `Addr` as exactly 32 lowercase hex chars, big-endian. -/
def addrToHex (a : Addr) : String :=
  let n := a.toNat
  String.ofList ((List.range 32).reverse.map (fun i => nib (n / (16 ^ i))))

def parseDeriv (kind payload : String) : Option Deriv :=
  match kind with
  | "preserve" => some .preserve
  | "literal"  => if payload.isEmpty then none else (hexToAddr payload).map .literal
  | "mac"      => if payload.isEmpty then none else (hexToNat? payload).map (fun n => .mac (BitVec.ofNat 48 n))
  | _          => none

def handle (line : String) : String :=
  match line.splitOn " " with
  | kind :: pStr :: addrHex :: rest =>
    match pStr.toNat?, hexToAddr addrHex, parseDeriv kind (rest.headD "") with
    | some p, some a, some d =>
      match derive { addr := a, prefixLen := p } d with
      | .ok out => s!"ok {addrToHex out}"
      | .error (.literalPrefixTooLong b) => s!"err literalPrefixTooLong {b}"
      | .error .macPrefixTooLong => "err macPrefixTooLong"
      | .error .macPrefixTooShort => "err macPrefixTooShort"
    | _, _, _ => "err parse"
  | _ => "err parse"

partial def loop (stdin stdout : IO.FS.Stream) : IO Unit := do
  let line ← stdin.getLine
  if line.isEmpty then pure ()
  else
    stdout.putStrLn (handle (line.trimAscii).toString)
    stdout.flush
    loop stdin stdout

def main : IO Unit := do
  loop (← IO.getStdin) (← IO.getStdout)
