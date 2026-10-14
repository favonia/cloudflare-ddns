# Design Note: Notification Model

Read when: deciding notification triggers, first-known address coverage, observation differences, lifecycle grouping, or evidence and uncertainty claims.

Defines: notification design principles and their lifecycle assumptions. These guide notification changes; they are not a claim that every derived behavior is already implemented.

## Events and Context

Notify about meaningful changes to managed content or service lifecycle, pending, failed or unconfirmed work, and newly observed address differences that the updater's own work cannot explain. Include enough known state to help the recipient understand the event.

Decide the trigger separately from its content. Previous addresses explain a transition; they need not create a new trigger. Determine operation outcomes from the operation contract, not address equality or the presence of formatted text. A successful no-op stays silent unless lifecycle context, missing first-known address context, or a newly unexplained observation justifies a notification.

Previous addresses are the managed DNS address set admitted as input to the operation, possibly from a valid cache. They are distinct from the previous detected public IP and the desired target. `static.empty` requests clearing: removal is a change, while already-empty successful assessment is a no-op. `none` leaves that family outside management and supplies no empty observation.

## Lifecycle and Composition

A scheduled process provides an ongoing service. Its startup and shutdown are meaningful lifecycle events. A one-shot invocation performs one operation; its ordinary process entry and exit are not separate service events. This distinction keeps externally scheduled one-shot runs from generating lifecycle noise.

One operation round contributes at most one notification envelope, combining relevant observations, first-known address context, and operation outcomes. A scheduled startup or shutdown flow incorporates its associated round or cleanup in its lifecycle envelope:

- With an immediate update, wait for the first round and combine startup with all assessed domain/family results and relevant WAF outcomes, including no-op and failures. Without an immediate update, report startup without inventing an address observation.
- Scheduled shutdown combines stopped context with cleanup outcomes, including pending or unconfirmed work. With cleanup disabled, report stopping without implying that managed content is empty.
- If no future execution can be scheduled, combine the scheduling failure with cleanup outcomes.
- `@once` reports its operation result without startup/shutdown context, deferred first-known coverage, or cross-round inference. An ordinary successful no-op stays silent. With `DELETE_ON_STOP=true`, it runs direct cleanup with the existing completion wait instead of an extra update.
- A handled signal during the immediate round preserves startup plus the actual interrupted or partial result, followed by a separate stopped/cleanup envelope. These are distinct lifecycle events; interruption must not discard successful partial work or imply a completed round.

Choose a primary notification kind without losing constituent failures. An associated update or cleanup failure takes its operation-failure kind rather than a successful lifecycle kind; an initial update failure is not a configuration/startup failure. Successful startup and shutdown envelopes use their lifecycle kinds. Scheduling failure takes priority on its exit path, with any cleanup failure retained explicitly. Ordinary operation kinds remain unchanged. An envelope containing only first-known context or unexplained observations uses a state-observation kind that does not imply a mutation.

Keep work, waiting, cancellation, heartbeat, and exit-status contracts in the [Lifecycle Model](lifecycle-model.markdown) independent of notification grouping. Notification delivery does not determine work success.

## First-Known Address Coverage

For a scheduled process, track first-known address context independently for each domain/IP family under a stable ownership scope. This is the first usable pre-operation address information included in a notification during this process; a later read does not reconstruct addresses at process startup.

A failed initial read leaves that scope uncovered while other scopes can be reported normally. When missing context becomes available later, include it once, even on no-op, without adding unrelated already-covered no-op scopes.

Coverage advances only when a nonempty envelope containing that scope's known pre-operation addresses is submitted to `Notifier.Send`, irrespective of its delivery Boolean. Known-empty and historical pre-operation addresses attached to a failure count; a desired target alone does not. Obtaining a snapshot or omitting it during rendering does not count. Coverage does not reset after cache expiry, later failure, or address changes. Keep it process-local, with no persistent history or delivery retries.

## Observation Differences

Keep observation inference separate from first-known coverage and delivery. For each domain/family, retain the latest admitted fresh remote observation and the completed effects or possible completed outcomes of subsequent own mutations. Compare a fresh pre-operation read with that model before applying this round's writes.

A compatible observation needs no extra observation-change notification and does not establish which request caused it. An observation outside the model supports reporting an unexplained address difference, even if reconciliation is a no-op. It does not identify an external actor. Record identities and duplicate multiplicity support inference; changes to them alone are not address-change triggers.

After either comparison result, replace the baseline and accumulated alternatives with the actual fresh observation. This resolves current-state uncertainty without proving whether an earlier unconfirmed request succeeded. Repeated unchanged reads therefore do not repeat the discrepancy, and old addresses gain no permanent exemption from comparison. Failed or invalid reads supply no new baseline; valid cached input is historical operation evidence, not an independent fresh observation. Cache expiration alone creates no notification trigger.

Inference must admit every possible completed own outcome so approximation cannot manufacture a discrepancy. Keep uncertainty local and storage bounded, with conservative widening when precision cannot be retained. Keep the representation, analysis budget, and aliasing rationale with their owning code.

## Evidence and Uncertainty

State reports describe evidence obtained for the operation. They do not certify live remote state at delivery time. Do not add provider or API reads solely to enrich a notification.

Keep known-empty distinct from unknown, observed addresses distinct from desired targets, and unattempted work distinct from no-op. An acknowledged DNS mutation supports reporting its completed effect; it does not imply a fresh read-back or prove exact final managed membership when returned metadata changes ownership. A pre-operation snapshot remains historical after an uncertain mutation and may accompany an unconfirmed result without being presented as final state.

DNS management operations are synchronous: an unconfirmed attempt may or may not have completed, rather than representing queued future work. Public DNS propagation and TTL do not turn completed management operations into pending writes. WAF item operations can remain asynchronous; pending exit cleanup is not confirmed completion. Historical WAF item reporting and cross-round WAF difference inference are outside this model.

Preserve useful results for successful domains alongside failures for other domains. Empty-state claims refer to the managed scope, not all DNS records or list contents. Group domains only when their rendered outcome, relevant previous and target sets, uncertainty, and observation context agree. Canonicalize address sets; a shared target alone is insufficient for grouping.

## Contract Placement

API boundaries own admitted reads, cache-versus-remote evidence, returned-data ownership, and supported mutation effects. Setters own immutable pre-operation snapshots and outcomes for operations actually reached. Updaters preserve intent, targets, partial DNS results, and WAF outcomes. Process-owned reporting below `cmd/ddns` owns inference, coverage, and semantic content selection; the composition root selects lifecycle context and submits envelopes without inspecting record internals or cache expiry. Notifiers format and deliver composed notifications through existing transports; domain-state rendering and grouping belong to the reporting layer.

Document snapshot ownership, copying, cache validity, result invariants, and caller obligations with the responsible types and functions. Exact wording and channel formatting follow [Operator Messages](../guides/operator-messages.markdown).
