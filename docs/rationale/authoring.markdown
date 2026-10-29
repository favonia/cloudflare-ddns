# Rationale Authoring

Rationale explains why a design is defensible and what would justify changing it. Organize arguments around the decisions a future maintainer needs to evaluate. Keep routine rules in their design owner; link to that owner instead of creating a second contract here.

## Argument And Evidence

Begin with the reader problem or relevant fact, explain the consequence, then compare viable alternatives. Repair ordinary grammar and test an alternative in the same situations before rejecting it. Preserve rejected alternatives when their tradeoff prevents future repetition of an unproductive design choice.

Distinguish source-supported facts, inferences, product choices, reader assumptions, and missing evidence where the distinction affects the argument. Code establishes what a producer can do; a fixture establishes behavior for its inputs; neither alone establishes that a remote service admits those inputs. Reviewer agreement is useful judgment, not a user study. Record disagreement when it changes the tradeoff.

Examples illustrate an argument unless explicitly identified as an established contract. Label candidates and their prerequisites locally. Never use another layer’s current implementation as proof that the design of that layer is correct. Record a specific revisit trigger when a choice depends on a weak premise or an unsettled requirement.

## Human Review

Each argumentative subsection has one of these markers immediately after its heading:

- `Human-reviewed: no.`
- `Human-reviewed: yes.`

`yes` means a human maintainer has reviewed the argument as written and accepts both its meaning and its presence here. It does not imply independent verification of every external fact. New or substantively revised arguments start at `no`; AI review and approval of an earlier message example do not promote them to `yes`. Formatting-only edits may preserve the marker.

Keep each marked unit focused enough to review independently. These markers track review of the rationale, not runtime implementation, exact-output approval, or test status.
