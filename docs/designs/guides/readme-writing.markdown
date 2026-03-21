# Design Note: README Writing

Read when: editing `README.markdown`.

Defines: README-specific writing rules derived from [Project Principles](../core/project-principles.markdown).

Does not define: feature semantics, a second decision tree, or local wording rules outside the README.

This note applies project-principle consequences to README readers as operators. Use [Project Principles](../core/project-principles.markdown) for tradeoffs. Use this note for the README-writing consequences of those tradeoffs.

## Operator-Facing Explanations

- Explain behavior through user-visible outcomes, required decisions, and actionable setup or upgrade steps.
- Prefer plain, concrete wording when it stays accurate enough for the reader to act correctly.
- Mention internal mechanisms only when they change what the reader must choose, configure, or verify.

## Point-Of-Use Clarity

- At the point where the reader acts, repeat short required facts such as permissions, prerequisites, or one-time manual steps when omitting them would risk a wrong setup.
- Prefer local repetition of required setup facts over forcing the reader to recover them from another section.
- When a setup example may interact with domains, records, or lists that already exist in Cloudflare, explain that starting state in the example or nearby prose if it changes the expected outcome or the reader's next step.
- Use concrete setting names, section names, or direct links when pointing elsewhere in the README.
- Avoid positional references such as "above" or "below" unless the referent is immediate and unlikely to drift.

## Layering And Placement

- Keep examples and nearby prose focused on the minimum information a reader needs to choose and apply the configuration correctly.
- Move exact reconciliation rules, edge cases, and implementation-level caveats to tables or later technical sections when they do not change the immediate setup or upgrade decision.
- Keep a short warning in the example or nearby prose when omitting that warning would likely cause a wrong setup or a misleading expectation.
- Keep advanced features and deep technical detail in advanced sections.
- Mention advanced features briefly in early setup sections only when needed for discoverability or because they are prerequisites for correct setup.

## Stable Terms And Reader Decisions

- Use terms consistently when they affect reader decisions.
- Words such as `optional` should keep one clear README meaning within the surrounding topic.
- Prefer direct descriptions of the user-visible state, such as "already exist", when that wording better explains the reader's decision than an internal-process term such as "already managed" or "already updated".

## Fixed README Markers

Use this fixed marker set only when it sharpens a reader's decision, expectation, or reading order. Keep each marker stable in meaning across the README.
This README uses the fixed marker set below.

| Marker                            | Stable meaning                                  | Use when                                                                                                                                                                                  | Do not use when                                                                                                                                        |
| --------------------------------- | ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `⚠️`                              | Scarce attention marker                         | The point must stand out to prevent a wrong setup, broken behavior, or a materially misleading expectation.                                                                               | The point is an ordinary caveat, background explanation, or something the reader could skip and still make the same correct setup or upgrade decision. |
| `🧪`                              | Contract-stability marker                       | Adopters should expect possible changes and review changelog entries when upgrading.                                                                                                      | The point is only release status, novelty, or recency.                                                                                                 |
| `🤖`                              | First-pass-skippable technical-detail marker    | The point is technical detail that most readers can skip on a first pass and return to only when they need deeper behavior, edge-case, implementation context, or advanced usage details. | The point is a required prerequisite, an immediate setup choice, or a warning that must stand out during setup or upgrade.                             |
| `(unreleased)`                    | Availability marker for not-yet-stable features | The feature is not in the latest stable release yet, and that stable-version gap changes the reader's decision or expectation.                                                            | The point is contract stability or general newness.                                                                                                    |
| `(available since version X.Y.Z)` | Availability marker for stable-version floor    | The feature is available in stable releases starting with version `X.Y.Z`, and that version boundary changes whether the reader can use it.                                               | The point is merely historical context or change log detail that does not affect the reader's current version decision.                                |

- Availability markers answer stable-version availability. Use `(unreleased)` or `(available since version X.Y.Z)` when that version boundary changes the reader's decision or expectation.
- If the exact stable-version boundary does not matter to the reader's current decision, omit the availability marker instead of adding it mechanically.
- If several nearby points compete for `⚠️`, keep it on the highest-risk point and rewrite the others in plain prose or section structure.
- Do not use `🧪` as a release-status marker.
- Do not invent additional fixed markers unless they solve a repeated README-level reader-decision problem. Decorative section emojis and one-off callout icons do not carry stable marker semantics.
- Apply markers only when they change the reader's decision or expectation; do not add them mechanically.

## Scope Boundary

This note applies only to `README.markdown`.

It does not define:

- durable feature semantics, which belong in `docs/designs/features/`
- local message wording outside the README
- changelog style or release-note policy beyond the README's need to signal operator-relevant availability or contract stability
