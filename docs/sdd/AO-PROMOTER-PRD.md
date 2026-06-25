# AO Promoter PRD

## Product Summary

AO Promoter is the gated path from a candidate AO component, release, or stack
revision into the active AO stack. It does not decide whether the candidate is
good in isolation. Instead, it verifies that every required evaluator and policy
gate has already produced current, public-safe, machine-readable evidence.

AO Promoter exists because recursive system improvement needs a final promotion
boundary. AO Arena can prove comparative performance, AO Crucible can prove
hardening resilience, AO Covenant can prove policy and safety, and AO
Foundry/AO Forge can prove goal execution. AO Promoter turns those independent
signals into one explicit promotion decision and one reversible activation plan.

## Users

| User | Job |
| --- | --- |
| AO framework maintainer | Promote a candidate into the active stack only after evidence closes. |
| Release reviewer | Inspect why a candidate was promoted, blocked, or rolled back. |
| Operator | Run a dry-run promotion locally without live credentials. |
| AO Foundry loop | Consume promotion decisions before switching active candidates. |
| AO Forge executor | Implement blocked-promotion remediation tasks from the decision report. |
| Future control-plane observer | Read promotion summaries without approving or mutating state. |

## v0.1 Goals

1. Validate candidate profiles and promotion packets.
2. Evaluate required gates from Arena, Crucible, Covenant, Foundry, Forge, AO2,
   and public-safety evidence.
3. Emit a deterministic promotion gate result.
4. Emit an activation plan that describes how the active stack would change.
5. Render the next active-stack manifest from an activation plan.
6. Emit a rollback plan before promotion can pass.
7. Render a public-safe Markdown promotion report.
8. Run default `apply` only in dry-run mode.
9. Block promotion when any required gate is stale, missing, unsafe, failed,
   unsigned when signature is required, or inconsistent with the candidate.

## Non-Goals

- Do not run live model providers in v0.1 default paths.
- Do not push, tag, release, upload, deploy, or mutate sibling repositories.
- Do not replace AO Arena, AO Crucible, AO Covenant, AO Foundry, AO Forge, AO2,
  AO Command, or ao2-control-plane.
- Do not infer approval from free-form text.
- Do not promote from aggregate score alone.
- Do not store secrets, private prompts, local absolute paths, or unredacted
  evidence in durable public artifacts.

## Success Metrics

AO Promoter v0.1 is successful when:

- the canonical promotion packet validates;
- failed, stale, missing, mismatched, and unsafe evidence fixtures fail closed;
- promotion passes only when all required gates are ready;
- the activation plan is deterministic and reversible;
- the rollback plan is created before the hard gate passes;
- public safety scans pass over README, docs, examples, cmd, and internal;
- a clean clone can run the full dry-run promotion gate;
- a junior engineer can implement the CLI from the SDD without inventing command
  semantics or contract shapes.

## Production Readiness Definition

The SDD is implementation-ready when the AO2 SDD plan validates and every
command, contract, fixture, gate, and final verification command is specified.
The product is production-ready when the implemented repository passes tests,
vet, JSON validation, dry-run promotion gate, active-stack render, rollback
planning, report rendering, public-safety scans, and clean-clone smoke.
