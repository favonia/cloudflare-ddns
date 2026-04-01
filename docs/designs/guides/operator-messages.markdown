# Design Note: Operator Messages

Read when: editing user-facing logs, notices, validation diagnostics, or advisory hints outside `README.markdown`.

Defines: repository-wide operator-message rules derived from [Project Principles](../core/project-principles.markdown).

Does not define: `README.markdown` writing rules, feature semantics, or exact package-local message text.

This note records the durable operator-message consequences of the project principles. It is not a general prose style guide.

## Design From The Final Message

- Design operator-facing wording from the final operator-visible message, not from internal implementation structure.
- You do not need to literally draft the message first, but the final wording must be equivalent to having started there.
- Refactor the code as needed so it generates that message shape intentionally.
- If the wording sounds driven by parser tokens, placeholders, helper names, or other implementation details, rewrite it.
- Use the later sections of this note as constraints on that outcome.

## One-Time Detail When It Helps

- When extra nuance is useful but not part of the primary outcome, emit it as follow-up detail with `NoticeOncef` or `InfoOncef` instead of bloating the main message.
- Good follow-up detail includes migration hints, setup guidance, timeout advice, or clarification that affects operator choice but would make the primary message harder to scan.
- Keep summary and follow-up detail semantically aligned. The detail may refine the message, but it should not reverse or contradict it.

## Channel-Specific Shape

- Heartbeat messages are terse status lines. Keep them compact and scan-friendly because heartbeat services mainly surface short status text.
- Notifier messages are prose-like summaries. When combining multiple outcomes, join follow-up fragments into one sentence with semicolons and finish the whole notifier message with a trailing period.
- For multi-item lists inside heartbeat messages, prefer compact joins such as `a, b`. For notifier prose, prefer English joins such as `a and b` or `a, b, and c`.
- Keep the sentence case of notifier follow-up fragments explicit at the call site so the first fragment reads as a sentence start and later fragments read naturally after `;`.

## Quoting And Value Shapes

- Use `%q` for parser and validation diagnostics on raw or untrusted inputs such as environment values, parser tokens, file paths copied from user input, or other text where escaping matters to remediation.
- Use `%s` for stable identifiers that are unlikely to be misunderstood without quotes, such as Cloudflare IDs, domain names, and full WAF list references in the form `account/name`.
- For advisory value display, shape the value for operator inspection instead of quoting mechanically. In current repository code this usually means helpers such as `pp.QuoteOrEmptyLabel`, `pp.QuoteIfNotHumanReadable`, `pp.QuoteIfUnsafeInSentence`, `pp.QuotePreviewOrEmptyLabel`, and `pp.QuotePreviewIfNotHumanReadable`.
- For known token-like values embedded inside English prose, prefer sentence-safe shaping such as `pp.QuoteIfUnsafeInSentence` over `%q` when preserving the raw shape improves readability. Use unconditional quoting instead when the value is opaque, malformed, or otherwise not trustworthy enough for heuristic shaping.
- When the empty string is the relevant state, prefer an explicit empty label such as `empty` or `(empty)` when that is clearer than showing `""`.
- Keep exact non-truncated values in mismatch or validation diagnostics when the user needs the full string to fix the problem. Use preview helpers only for advisory messages where exact full fidelity is not required.
- When reconciling or summarizing sets, make empty and partial results explicit. If the kept subset and dropped remainder both matter to operator action, show both, and use explicit empty labels such as `no tags`, `none`, or `(none)` instead of letting joins disappear silently.

## Runtime Message Style

- Keep short operational `Noticef` and `Infof` messages compact; in current repository usage, that normally means no trailing period.
- Factor repeated guidance into helper functions when the repetition is semantic, such as permission or mismatch hints, instead of duplicating long message text.

## Honest Failure Claims

Keep failure wording aligned with what the code can honestly claim about remote state.

- For state-changing remote operations, prefer wording like `could not confirm ...` when the request may already have taken effect remotely even though the local caller did not get a trustworthy success result.
- When that ambiguous mutation outcome can leave managed state uncertain, keep the follow-up risk explicit, such as `records might be inconsistent` or `content may be inconsistent`.
- For read, probe, lookup, parse, or local setup failures that do not create remote-state ambiguity, plain `failed to ...` wording is acceptable.
- If the code has conclusive evidence for a stronger operator-facing classification, say that stronger fact directly instead of preserving generic failure wording.

## Scope Boundary

This note applies to operator-facing runtime messages outside `README.markdown`.

It does not define:

- `README.markdown` explanations, which belong in [README Writing](readme-writing.markdown)
- one-off local wording that does not need a durable repository-wide rule
- feature-specific warning triggers or contracts, which belong in `docs/designs/features/` when they are durable
