# Design Note: Naming Conventions

Read when: adding or renaming code identifiers, config fields, or user-facing setting names.

Defines: a small set of repository-wide naming rules that are easy to lose during local cleanup.

Does not define: package boundaries, feature semantics, or a general style guide.

This lower-level guide applies [Project Principles](../core/project-principles.markdown) to recurring naming choices across unrelated parts of the repository. If this note conflicts with the principles, the principles decide.

This note records only naming rules whose justifications come from operator clarity or maintainability concerns that recur across unrelated code and docs.

## Semantic Names First

Prefer names that reflect the semantic role and per-lookup cardinality of a value, not just the container type that currently holds it.

- Do not pluralize a variable only because its current representation is `map[..]...`.
- This improves maintainability by keeping names stable when storage choices change and by making review discussions about meaning instead of representation.
- If each lookup yields one detected IP for that family, prefer a name such as `detectedIP` over `detectedIPs`.
- If each lookup yields multiple targets for that family, prefer a plural semantic name such as `targetsByFamily` or `detectedTargets`.

## Write Values Versus Ownership Selectors

For user-facing setting names and config field names, keep write-side values singular and ownership selectors plural when that contrast describes the real scope difference.

This improves operator clarity by distinguishing "the value this updater writes onto one managed object" from "the selector that defines which managed objects are in scope."

- Use singular names for one value written to one managed object, such as `RECORD_COMMENT` or `WAF_LIST_ITEM_COMMENT`.
- Use plural selector names for settings that define the scope of a managed set, such as `MANAGED_RECORDS_COMMENT_REGEX` or `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.
- Keep selector names distinct from write-side value names so operators can scan environment-heavy setups without confusing "what this updater writes" with "what this updater may mutate".

## Canonical `String` Versus Human `Describe`

When a value has both a canonical, parseable form and an annotated human form, split them: `String()` for canonical, `Describe()` for human. `api.TTL` is the precedent (`String()` → `1`; `Describe()` → `1 (auto)`).

This keeps diagnostics quoting syntax operators can copy back while summaries stay readable.

- `String()` is the round-trippable syntax a user writes and diagnostics quote back (`hostid6.Derivation.String()` → `preserve`; `hostid6.Set.String()` → `[::1,::2]`).
- `Describe()` is human prose that may add annotations not meant to be parsed (`hostid6.Derivation.Describe()` → `preserve (using detected)`); keep such annotations out of any value an error quotes as the syntax to edit.
- With a single rendering, name by intent; a `Describe()` that is also valid syntax (`api.WAFList.Describe()` → `account/name`) is fine when no separate human form exists.

## Scope Boundary

This note applies only to naming choices that recur across unrelated code and docs.

It does not define:

- local helper names whose meaning is obvious within one small file
- public wording rules for logs or notices, which belong in [Operator Messages](operator-messages.markdown)
- feature-specific ownership semantics, which belong in `docs/designs/features/`
