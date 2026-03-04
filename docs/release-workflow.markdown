# Release Workflow

This note records maintainer-facing conventions for preparing releases, especially changelog edits and README feature notes.

## Changelog

1. Use the usual release-time process to generate the initial changelog entries.
2. Before the release, remove duplicates, tighten wording, and group entries by type.
3. For each release section, verify the version header, compare link, and release date.
4. After cutting the release, restore the `[Unreleased]` section for upcoming changes.
5. Optional but preferred: add one line per user-visible change during pull requests to reduce release-day editing.
6. Before the release, verify that README availability notes and experimental-feature notes still match the actual feature state.

## Feature-Note Lifecycle

1. For each newly introduced user-facing feature, add an availability note in `README.markdown`.
2. Before the first release tag for that feature exists, say `unreleased` instead of guessing a future version number.
3. After the first release tag exists, switch to a concrete note such as `available since version X.Y.Z`.
4. Keep the availability note for about one year, or a similar release window, then remove it once the feature is no longer meaningfully new.
5. Mark experimental features clearly as experimental and explicitly note that they are subject to change.
6. After about one year, an experimental feature becomes eligible for graduation, but graduation is still an explicit maintainer decision.
7. When graduating a feature, remove the experimental marker and add a release note so users can see that the stability level changed.
