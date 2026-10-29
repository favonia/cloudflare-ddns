# Design Note: Operator Messages

Read when: editing user-facing logs, notices, validation diagnostics, or advisory hints outside `README.markdown`.

Defines: repository-wide operator-message rules derived from [Project Principles](../core/project-principles.markdown).

Does not define: `README.markdown` writing rules, feature semantics, or exact package-local message text.

Notification triggers, first-known address coverage, observation inference, lifecycle composition, and state-evidence principles are owned by [Notification Model](../features/notification-model.markdown).

The [Notification Messages rationale](../../rationale/notification-messages.markdown) records the reader assumptions, alternatives, and evidence behind presentation choices.

## Verbosity Intent

- Quiet output is for status surfaces and quick scans. Preserve essential outcomes, material risks, and decisions the operator may need to make next; omit detail that is mainly explanatory, inspectable, or educational.
- Normal output is for an operator actively reading the run. It may include context, configuration summaries, and remediation guidance, but should still read as an outcome-oriented narrative rather than a debug dump.

## One-Time Detail When It Helps

When extra nuance is useful but not part of the primary outcome, emit it as follow-up detail with `NoticeOncef` or `InfoOncef` instead of bloating the main message.

## Channel-Specific Shape

- Heartbeat messages are terse status lines. Keep them compact and scan-friendly because heartbeat services mainly surface short status text.
- Notifier messages use scope blocks separated by blank lines, with lifecycle context and prerequisite problems kept visible. Multiple blocks remain one notification delivery. The [Notification Model](../features/notification-model.markdown) owns content-delta, operation-status, source, and scope rules.
- For multi-item lists inside heartbeat messages, prefer compact joins such as `a, b`. For notifier content, use English joins: `a and b` for two items and the Oxford comma in `a, b, and c` for three or more, without braces or square brackets.
- Headings identify resources; place managed scope in the observation that needs it. Keep current observations, historical input, and completeness conclusions distinct, including empty sets.
- Keep content changes distinct from individual operation outcomes. Source labels qualify admitted evidence; they do not weaken confirmed actions or certify live state at delivery.
- A combined notification has one primary kind but must retain all constituent failures in its content. Keep domain/family identity visible and distinguish historical addresses, desired targets, completed operations, and unconfirmed outcomes; grouping follows the Notification Model.
- Review composed runtime output in encounter order with adjacent CLI, heartbeat, and notifier messages. When exact human-facing wording is deferred, obtain maintainer approval of the recaptured production output before locking exact-text tests or goldens.

## Quoting And Value Shapes

- Use `%q` for parser and validation diagnostics on raw or untrusted inputs such as environment values, parser tokens, file paths copied from user input, or other text where escaping matters to remediation.
- Use `%s` for stable identifiers that are unlikely to be misunderstood without quotes, such as Cloudflare IDs, domain names, and full WAF list references in the form `account/name`.
- For advisory value display, shape the value for operator inspection instead of quoting mechanically. In current repository code this usually means helpers such as `pp.QuoteOrEmptyLabel`, `pp.QuoteIfNotHumanReadable`, `pp.QuoteIfUnsafeInSentence`, `pp.QuotePreviewOrEmptyLabel`, and `pp.QuotePreviewIfNotHumanReadable`.
- For known token-like values embedded inside English prose, prefer sentence-safe shaping such as `pp.QuoteIfUnsafeInSentence` over `%q` when preserving the raw shape improves readability. Use unconditional quoting instead when the value is opaque, malformed, or otherwise not trustworthy enough for heuristic shaping.
- When the empty string is the relevant state, prefer an explicit empty label such as `empty` or `(empty)` when that is clearer than showing `""`.
- Keep exact non-truncated values in mismatch or validation diagnostics when the user needs the full string to fix the problem. Use preview helpers only for advisory messages where exact full fidelity is not required.
- When reconciling or summarizing sets, make empty and partial results explicit. If the kept subset and dropped remainder both matter to operator action, show both, and use explicit empty labels such as `no tags`, `none`, or `(none)` instead of letting joins disappear silently.

## Runtime Message Style

Keep short operational `Noticef` and `Infof` messages compact; in current repository usage, that normally means no trailing period.

## Honest Failure Claims

Keep failure wording aligned with what the code can honestly claim about remote state.

- For state-changing remote operations, prefer wording like `could not confirm ...` when the request may already have taken effect remotely even though the local caller did not get a trustworthy success result.
- Make material uncertainty explicit through the affected operation and relevant context. Add a follow-up risk only when it conveys a consequence not already clear from those rows; do not automatically append an inconsistent-content warning to every unconfirmed operation.
- For read, probe, lookup, parse, or local setup failures that do not create remote-state ambiguity, plain `failed to ...` wording is acceptable.
- If the code has conclusive evidence for a stronger operator-facing classification, say that stronger fact directly instead of preserving generic failure wording.

## Supplementary Information

Evaluate each composed message in its own channel. After the operation or observation, add context only when it supplies a required fact the reader has not already received: scope, time, source, completeness, another outcome, or a specific evidence problem. Identical values can carry different facts; different words can repeat the same fact. In particular, operation operands do not automatically establish that the whole managed inventory has been listed.

Preserve known success when explaining a separate parsing or membership problem. Name the supported problem directly; do not use a generic uncertainty sentence to combine unrelated causes. Do not infer stronger identity, current-state, or completeness claims to shorten a message. Missing structured evidence is an implementation limitation to resolve or respect, not a reason to invent a reassuring explanation. A specific API cause and a generic setter uncertainty summary do not need separate CLI rows; retain independent cancellation, fallback, successful progress, and failure outcomes.

## Comparing Wording

Compare viable templates across the same relevant complete messages, including singular/plural, empty/nonempty, partial results, family scope, and cached provenance. Repair ordinary grammatical variants before rejecting a template. Choose consistency when it helps the reader compare facts; do not require identical sentence structure for observations, historical context, and operation conclusions.

Use separate source and reader passes. Establish which scenarios are producer-reachable, defensive, or still require evidence; then let readers assess the messages without teaching them the hidden convention being tested. Record assumptions and revisit reasons in the rationale owner rather than growing this guide into a catalog of every wording alternative.
