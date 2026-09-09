# Design Note: Enforcement-Point Explanations

Read when: writing code comments, design-note pointers, or other non-README explanatory text at the point where a rule is enforced.

Defines: repository-wide rule for explaining rules at the enforcement point, derived from [Project Principles](../core/project-principles.markdown).

For feature semantics, see `docs/design/features/`; for README prose, see [README Writing](readme-writing.markdown); for design-document retrieval and placement, see [Design Documents](../README.markdown).

## Enforcement-Point Explanations

Document nontrivial function contracts, including those of internal helpers. At the enforcement point, prefer the shortest explanation that preserves the intended rule: a code comment for local context, or a design-note pointer when the full rule is shared across sites or needs durable cross-file context.

- During other code changes, preserve existing explanations and update them to match the changed code; remove them only when they no longer apply or their information has moved to a smaller correct durable home.
- Do not add explanatory padding that only restates obvious code, anticipates unlikely objections, or defends the decision against readers who are not the target audience.
- Mention internal mechanisms only when they change operator decisions, maintenance work, or local correctness constraints.
