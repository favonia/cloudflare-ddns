# Release Workflow

This note records maintainer-facing conventions for preparing releases, especially changelog edits and README feature notes.

## Changelog

Help operators understand the release's value and prepare for upgrades.

1. Review every commit and the complete diff from the previous release tag, consulting current documentation as needed. Build a working inventory before selecting entries.
2. Group related changes by their final user-visible outcome compared with the previous release. Select material capabilities, fixes, and compatibility changes; check that omissions do not give a misleading picture of the release.
3. Explain each outcome only as far as needed to understand its value or act on it. Include advance guidance when upgrading requires preparation or would otherwise have silent or misleading effects. Clear runtime diagnostics may reduce or eliminate the need for instructions in the changelog.

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
