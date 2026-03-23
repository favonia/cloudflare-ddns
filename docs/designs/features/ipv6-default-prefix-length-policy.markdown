# Design Note: IPv6 Default Prefix Length Policy

Read when: changing the default meaning of bare detected IPv6 addresses, including IPv6 lifting defaults, WAF IPv6 projection defaults, or exact-address versus network-presence semantics.

Defines: the product policy for choosing the default IPv6 prefix length when the updater has only a bare detected IPv6 address and no more specific operator instruction.

Does not define: provider-specific discovery rules, full host-ID grammar, exact Cloudflare API capability bounds, or ownership and reconciliation rules.

## Goal

Choose a default IPv6 interpretation rule that remains defensible even when the downstream platform supports both exact-address and broader-prefix targets, and that keeps one coherent observed-prefix model across later DNS and WAF derivation.

## Core Policy

When the updater has only a bare detected IPv6 address and no more specific operator instruction, the default semantic boundary is `/64`.

- This is a product default, not a universal truth about IPv6 deployments.
- This is a default interpretation rule under incomplete information.
- This default is chosen for the shared observed-prefix model that sits before resource-specific derivation.
- Explicit operator configuration may choose narrower or broader prefixes.

## Non-Historical Rationale

### Shared Observed-Prefix Model

A bare observed IPv6 address usually carries two different kinds of information at once.

- a prefix that locates the current network attachment
- lower bits that identify one interface within that attachment

The product model therefore cannot stop at "an address string was observed." It needs one default way to lift that observation into an observed prefix plus lower bits before DNS and WAF derive their resource-specific targets.

That shared observed-prefix model matters on both sides of the product.

- WAF-side reasoning often wants stable attachment or network-presence semantics rather than one transient interface identifier.
- DNS-side and host-ID reasoning want lower-bit space that can still be preserved or transformed under the observed prefix.

Future DNS-side derivation is expected to include both preserving observed lower bits and constructor-style host-ID generation such as `mac(...)`. A good default should therefore be judged not only by how it projects to WAF today, but also by whether it leaves one coherent observed-prefix object that DNS and WAF can both use.

### Why `/64` Is The Current Recommended Default

`/64` is the strongest current shared default because the two main lines of reasoning converge there.

- On the WAF side, `/64` usually matches stable attachment or site presence better than one exact interface identifier.
- On the DNS and host-ID side, `/64` keeps ordinary lower-bit derivation space under the observed prefix, so future derivation can preserve lower bits or synthesize a host ID without first changing the meaning of the shared object.
- It is broad enough to survive ordinary interface-identifier churn within one attachment.
- It is narrow enough to avoid defaulting all the way to larger delegated customer space such as `/56` or `/48`.
- In many common single-site deployments, the extra sibling addresses covered by `/64` still fall within the same practical trust boundary, even though that assumption fails in mixed-trust or shared environments.

Under that shared observed-prefix model, the WAF argument and the DNS-side derivation argument are not separate accidents. Today they point to the same default boundary.

That same convergence also justifies one shared configuration knob today, if the product exposes this choice. The common operator question is still one question about the default meaning of a bare observed IPv6 address under incomplete information. In the common deployments this note is optimizing for, DNS-side derivation and WAF-side projection are both different ways of acting on that same attachment-level intent. A split configuration would therefore imply a product distinction that the current use cases do not yet justify.

### Why `/128` Remains A Strong Alternative

`/128` still has serious advantages and remains a good explicit choice.

- It matches exact endpoint identity and least-privilege authorization more directly.
- It is easier to audit because the stored target matches the exact observed address.
- It avoids silently covering sibling addresses that were never observed.
- It is often the better choice in multi-tenant, mixed-trust, point-to-point, or exact-endpoint environments.

Those are real counterarguments, not edge cases to dismiss. Now that downstream platforms can accept exact IPv6 addresses and IPv6 CIDRs up to `/128`, exact-address defaults are again a live product choice.

### Why `/128` Is Not The General Fallback Today

`/128` is still too literal to be the one shared fallback when the updater has only a bare observation and no stronger operator signal.

- It collapses the ordinary host-ID derivation space to zero, so richer DNS-side derivation would often need extra operator choice or a different hidden default before it could do anything beyond "reuse this exact address."
- It makes default attachment-level interpretation brittle when interface identifiers change but the relevant site or network presence has not.
- It weakens the coherence of the shared observed-prefix model by making the default observed object an exact endpoint while other planned derivations still want prefix-plus-host-bits semantics.

This is a reason to prefer `/64` as the default under incomplete information, not a claim that `/128` is wrong in general.

### What This Policy Does Not Claim

- It does not claim every real IPv6 deployment uses `/64`.
- It does not claim `/64` is always the safest authorization boundary.
- It does not claim downstream platform capability should dictate the product default.
- It does not claim the current DNS-side and WAF-side convergence can never split.

## When `/64` Is The Wrong Default

`/64` is the wrong default when the operator's real intent is narrower than stable attachment identity or when a `/64` exceeds the practical trust boundary.

Examples:

- exact per-device allow or block policies
- multi-tenant or mixed-trust environments that share one `/64`
- point-to-point or single-host infrastructure
- explicit least-privilege or exact-endpoint audit requirements

## Relationship To Lifecycle And Resources

This note refines lifecycle derivation under [Lifecycle Model](lifecycle-model.markdown).

- Provider contracts define how bare observations become raw IPv6 data.
- This note defines the default observed-prefix interpretation when that observation lacks a more specific prefix length.
- DNS and WAF may derive different final targets from the resulting observed-prefix object, but the default should keep that shared object coherent.
- One shared default or shared knob is justified as long as DNS-side and WAF-side semantics still express the same product question about the default meaning of a bare observed IPv6 address.
- Downstream support for narrower WAF items does not, by itself, change the default interpretation rule.

## Historical Context

The policy above does not rely on history.

Historical context is still useful because Cloudflare capability changed over time and helped resurface this design question.

- [On September 26, 2024](https://github.com/cloudflare/cloudflare-docs/commit/6c06754db28c82bcd1947cfc73d2a112068d3e67), Cloudflare documentation corrected the documented IPv6 custom-list range from `/4`-`/64` to `/12`-`/64`.
- [On July 8, 2025](https://github.com/cloudflare/cloudflare-docs/commit/5cbe6f1042e3c932c6b8c2001366ef5b908cb0d2), Cloudflare documentation changed again from IPv6 CIDR ranges `/12`-`/64` to individual IPv6 addresses plus IPv6 CIDR ranges `/12`-`/128`.
- That removed the old platform constraint, but it did not settle the product-default question. The default remains a product-semantics choice about how to interpret a bare observation under incomplete information.

## Extension Points

- If future evidence shows that most users mean exact endpoint identity rather than stable network presence, revisit this default.
- If future product work exposes configurable prefix lengths, start from one shared configuration knob. Split it only if DNS-side observed-prefix interpretation and WAF-side projection stop moving together in ordinary use or if the product gains a clear resource-specific reason to prefer different defaults.
- If future DNS-side derivation stops needing ordinary host-bit space under the observed prefix, or future WAF use cases stop preferring attachment-level semantics, treat the current convergence as broken and revisit this note.
- If future design work adopts a different default attachment boundary, update this note and its linked lifecycle or provider notes together.
