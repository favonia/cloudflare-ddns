# Scripts

This directory contains repository-local helper tooling that is intentionally kept separate from the main application code.

- `github-actions/` holds GitHub Actions-only helper tools. Each tool lives in its own subdirectory with its own `go.mod` so workflow dependencies stay isolated from the main application module.
