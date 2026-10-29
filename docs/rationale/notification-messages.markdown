# Rationale: Notification Messages

Read when: evaluating the notification presentation model or revisiting a message whose clarity depends on surrounding output.

The governing contracts are [Notification Model](../design/features/notification-model.markdown), [Lifecycle Model](../design/features/lifecycle-model.markdown), and [Operator Messages](../design/guides/operator-messages.markdown). This document explains their motivations and evaluates possible refinements. It does not approve exact wording or claim that every candidate below is implemented.

The organizing question is what an operator can reasonably understand from one notification without knowing the renderer’s hidden rules. [Reader interpretation](#reader-interpretation) explains the product assumptions; [Evidence and outcomes](#evidence-and-outcomes) explains what can be claimed; [Presentation choices](#presentation-choices) compares formats; [Review and revision](#review-and-revision) describes how to challenge the arguments.

## Reader Interpretation

### A Notification Must Work Without Its Explanation

Human-reviewed: no.

A recipient commonly sees one message after an event, without the preceding logs, design notes, or an explanation of how omitted rows should be interpreted. That is the reader model behind compact scope blocks and locally meaningful labels. It is a product assumption about use, not a measured claim about every deployment.

A renderer can consistently use a convention that readers never discover. For example, a heading ending in `(filtered)` does not itself explain whether IPv4, IPv6, comments, detection results, or the rows below were filtered. Changing it to `(managed items)` names a category but still leaves the relationship between the parenthesis and the following statements implicit.

The candidate `Current managed IPv6 items: none` puts the subject and scope in the observation itself. Compared with `Current IPv6: no managed items`, it avoids first inviting the reader to expect a detected IPv6 address. This matters beside `IPv6 address detection failed.`: the failed target detection and the observed stored items answer different questions.

The longer subject costs space, especially for long domain names and narrow clients. The argument should be revisited if full-message review shows the shortened label is equally understandable, or if another layout makes the scope explicit without repeating it. A preference for a shorter string alone does not establish that equivalence.

### Completeness Is Information, Not The Absence Of More Rows

Human-reviewed: no.

`Added: A` identifies an addition. It does not tell an unbriefed reader whether B was already present. Similarly, `Removed: A` does not establish that nothing remains. Empty, complete, and unknown are distinct facts even when a renderer represents all three with no further value rows.

For a known complete pure addition, a sentence such as `This is the only managed address.` supplies the missing claim. Its plural counterpart, `These are all the managed addresses.`, is an ordinary grammatical variation rather than an implementation disadvantage. `No other managed addresses.` is a viable shorter alternative; positive wording ties the conclusion directly to what the reader just saw. For confirmed clearing, `No managed addresses remain.` answers the different question raised by removal.

Always repeating the full current inventory would also establish completeness. It makes large updates longer and repeats values that a compact delta already communicates. Always printing a count is shorter but introduces another number and still leaves the reader to locate the surviving values. The candidate compromise adds a conclusion where omission is especially ambiguous, while retaining the accepted compact mixed-delta and single-replacement forms.

That compromise has a real weakness: an arrow or an Added/Removed/Unchanged block is not a logical proof that every value was listed. It relies on a reasonable reading of the whole block. If fresh readers repeatedly miss surviving content in those forms, revisit the compromise rather than claiming that a documented convention makes the ambiguity disappear.

### Scope-Specific Knowledge Does Not Establish Another Scope’s Completeness

Human-reviewed: no.

A WAF message can add one IPv4 item while separately observing IPv6 contents after IPv6 detection failed. Reporting `Current managed IPv6 items: none` resolves the IPv6 question. It does not tell the reader whether the added IPv4 item is the only IPv4 item.

A candidate composition is:

```text
IPv6 address detection failed.

WAF list account/list
Added: 192.0.2.1/32
This is the only managed IPv4 item.
Current managed IPv6 items: none
```

The middle sentence closes the active family’s completeness question immediately. A global `These are all the managed items.` after both rows asks “These” to refer across different kinds of statement; an immediately preceding `none` makes that reference especially weak. Repeating the entire inventory is clearer than an ambiguous pronoun, but the family-specific sentence avoids that repetition here.

This candidate requires trustworthy complete contents for the stated scopes. Configured ownership and temporarily available targets are different inputs: a failed detection does not itself remove a family from management. Its implementation must preserve that distinction rather than infer scope from whichever operations happened to run.

### Repeated Values And Repeated Facts Are Different

Human-reviewed: no.

A useful supplement answers a question not already answered by the visible message. Two rows may use different words but repeat the same fact. Two rows may repeat an IP while adding a different fact.

After `Removal unconfirmed: A`, a generic `Managed contents could not be confirmed` usually restates the consequence already visible to the reader. It may even suggest a second verification failure that never occurred. Distinguishing “operation” from “aggregate contents” internally is not sufficient reason to print both statements.

Conversely, a removal operand identifies the item targeted for deletion; a complete Before inventory states what the managed set contained before the operation. They can have identical values without identical meaning. Omitting the inventory is justified only when its required information has already been conveyed or is not needed in this notification, not merely because every string appeared earlier.

For cleanup that targets every admitted managed identity, `Cleanup started for all managed items: A and B` can combine the action and coverage. With cached evidence, a candidate alternative is `Cleanup started: A and B` followed by `These were all the managed items (cached).` This avoids making the start event itself sound cached. Neither form is valid for a subset or an untrusted identity set.

The test applies within each channel. A specific console error may make a console wrapper redundant. It cannot make the same fact redundant in a separate notifier message whose recipient never saw that console output.

### Silence Should Not Acquire A New Meaning

Human-reviewed: no.

An ordinary no-op is useful work but usually not a new event. Sending the same “no update needed” inventory every round would make meaningful changes and failures harder to notice. It would also change the notification policy to solve a presentation problem.

Startup and first-known observations provide context without requiring perpetual success reminders. First-known means the first usable information included during this process, not a reconstruction of what existed at process startup. A later IPv6 failure should not cause unrelated already-covered IPv4 no-op content to be repeated merely to make a message look complete.

The design separates selection from wording: information useful in a selected notification need not create another notification trigger. Revisit that product choice if operators require an explicit periodic activity signal; such a requirement belongs in lifecycle or heartbeat policy, not an accidental extra notifier row. This rationale does not change repeated-failure policy.

## Evidence And Outcomes

### An Operation Result Is Not A Final Inventory

Human-reviewed: no.

A successful update establishes the result promised by its API contract. It does not establish that every other operation succeeded, that the whole managed set equals the new target, or that the item still belongs to the configured management scope. That distinction motivates successful content deltas for complete reconciliation and explicit operation outcomes for partial work.

Consider an addition that completes, followed by a deletion whose result is unconfirmed:

```text
WAF list account/list
Addition completed: 192.0.2.1/32
Removal unconfirmed: 198.51.100.0/24
Managed items before the update: 198.51.100.0/24
```

The first result stays completed. The later mutation makes the earlier returned inventory historical. The message therefore does not present the successfully added subset as the final list, and does not need another generic uncertainty sentence.

List deletion and item removal also need distinct names. A failed `DeleteList` request can be followed by a fresh observation of an empty list or successful fallback item removal. `Whole-list cleanup unconfirmed` can make that sequence look contradictory; `List deletion unconfirmed` identifies the uncertain operation while preserving the independently known contents.

All-unconfirmed wording would discard known progress; all-success wording would overstate it. Naming the exact operation preserves what the operator can rely on. The [DNS result](../../internal/api/dns_result.go) and [WAF result](../../internal/api/waf_result.go) contracts, rather than the English verb alone, establish the strength of those claims.

### Uncertainty Needs A Cause, Not A Universal Disclaimer

Human-reviewed: no.

The renderer can lack a complete delta for several different reasons. Treating them as one “contents unconfirmed” state hides useful distinctions:

| Evidence | What the message needs to convey |
| -- | -- |
| A dispatched operation is unconfirmed | The uncertain operation and its operands; relevant prior context |
| An operation completed but its returned contents could not be parsed | Completion plus the independent returned-data problem |
| A returned known item left management | The successful operation, supported exclusion, and any trustworthy current inventory |
| Returned identities are missing or conflicting | The evidence limitation without inventing counts or complete contents |
| No admitted baseline exists | No invented Before or delta; a separately trustworthy current observation may still be useful |
| A later mutation may have changed an earlier snapshot | Historical rather than current use of that snapshot |

For the parsing case, a candidate `Could not read the returned list contents.` explains something that `Addition completed` does not. It must refer to an actual unusable response, not imply a new read-back attempt. In the ordinary create-before-delete path, that failure stops later deletion, which is not attempted rather than unconfirmed. See [WAF API handling](../../internal/api/cloudflare_waf.go) and [setter sequencing](../../internal/setter/setter.go).

`could not confirm` and `unconfirmed` express the same uncertainty at different grammatical positions. The former fits a sentence; the latter fits an operation label. Replacing every failure with either phrase would obscure definite local failures and known successful mutations. Plain, specific wording is more accurate when the evidence is stronger.

### Membership And Content Knowledge Are Separate

Human-reviewed: no.

An item can remain physically present while its comment removes it from the managed selection. Calling that a removal would misdescribe the operation. Conversely, rejecting a successful managed-content delta does not mean every returned fact is unknown: a complete, trustworthy selected inventory can still establish what is currently managed.

The candidate design therefore preserves operation outcomes and an exclusion warning while allowing a supported terminal Current observation. It must check for later mutations that could invalidate the observation. An empty batch with no effect does not invalidate it merely because it appears later in the execution sequence.

This is a proposed refinement, not a claim about all current renderer paths. The [WAF content model](../../internal/updater/waf_content.go) and [renderer](../../internal/updater/render.go) must be checked before relying on terminal inventory after a rejected delta. A single delta-known Boolean does not explain why evidence was rejected or which other observations remain usable.

### Direct Comment Wording Requires Direct Identity Evidence

Human-reviewed: no.

In the normal DNS configuration and reconciliation pipeline, admitted comments match a stable selector, and requested comments come from those admitted values or a validated matching fallback. If the successfully returned comment fails that selector, it differs from the requested comment. For an existing admitted record it also differs from the earlier comment. Cache age affects when the difference was observed, not the implication that the values differ. See [configuration admission](../../internal/config/config_read.go), [metadata reconciliation](../../internal/setter/setter.go), and [DNS returned membership](../../internal/api/cloudflare_records.go).

This supports the candidate `Record for 192.0.2.2 excluded from management: its unexpected new comment does not match the filter.` “Unexpected” describes the returned discrepancy; it does not identify an actor. Compared with `its comment unexpectedly does not match the filter`, the candidate makes the changed value visible rather than placing surprise on a static filter result. For creation there is no earlier stored version, so `changed and no longer matches` is not a suitable universal replacement.

For WAF, a previously managed ID returned outside the selector supports `its comment unexpectedly changed and no longer matches the filter`. A full-list response containing an unselected item with a requested prefix does not, by that prefix alone, prove it is the newly created item. The current result does not retain an explicit created-ID association or a full unselected pre-operation inventory. A conservative guard may reject a delta without authorizing an exclusion accusation about that item.

The words should become more direct when the evidence supports them, rather than remain hedged as a stylistic habit. They should become more specific about the limitation when identity is missing, rather than manufacture a remote-modification story. Revisit the WAF restriction if the API boundary supplies a reliable created-item association. Another program is a plausible explanation for changed comments; the notification cannot establish who acted or with what intent.

### Current, Before, Cached, Empty, And Missing Are Different Axes

Human-reviewed: no.

Current and Before locate an observation relative to work; cached identifies its source. A valid cache entry is admitted evidence under the cache contract, not a fresh remote read or a weakened successful mutation. Consequently, a cached old arrow operand and a cached inventory label serve different grammatical roles but preserve the same provenance.

A group-level `(cached)` keeps repeated old values compact, but attachment to an action label can initially sound like a cached action result. `Removed (addresses from cache)` makes the source more explicit at the cost of longer recurring labels; a trailing marker after several values can appear to qualify only the last value. Keep the compact group marker as a deliberate tradeoff, alongside an explicit operation status; it must never weaken confirmed completion.

An empty managed selection is a known observation. Unknown input supplies no such observation. A missing WAF list is yet another fact: an existing list can contain zero managed items while retaining unowned contents. None of these states can be inferred merely because a value join produces an empty string.

The choice between Current and Before needs operation evidence. A completed or unconfirmed mutation can make prior contents historical. A failed prerequisite with no relevant mutation may leave admitted contents usable under the existing Current/cache contract. However, WAF `NoEffect` alone does not establish that no structural list-creation request was dispatched: a failed creation can have an uncertain structural outcome. A candidate Current path for a failed lookup therefore needs that distinction propagated, not inferred from the status name.

A more detailed timestamp or freshness report could make these distinctions explicit, but it would add recurring text without changing many operator decisions. Revisit source labels if cache provenance consistently fails to communicate their limits; do not quietly change the cache contract to repair a sentence.

### Later Observations Can Resolve Questions Without Rewriting History

Human-reviewed: no.

An unconfirmed request leaves multiple possible outcomes. A later fresh read can show contents compatible with the updater’s completed or possible effects, or a difference that those effects cannot explain. This makes the read useful without pretending that the earlier operation was known successful at the time it was reported. The [observation-inference contract](../design/features/notification-model.markdown#observation-differences) keeps that model separate from notification delivery and first-known coverage.

Compatibility does not identify which request succeeded; different histories may lead to the same observed contents. An unexplained difference is also information, but it does not identify an external actor. A message can report the observed difference directly without calling it a failed pending write or an intentional manual edit.

Present that unexplained observation before this round’s operation result. Otherwise, a subsequent successful update can make the reader attribute the surprising change to the updater’s own write. When no update is needed, the observation and the no-op assessment still answer different questions: what changed since earlier knowledge, and whether work is needed now. A cached read supplies no new remote observation for this inference.

### Stopping, Pending Cleanup, And Fallback Are Real Events

Human-reviewed: no.

`Cleanup started` describes accepted work without asserting removal. `No managed items remain` requires evidence of an empty managed result. Combining them would turn an accepted request into a completed outcome. Normal DNS management writes are synchronous under their contract; uncertainty is not a queue of future DNS writes. DNS propagation is not evidence that a successful management write remains pending.

A failed whole-list deletion followed by an empty item-cleanup fallback supplies two facts: the first attempt is unconfirmed, and a later observation found no managed items. Omitting the earlier attempt would hide a failure; labeling the later observation as the input to that earlier attempt would reverse time. [Cleanup evidence](../../internal/api/waf_result.go) preserves the ordering.

The notification should describe the existing waiting and cancellation policy, including accepted unwaited shutdown cleanup. Adding waits or reads solely to produce a stronger sentence changes runtime behavior and needs a separate design decision. The governing distinction belongs in the lifecycle model rather than a collection of wording exceptions.

## Presentation Choices

### Compare Templates Across The Same Situations

Human-reviewed: no.

A short label, a complete label, and a sentence are all viable ways to report an empty observation. Comparing one only in its easiest case gives an unreliable preference:

| Situation | Short label plus `no` | Complete label plus `none` | Direct statement |
| -- | -- | -- | -- |
| Current whole selection | `Current: no managed items` | `Current managed items: none` | `Currently no managed items.` |
| Current IPv6 selection | `Current IPv6: no managed items` | `Current managed IPv6 items: none` | `Currently no managed IPv6 items.` |
| Empty before update | `Before the update: no managed items` | `Managed items before the update: none` | `No managed items before the update.` |
| Empty before cleanup | `Before cleanup: no managed items` | `Managed items before cleanup: none` | `No managed items before cleanup.` |

The complete current label identifies the observed set before its value and allows an IP list in the same position when nonempty. The direct historical statement reads naturally and names its reference event. `before update` is understandable shorthand; `before the update` more naturally identifies the particular operation already described. `Previously` without an event can refer to an earlier run rather than this operation’s input.

The candidate preference is complete labels for current inventories, direct statements for empty history, and `No managed items remain.` for a proven empty result after removals. The difference is intentional: observation, prior context, and the conclusion of removal serve different reader questions. Full sentences such as `There are currently no managed items.` are also clear; their extra words need not be rejected as wrong just because a compact label suits this channel.

These are editorial judgments, not universal English laws. Reviewers can reasonably prefer the direct current statement. Revisit the choice by comparing complete messages with populated, empty, family-specific, and cached variants, rather than making visual uniformity the deciding criterion.

### Arrows Describe Replacements; Lists Describe Sets

Human-reviewed: no.

For one removed and one added value, `Updated: A → B` shows a replacement compactly. With multiple values, arrows and braces ask readers to parse set notation and can suggest pairings that the operation evidence does not establish. Comparing before and after sets alone cannot recover which record operation changed which address.

For a complete change from A to A, B, and C, the content description `Added: B and C` plus `Unchanged: A` makes the retained value explicit. `Updated: A → {A, B, C}` is mathematically readable but gives less direct help distinguishing new content from retained content. For an empty old or new set, Added or Removed is more natural than an arrow involving an empty-set symbol.

English joins work for single values, two-item lists, and larger lists; the Oxford comma preserves grouping without braces or brackets. Heartbeats have a different space budget and can use more compact joins. The notifier’s chosen syntax does not require applying its English list style to every status channel.

The arrow’s meaning also depends on its label. A complete content `Updated` summary and an incomplete-work `Update confirmed` operation pair make different claims. An independently evidenced old-to-new pair is required for the latter; a sorted pairing of sets is not a substitute.

### Object Nouns And Displayed Values Need Not Be Identical

Human-reviewed: no.

A WAF item has an identity, a prefix value, and a comment. Ownership and deletion apply to the item; coverage uses its prefix. This supports `item` for operator-facing list membership while retaining `prefix`, `IP range`, and prefix length where values and coverage are the topic. See [WAF ownership](../design/features/managed-waf-item-ownership.markdown) and [API types](../../internal/api/base.go).

`IP range` is a viable description of displayed contents, including a host represented by `/32` or `/128`. `IP prefix` is precise but more technical. `Subnet` or `network` can suggest deployment topology, while `address` alone can conceal that a value covers many addresses. `CIDR block` is precise but adds specialized vocabulary. The best noun follows the operator’s question, not the Go field name.

Using an IP value to identify an item is natural, but it does not turn distinct item identities into one object. The candidate “only managed item” conclusion therefore needs identity evidence, not a count of deduplicated prefix strings. `Unchanged` in a content summary preserves the displayed value; it does not guarantee unchanged identity or comment.

### Counts Explain Ambiguity Only When Their Evidence Exists

Human-reviewed: no.

A plain `(1 record)` often adds little when the admitted scope is known to contain exactly one record with that IP. An `other record` qualifier can add something essential after an earlier displayed update from the same IP: it distinguishes a separate deletion from the record already updated.

That explanation requires trustworthy, disjoint identities in the same scope. `Before`, a successful content-level arrow, and an earlier occurrence of the same address are not interchangeable with a known per-record operation. Plural counts likewise describe known records, not distinct address strings. Source provenance and multiplicity are separate: a cached group can still have a trustworthy count, and an unknown count cannot be repaired with `(cached)`.

Local duplicate-record fixtures test defensive behavior. They do not prove that Cloudflare accepts every same-domain/type/IP combination, and they should not force ordinary messages to carry counts solely to explain an unproven service state. Conversely, a reported rejection does not establish universal uniqueness for every API path and metadata combination. Revisit ordinary cardinality wording when service admissibility is demonstrated, while preserving conservative handling for malformed or ambiguous evidence.

### Ordering And Grouping Carry Meaning

Human-reviewed: no.

Lifecycle context explains why an otherwise quiet observation appears. Family and resource headings identify which results belong together. Keeping a confirmed operation next to its exclusion warning lets the reader distinguish write success from management membership without searching elsewhere in the notification.

Grouping only by desired target would combine scopes whose old state, provenance, outcome, or uncertainty differ. That reduces line count by discarding distinctions. Separate blocks can therefore be clearer even when they repeat the same target. Within one block, an operation-specific warning belongs near the operation, while a complete inventory belongs after the facts it helps interpret.

A resource-independent footer is a weak place to explain scoped ownership or partial failure: the reader must reconstruct which blocks it qualifies. The countervailing cost is repeated labels in a long message. Revisit grouping through complete multi-domain, IPv4/IPv6, and WAF examples; do not judge it from an isolated line.

## Review And Revision

### Reachable Cases And Defensive Cases Need Different Evidence

Human-reviewed: no.

A useful comparison covers empty and nonempty input, one and multiple values, overlap and disjoint sets, additions and removals, fresh and valid-cached input, expired-cache reload, configured and unavailable families, operation success and uncertainty, and shutdown waiting policy. These are dimensions to investigate, not a claim that their full Cartesian product is possible.

Control flow removes combinations: an ordinary WAF create parsing failure stops subsequent deletion; an empty Delete batch does not invalidate a preceding usable result; an initial ordinary read failure cannot produce later ordinary writes. Identity anomalies can still be valuable defensive tests without being normal service behavior. Structural prerequisite outcomes may need more evidence than an operation status exposes.

Source review should identify the producer path or the defensive premise for each example. Fixtures then check semantic distinctions, grouping, ordering, and provenance. Full rendered captures check whether the composed result communicates those distinctions. None of these checks alone proves remote API admissibility or reader comprehension.

### Independent Reading And Maintainer Review Have Different Jobs

Human-reviewed: no.

A source reviewer needs contracts and control flow. A fresh reader should first see the message without the hidden rule it is meant to demonstrate. Teaching that rule in the review prompt can produce apparent agreement without testing whether the message communicates it.

Independent AI reviews can reveal missing scope, awkward pronouns, contradictory implications, and unsupported claims. They are not a population study, and majority preference does not prove a design is correct. Preserve substantive disagreements: a source reviewer may see identical operand coverage where a reader still lacks a complete-set claim.

Human approval of one sentence does not approve a later composition or this newly written rationale. Examples here remain argumentative material; changes to exact runtime output follow the composed-output review process in Operator Messages. Revisit an argument when its evidence premise changes, a new producer path defeats its scope, or unbriefed readers make a consequential inference the message was intended to avoid. Update the governing contract when the decision changes, instead of leaving a better explanation attached to an obsolete rule.
