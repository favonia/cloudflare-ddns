# Design Note: IPv6 Default Prefix Policy

Read when: changing the default meaning of bare detected IPv6 addresses, including IPv6 lifting defaults, WAF IPv6 projection defaults, or exact-address versus network-presence semantics.

Defines: the product policy for choosing the default IPv6 prefix length when the updater has only a bare detected IPv6 address and no more specific operator instruction.

Does not define: provider-specific discovery rules, full host-ID grammar, exact Cloudflare API capability bounds, or ownership and reconciliation rules.

## Goal

Choose a default IPv6 interpretation rule that remains defensible even when the downstream platform supports both exact-address and broader-prefix targets.

## Core Policy

When the updater has only a bare detected IPv6 address and no more specific operator instruction, the default semantic boundary is `/64`.

- This is a product default, not a universal truth about IPv6 deployments.
- This is a default interpretation rule under incomplete information.
- Explicit operator configuration may choose narrower or broader prefixes.

## Non-Historical Rationale

### Product Semantics

A general-purpose dynamic-IP updater is usually trying to represent stable network or site presence, not one transient interface identifier.

Observed bare IPv6 addresses often mix two different kinds of information:

- a prefix that locates the current network attachment
- lower bits that identify one interface within that attachment

The default should therefore prefer the stable identity that the product most plausibly tracks when no stronger operator signal exists.

### Why `/64`

`/64` is the best default boundary for that meaning.

- It is broad enough to survive ordinary interface-identifier churn within one attachment.
- It is narrow enough to avoid defaulting all the way to delegated customer prefixes such as `/56` or `/48`.
- It keeps future host-ID derivation semantics coherent because the observed prefix and the preserved lower bits still describe the same attachment-level object.

### Why Not `/128` By Default

`/128` is too literal for the general default.

- It treats one exact observed interface identifier as the primary identity.
- It makes default behavior brittle when the host portion changes but the relevant site or network presence has not.
- It is a good explicit choice for least-privilege or per-device policies, but it is too specific to be the general fallback.

### What This Policy Does Not Claim

- It does not claim every real IPv6 deployment uses `/64`.
- It does not claim `/64` is always the safest authorization boundary.
- It does not claim downstream platform capability should dictate the product default.

## When `/64` Is The Wrong Default

`/64` is the wrong default when the operator's real intent is narrower than stable attachment identity.

Examples:

- exact per-device allow or block policies
- multi-tenant or mixed-trust environments that share one `/64`
- point-to-point or single-host infrastructure
- explicit least-privilege or exact-endpoint audit requirements

## Relationship To Lifecycle And Resources

This note refines lifecycle derivation under [Lifecycle Model](lifecycle-model.markdown).

- Provider contracts define how bare observations become raw IPv6 data.
- This note defines the default interpretation when that observation lacks a more specific prefix length.
- DNS and WAF may derive different final targets from the resulting observed-prefix model.
- Downstream support for narrower WAF items does not, by itself, change the default interpretation rule.

## Historical Context

The policy above does not rely on history.

Historical context is still useful because Cloudflare capability changed over time and made this design question visible again.

- On September 26, 2024, Cloudflare documentation corrected the documented IPv6 custom-list minimum from `/4` to `/12`.
- On July 8, 2025, Cloudflare documentation changed again to allow individual IPv6 addresses and IPv6 CIDRs up to `/128`.
- That removed the old platform constraint, but it did not settle the product-default question. The default remains a product-semantics choice.

## Extension Points

- If future evidence shows that most users mean exact endpoint identity rather than stable network presence, revisit this default.
- If future configuration splits the WAF default projection from the general IPv6 lifting default, update this note before changing user-facing semantics.
- If future design work adopts a different default attachment boundary, update this note and its linked lifecycle or provider notes together.
