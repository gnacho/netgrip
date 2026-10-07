# Architecture Decision Records

Short, numbered records of the decisions that shape NetGrip: the question
each one answered, the alternatives that lost, and what the decision rules
out. Write a new one when a change would be easier to review as a decision
than as code.

| ADR | Decision |
|-----|----------|
| [000](ADR/000-template.md) | Template |
| [001](ADR/001-single-binary-packaging.md) | Single Go binary, packaged for OpenWrt |
| [002](ADR/002-authentication.md) | One admin, one session, one MCP token |
| [003](ADR/003-executor.md) | Every write goes through the executor |
| [004](ADR/004-advanced-mode.md) | Advanced mode is an opt-in gate, not a visibility trick |
| [005](ADR/005-demo-mode.md) | Demo mode is a frontend contract, not a server mode |
