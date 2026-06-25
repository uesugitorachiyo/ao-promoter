# AO Promoter SDD Planner Prompt

Use this prompt when asking an SDD planner to generate or improve AO Promoter.

```text
Create an `ao2.sdd-plan.v1` plan for AO Promoter.

Context:
- AO Promoter is the gated promotion path from candidate to active AO stack.
- It consumes evidence from AO Arena, AO Crucible, AO Covenant, AO Foundry, AO
  Forge, and AO2.
- The v0.1 implementation is a local-first Go CLI.
- Default execution is fixture and dry-run only.
- The product must support Ubuntu, macOS, and Windows.
- The plan must not require live providers, network mutation, credentials,
  sibling repository mutation, push, tag, release, upload, or deploy.

Required SDD outputs:
- PRD;
- architecture;
- contracts;
- gate matrix;
- active-stack model;
- safety model;
- implementation slices;
- acceptance gates;
- handoff prompt;
- AO2-valid plan JSON.

The plan must be concrete enough that a junior engineer can implement the Go CLI
without inventing command semantics, schema families, fixture names, evidence
roles, promotion blockers, active-stack fields, rollback behavior, dry-run apply
rules, safety rules, or final verification commands.
```
