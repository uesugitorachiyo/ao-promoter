# AO Promoter Gates

## Gate Matrix

The canonical v0.1 gate matrix requires every gate to pass. No weighted average
can override a failed required gate.

| Gate role | Accepted status | Blocks when |
| --- | --- | --- |
| `arena_promotion_gate` | `passed` | missing, stale, failed, wrong candidate, or digest mismatch |
| `crucible_hardening_gate` | `passed` | missing, stale, failed, score below threshold, or digest mismatch |
| `covenant_policy_decision` | `allowed` | missing, denied, revoked, stale, or unsigned when signature is required |
| `foundry_goal_readiness` | `ready` | missing, not ready, stale, or wrong goal/candidate |
| `forge_packet_summary` | `verified` | missing, failed, stale, or references unverified work |
| `ao2_run_summary` | `passed` | missing, failed, stale, or mutates artifacts outside authority |
| `public_safety_scan` | `passed` | missing, failed, or has non-zero findings |
| `rollback_plan_ready` | `ready` | missing rollback plan, invalid previous stack, or non-reversible action |

## Promotion Decision

`promoter gates evaluate` emits:

- `schema_version`: `ao.promoter.gate.v0.1`;
- `status`: `passed` or `failed`;
- `candidate_id`;
- `target_stack_id`;
- `gate_results`;
- `blockers`;
- `required_followups`;
- `promotion_allowed`;
- `activation_plan_allowed`.

Promotion passes only when:

- all required gate roles are present;
- every required gate has an accepted status;
- every evidence digest matches;
- every evidence item is fresh;
- every evidence item references the same candidate ID;
- public safety has zero findings;
- rollback can be planned;
- packet is dry-run-only in v0.1.

## Blocker Semantics

Blockers are structured and stable. Each blocker includes:

- `blocker_id`;
- `gate_role`;
- `severity`;
- `reason`;
- `evidence_path`;
- `recommended_action`.

Critical blockers:

- missing required gate;
- failed Crucible hardening gate;
- failed public-safety scan;
- digest mismatch;
- candidate mismatch;
- disabled dry-run guard;
- missing rollback plan.

High blockers:

- stale evidence;
- missing optional report surface;
- inconsistent active-stack slot metadata.

## Gate Examples

Passing example:

- Arena status: `passed`;
- Crucible status: `passed`;
- Covenant status: `allowed`;
- Foundry status: `ready`;
- Forge status: `verified`;
- AO2 status: `passed`;
- Safety status: `passed`;
- Rollback status: `ready`;
- Decision: `passed`.

First docs-only live mutation boundary example:

- Covenant docs-only approval ticket: `approved`, unexpired, exact-scope;
- Foundry approval gate: `ready`;
- Forge live-docs guard: `ready`;
- AO2 docs-only patch packet: `ready`, with exact changed-file list and
  rollback patch;
- Sentinel verdict: `clear`;
- Foundry rollback execution rehearsal: `ready`;
- AO Command readback: `ready`, `operator_mode=read_only`;
- Decision: `passed` for the exact approved docs-only PR rehearsal scope only.
  The decision does not apply patches, create branches, publish, release, call
  providers, or approve fully unsupervised complex live mutation.

Failed example:

- Crucible status: `failed`;
- Safety status: `passed`;
- Rollback status: `ready`;
- Decision: `failed`;
- Reason: hardening gate cannot be bypassed by other passing gates.
