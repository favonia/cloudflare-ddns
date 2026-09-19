# Release Workflow

This note records maintainer-facing conventions for preparing releases, especially changelog edits and README feature notes.

## Changelog

The changelog helps operators understand the release's value and make informed upgrade and configuration decisions.

1. **Establish the evidence.** Review every commit in the release range and the complete diff from the previous release tag to the tree being prepared. Consult `README.markdown` and relevant design documents for current behavior and terminology. Keep a working inventory of the changes and their effects.
2. **Synthesize and select.** Group related commits into final user-visible outcomes, checking both versions' final contents so superseded changes and development-cycle fixes are accounted for. Select outcomes that materially help readers understand what the release offers or what it requires of them. A verified difference or an existing draft entry does not by itself justify inclusion; an omitted change should not leave a materially misleading picture of the release.
3. **Choose the explanation.** Describe each selected outcome at the level readers need to understand its value or act on it. When runtime diagnostics clearly explain a problem and its remedy, a brief description of the improvement usually suffices. Give advance guidance when readers need to prepare before upgrading or when an effect would otherwise be silent or misleading. The existence of a diagnostic alone determines neither inclusion nor omission.
4. **Review the whole release.** Check coverage and relevance together: preserve material outcomes and remove detail that does not help the reader's release-level decisions or expectations. Organize the selected outcomes for those readers, combining overlap. When feedback identifies an omission or excessive detail, reconsider the affected outcome in the context of the whole release rather than applying a blanket inclusion or omission rule.

## Release Checks

- Before publication, verify the version header, compare link, and release date. Check README availability notes and experimental markers against the release being prepared; follow [README Writing](design/guides/readme-writing.markdown) for README conventions.
- Verify that features still marked experimental in `README.markdown` emit a warning when enabled.
- After publication, verify published artifacts, documented image tags, and signatures.

## Feature-Note Lifecycle

1. For each newly introduced user-facing feature, add a note in `README.markdown` when the feature needs extra rollout or stability context.
2. During development, label a feature as `unreleased` until it is included in preparation for a specific release.
3. When preparing that release, replace its features' `unreleased` markers with `available since version X.Y.Z` in the same change as the changelog, so the tagged README describes the release being shipped. If the release scope or version changes, update both documents together.
4. Keep the availability note for about one year, or a similar release window, then remove it once the feature is no longer meaningfully new.
5. Mark experimental features clearly as experimental and explicitly note that they are subject to change.
6. After about one year, an experimental feature becomes eligible for graduation, but graduation remains an explicit maintainer decision.
7. When graduating a feature, remove the experimental marker and add a release note so users can see that the stability level changed.
