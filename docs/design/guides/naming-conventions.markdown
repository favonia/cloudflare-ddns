# Design Note: Naming Conventions

Read when: adding or renaming code identifiers, config fields, or user-facing setting names.

Defines: a small set of repository-wide naming rules that are easy to lose during local cleanup.

## Semantic Names First

Prefer names that reflect the semantic role and per-lookup cardinality of a value, not just the container type that currently holds it.

- Do not pluralize a variable only because its current representation is `map[..]...`.
- If each lookup yields one detected IP for that family, prefer a name such as `detectedIP` over `detectedIPs`.
- If each lookup yields multiple targets for that family, prefer a plural semantic name such as `targetsByFamily` or `detectedTargets`.

## Write Values Versus Ownership Selectors

For user-facing setting names and config field names, keep write-side values singular and ownership selectors plural when that contrast describes the real scope difference.

- Use singular names for one value written to one managed object, such as `RECORD_COMMENT` or `WAF_LIST_ITEM_COMMENT`.
- Use plural selector names for settings that define the scope of a managed set, such as `MANAGED_RECORDS_COMMENT_REGEX` or `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.

## Canonical `String` Versus Human `Describe`

When a value has both a canonical, parseable form and an annotated human form, split them: `String()` for canonical, `Describe()` for human. `api.TTL` is the precedent (`String()` → `1`; `Describe()` → `1 (auto)`).

This keeps diagnostics quoting syntax operators can copy back while summaries stay readable.

- `String()` is the round-trippable syntax a user writes and diagnostics quote back (`hostid6.Derivation.String()` → `preserve`; `hostid6.Set.String()` → `[::1,::2]`).
- `Describe()` is human prose that may add annotations not meant to be parsed (`hostid6.Derivation.Describe()` → `preserve (using detected)`); keep such annotations out of any value an error quotes as the syntax to edit.
