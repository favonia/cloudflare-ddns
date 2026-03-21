# Design Note: Project Principles

Read when: making design tradeoffs or choosing between competing implementations.

Defines: the project-wide priorities that should influence many tasks.

## Decision Tree

Apply this strictly ordered list from top to bottom.

Use only the criteria written in this tree.

### 0. Required Behavior

- Provide the behavior the maintainer wants.

### 1. Practical Security

- Favor memory safety and a small attack surface.
- Prefer open, inspectable designs over obscurity.
- Prefer designs that make bugs and misconfigurations easier to detect through analysis, tests, fuzzing, validation, or clear operator feedback.

### 2. Resilience

- Prefer automatic recovery from transient failures, startup delays, remote instability, and interrupted runs.
- Prefer designs whose partial progress leaves the system in a safer and more recoverable state.

### 3. Critical Efficiency

- Prefer lower network, CPU, memory, and operational cost at critical spots.

### 4. Operator Clarity

- Explain behavior through observable outcomes, operator decisions, and actionable next steps.
- Mention internal mechanisms only when they change operator decisions.

### 5. Principled Design

- Prefer designs whose local decisions are determined by global principles and local context over ad hoc local decisions.

### 6. Maintainability

- Prefer choices that reduce long-term maintenance burden.
