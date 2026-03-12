# Design Note: SHOUTRRR Input Format

Read when: changing `SHOUTRRR` parsing or validation behavior.

Defines: the `SHOUTRRR` input contract and the handling of suspicious single-line values.

Does not define: downstream shoutrrr runtime behavior after parsing succeeds.

## Contract

`SHOUTRRR` is a newline-separated list of shoutrrr URLs.

The parser preserves each configured line as one URL. It does not rewrite one line into multiple URLs.

## Suspicious Space Handling

A single configured line that contains raw ASCII space characters is treated as suspicious.

The detector analyzes only raw ASCII spaces:

- it splits only on the space character `U+0020`
- it ignores empty segments created by repeated spaces
- it trims surrounding Unicode whitespace from each candidate segment before checking whether that segment is URL-like

Each line is classified as follows:

- `clean`: the line contains no raw ASCII space characters
- `warn-and-proceed`: only the first space-separated segment is URL-like, and the whole line is also URL-like
- `fail`: any other line containing raw ASCII space characters

Warnings are emitted only if every line is either `clean` or `warn-and-proceed`. If any line fails, the parser emits only the hard-error path and returns failure.

## Rationale

This policy preserves single-line values that still parse as one URL while rejecting inputs that look like multiple URLs folded onto one line.

The design intentionally avoids two failure modes:

- silently rewriting one ambiguous line into multiple URLs
- deferring a folded multi-URL mistake until downstream shoutrrr runtime behavior
