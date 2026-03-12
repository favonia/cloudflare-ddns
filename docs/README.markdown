# Documentation

This directory holds the public documentation set beyond the top-level `README.markdown`.

## Top-Level Documents

- `docs/CONTRIBUTING.markdown`: contributor workflow and expectations.
- `docs/CODE_OF_CONDUCT.markdown`: community conduct rules.
- `docs/SECURITY.md`: supported versions and vulnerability reporting.
- `docs/release-workflow.markdown`: maintainer release and feature-note conventions.
- `docs/designs/`: durable design documents for future developers, including AI agents.

Use `docs/designs/` for the current intended design of the codebase, including durable constraints, invariants, and intended extension points.

The `docs/designs/` collection should be self-contained. Documents there may refer to each other, but they should not depend on private planning material outside this collection.

Start with [`docs/designs/README.markdown`](designs/README.markdown) for the retrieval map. Notes under `docs/designs/core/` are the small always-read set; notes under `docs/designs/guides/` and `docs/designs/features/` are task-triggered references.
