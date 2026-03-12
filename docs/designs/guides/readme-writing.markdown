# Design Note: README Writing

Read when: editing `README.markdown`.

Defines: the shared writing rules for beginner-facing README content.

Does not define: feature semantics beyond what the README needs to explain.

## Priorities

1. Put beginners first.
   - Favor plain, concrete English, clear examples, and visible user outcomes over internal shorthand.
2. Optimize for first adoption.
   - In snippets and nearby prose, explain what happens when users start using the updater for domains, records, or lists that may already exist in Cloudflare.
   - When that is the point, say "already exist" directly instead of rewriting it into internal-process wording such as "already managed" or "already updated".
3. Use two layers when precision would otherwise hurt readability.
   - Keep the first layer simple in snippets and nearby prose.
   - Put exact reconciliation rules, edge cases, and implementation-level caveats in tables, advanced notes, or `🤖` paragraphs.
   - Exception: if a caveat materially affects first adoption and a beginner may reasonably stop reading after the example, keep a short warning in the example itself instead of moving it only to later prose.
4. Avoid internal terms unless they are necessary for accuracy.
   - Terms such as "managed", "matching", and "configured value" are acceptable in technical sections, but should not replace simpler beginner wording when simpler wording stays accurate enough.
5. Keep words stable in meaning.
   - In the README, words such as `optional` should keep one clear meaning.
   - `Optional` describes whether a setting is required to configure the updater, not whether changing existing resources is optional.
6. Prefer local clarity over global deduplication.
   - Deduplicate long explanations, not required setup facts.
   - If a reader cannot complete the setup correctly without a short fact such as a required permission, prerequisite, or one-time manual step, repeat that fact at the point of use.
7. Prefer explicit references over positional references.
   - Use concrete setting names, section names, or direct links when pointing elsewhere in the README.
   - Avoid vague wording such as "above" or "below" unless the referent is immediate and unlikely to drift, such as the next table or the previous row.
8. Keep advanced features in advanced sections.
   - Mention advanced features briefly in beginner-facing sections only when needed for discoverability or a required prerequisite.
   - Keep the detailed guidance in special-setup sections, reference tables, or technical notes.
9. Keep labels and emojis semantically strict.
   - Use `⚠️` only for information a reader must notice to avoid a wrong setup, broken behavior, or misleading result.
   - Use `🧪` only for experimental features.
   - Do not add labels merely to balance positive and negative prose.
