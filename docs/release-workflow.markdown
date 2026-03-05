# Release Workflow

This note records maintainer-facing conventions for preparing releases, especially changelog edits and README feature notes.

## Changelog

1. Use the usual release-time process to generate the initial changelog entries.
2. Before rewriting the release notes, inspect the release diff and the latest user-facing documents, especially `README.markdown` and any relevant files in `docs/designs/`.
3. Explain user-visible changes in terms of actual behavior, scope, and current terminology. Do not rely only on pull request titles or commit subjects.
4. Before the release, remove duplicates, tighten wording, and group entries by type.
5. For each release section, verify the version header, compare link, and release date.
6. Before the release, review README notes and wording together: availability notes and experimental markers should match actual feature state; default/opt-in sections should emphasize meaningful operational deltas; wording should stay accessible to not-so-technical users; and intro paragraphs should avoid dense setting-to-setting mapping details (move those to setting tables or dedicated reference sections).
7. Optional but preferred: add one line per user-visible change during pull requests to reduce release-day editing.

## Feature-Note Lifecycle

1. For each newly introduced user-facing feature, add a note in `README.markdown` when the feature needs extra rollout or stability context.
2. Before the first release tag for that feature exists, avoid guessing a future version number. If a note is still useful, label the feature as `unreleased`.
3. After the first release tag exists, replace `unreleased` with a concrete note such as `available since version X.Y.Z`.
4. Keep the availability note for about one year, or a similar release window, then remove it once the feature is no longer meaningfully new.
5. Mark experimental features clearly as experimental and explicitly note that they are subject to change.
6. After about one year, an experimental feature becomes eligible for graduation, but graduation remains an explicit maintainer decision.
7. When graduating a feature, remove the experimental marker and add a release note so users can see that the stability level changed.
