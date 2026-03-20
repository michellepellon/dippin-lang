# Context Summary (fidelity: summary:high)

## graph.default_fidelity
summary:high

## tool_stdout
has_next

## graph.max_restarts
7

## graph.rankdir
LR

## graph.default_max_retry
3

## graph.goal
Build the Dippin toolchain (parser, validator, formatter, DOT exporter, migration tool) by reading the design spec and iteratively implementing components into the dippin-lang Go module. Ledger-driven: picks the next uncompleted component automatically and loops until all are done.

---

You are building the Dippin toolchain. Read .tracker/current_context.md to see which component is in_progress and the current state of the codebase.

Read the design spec at ../DIPPIN_DESIGN_PLAN.md — this is the authoritative specification.

Read the current codebase to understand what already exists (especially ir/ types).

Produce a focused implementation plan for THIS component only:
1. List the exact files to create/modify
2. List the types and functions to implement
3. List 10+ test cases (happy path, error cases, edge cases)
4. Reference the specific design spec sections that govern this component
5. Note any dependencies on existing packages

Write the plan to .tracker/current_plan.md so implementation agents can read it.

Do NOT implement yet — just plan. Be precise enough that an implementation agent can work from this plan without ambiguity.