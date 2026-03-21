# Design Note: Derived Rules

Read when: writing code comments, design-note cross-references, or other repository-wide explanatory text outside `README.markdown`.

Defines: repository-wide explanatory rules derived from [Project Principles](../core/project-principles.markdown).

Does not define: new decision-tree criteria, feature semantics, or README-specific writing rules.

This note records derived rules only. If a rule would change design tradeoffs, put it in [Project Principles](../core/project-principles.markdown) instead.

## Enforcement-Point Explanations

At the enforcement point, prefer the shortest comment or design-note pointer that preserves the intended rule.

- Prefer a short code comment when the rule is local and the required context fits there.
- Prefer a design-note pointer when the full rule is shared across sites or needs durable cross-file context.
- Do not add explanatory padding that only restates obvious code, anticipates unlikely objections, or defends the decision against readers who are not the target audience.
- Mention internal mechanisms only when they change operator decisions, maintenance work, or local correctness constraints.

## Scope Boundary

This note applies to repository-wide explanatory text outside `README.markdown`.

It does not define:

- feature-specific behavior, which belongs in `docs/designs/features/`
- beginner-facing README writing, which belongs in [README Writing](readme-writing.markdown)
- local one-off wording preferences that do not need a durable repository-wide rule
