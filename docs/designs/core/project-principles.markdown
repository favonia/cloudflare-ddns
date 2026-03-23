# Design Note: Project Principles

Read when: making design tradeoffs or choosing between competing implementations.

Defines: the project-wide priorities that should influence many tasks.

## Decision Tree

- Apply this strictly ordered list from top to bottom.
- Use only the criteria written in this tree.
- A fact may matter at more than one step.
- When the same fact affects multiple steps, the highest-priority affected step decides.

Examples:

- If a clearer configuration surface is more likely to weaken protection through operator error, `Pragmatic Security` decides before `Operator Clarity`.
- If a lower-request strategy weakens transient-failure recovery, `Resilience` decides before `Critical Efficiency`.

### 0. Required Behavior

- Provide the behavior the maintainer wants.

### 1. Pragmatic Security

- Avoid unnecessary security risk without making supported deployments unusable.
- Do not rely on obscurity for security.
- Detect bugs through analysis, tests, fuzzing, or formal verification.

### 2. Resilience

- Recover automatically from transient failures, startup delays, remote instability, and interrupted runs.
- Keep partial progress in a safer and more recoverable state.

### 3. Critical Efficiency

- Reduce network, CPU, memory, and operational cost at critical spots.

### 4. Operator Clarity

- Detect operator misconfigurations early.
- Explain behavior through observable outcomes, operator decisions, and actionable next steps.
- Mention internal mechanisms only when they change operator decisions.

### 5. Principled Design

- Let global principles and local context determine local decisions instead of ad hoc choices.

### 6. Maintainability

- Reduce long-term maintenance burden.
