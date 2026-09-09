# Design Note: README Writing

Read when: editing `README.markdown`.

Defines: README-specific writing rules derived from [Project Principles](../core/project-principles.markdown).

## Operator Decisions

Keep the README focused on what operators must decide, expect, configure, or verify, following [Project Principles](../core/project-principles.markdown).

- Keep required permissions, prerequisites, and one-time manual steps at the point of setup, repeating them when omission would risk a wrong setup.
- When a setup example may interact with domains, records, or lists that already exist in Cloudflare, explain that starting state if it changes the expected outcome or next step.
- Keep advanced features and exact reconciliation rules in advanced or technical sections, with early pointers when needed for discoverability or correct setup.

## Fixed README Markers

Use these fixed markers only when they sharpen a reader's decision, expectation, or reading order. Keep their meanings stable across the README.

| Marker                            | Stable meaning                                  | Use when                                                                                                                                                                                  | Do not use when                                                                                                                                        |
| --------------------------------- | ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `⚠️`                              | Scarce attention marker                         | The point must stand out to prevent a wrong setup, broken behavior, or a materially misleading expectation.                                                                               | The point is an ordinary caveat, background explanation, or something the reader could skip and still make the same correct setup or upgrade decision. |
| `🧪`                              | Contract-stability marker                       | Adopters should expect possible changes and review changelog entries when upgrading.                                                                                                      | The point is only release status, novelty, or recency.                                                                                                 |
| `🤖`                              | First-pass-skippable technical-detail marker    | The point is technical detail that most readers can skip on a first pass and return to only when they need deeper behavior, edge-case, implementation context, or advanced usage details. | The point is a required prerequisite, an immediate setup choice, or a warning that must stand out during setup or upgrade.                             |
| `(unreleased)`                    | Availability marker for not-yet-stable features | The feature is not in the latest stable release yet, and that stable-version gap changes the reader's decision or expectation.                                                            | The point is contract stability or general newness.                                                                                                    |
| `(available since version X.Y.Z)` | Availability marker for stable-version floor    | The feature is available in stable releases starting with version `X.Y.Z`, and that version boundary changes whether the reader can use it.                                               | The point is merely historical context or change log detail that does not affect the reader's current version decision.                                |

- If several nearby points compete for `⚠️`, keep it on the highest-risk point and rewrite the others in plain prose or section structure.
- Do not invent additional fixed markers unless they solve a repeated README-level reader-decision problem. Decorative section emojis and one-off callout icons do not carry stable marker semantics.
