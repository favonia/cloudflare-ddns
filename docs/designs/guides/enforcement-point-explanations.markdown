# Design Note: Enforcement-Point Explanations

Read when: writing code comments, design-note pointers, or other non-README explanatory text at the point where a rule is enforced.

Defines: repository-wide rule for explaining rules at the enforcement point, derived from [Project Principles](../core/project-principles.markdown).

Does not define: feature semantics, README-specific writing rules, or a general writing style guide.

This note records one repository-wide explanation rule. Use [Project Principles](../core/project-principles.markdown) for tradeoffs and this note for the local explanatory consequence of those tradeoffs.

## Enforcement-Point Explanations

At the enforcement point, prefer the shortest comment or design-note pointer that preserves the intended rule.

- Prefer a short code comment when the rule is local and the required context fits there.
- Prefer a design-note pointer when the full rule is shared across sites or needs durable cross-file context.
- Do not add explanatory padding that only restates obvious code, anticipates unlikely objections, or defends the decision against readers who are not the target audience.
- Mention internal mechanisms only when they change operator decisions, maintenance work, or local correctness constraints.

## Scope Boundary

This note applies across the repository to explanatory text at the site where code, tests, or developer docs enforce or point to a rule, outside `README.markdown`.

It does not define:

- feature-specific behavior, which belongs in `docs/designs/features/`
- `README.markdown` writing rules, which belong in [README Writing](readme-writing.markdown)
- retrieval or placement rules for `docs/designs/`, which belong in [docs/designs/README.markdown](../README.markdown)
- local one-off wording that does not need a durable repository-wide rule
