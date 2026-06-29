# AO Promoter Contracts

## Contract Families

| Contract | Planned schema path | Purpose |
| --- | --- | --- |
| Candidate | `docs/contracts/promoter-candidate-v0.1.schema.json` | Candidate identity, component kind, version, source, target slot, and trust boundary. |
| Promotion packet | `docs/contracts/promoter-packet-v0.1.schema.json` | Candidate plus all required evidence references and promotion policy. |
| Evidence reference | `docs/contracts/promoter-evidence-ref-v0.1.schema.json` | Evidence path, schema version, SHA-256 digest, freshness, and gate role. |
| Promotion gate | `docs/contracts/promoter-gate-v0.1.schema.json` | Pass/fail decision, blockers, gate matrix, and score summary. |
| Activation plan | `docs/contracts/promoter-activation-plan-v0.1.schema.json` | Proposed active-stack slot updates and dry-run actions. |
| Active stack | `docs/contracts/promoter-active-stack-v0.1.schema.json` | Current or next active AO stack manifest. |
| Rollback plan | `docs/contracts/promoter-rollback-plan-v0.1.schema.json` | Reversal actions and previous stack references. |
| Apply result | `docs/contracts/promoter-apply-result-v0.1.schema.json` | Dry-run apply summary and mutation flag. |
| Safety scan | `docs/contracts/promoter-safety-scan-v0.1.schema.json` | Public-safety scan result and redacted findings. |
| Promotion report | `docs/contracts/promoter-report-v0.1.schema.json` | Machine-readable report summary for Markdown rendering. |
| Live mutation boundary | `docs/contracts/promoter-live-mutation-boundary-v0.1.schema.json` | Dry-run activation boundary for governed live-mutation readiness evidence. |

## Candidate Required Fields

- `schema_version`: `ao.promoter.candidate.v0.1`;
- `candidate_id`;
- `display_name`;
- `component_kind`;
- `version`;
- `source_ref`;
- `target_slot`;
- `target_stack_id`;
- `trust_boundary`;
- `expected_gate_roles`.

Allowed `component_kind` values:

- `factory`;
- `orchestrator`;
- `benchmark`;
- `hardening`;
- `policy`;
- `command_surface`;
- `control_plane`;
- `stack_revision`.

## Promotion Packet Required Fields

- `schema_version`: `ao.promoter.packet.v0.1`;
- `packet_id`;
- `candidate`;
- `current_active_stack`;
- `required_gate_roles`;
- `evidence`;
- `freshness_policy`;
- `promotion_policy`;
- `rollback_required`;
- `dry_run_only`;

`dry_run_only` must be true in v0.1 valid fixtures.

## Required Gate Roles

The canonical v0.1 packet requires:

- `arena_promotion_gate`;
- `crucible_hardening_gate`;
- `covenant_policy_decision`;
- `foundry_goal_readiness`;
- `forge_packet_summary`;
- `ao2_run_summary`;
- `public_safety_scan`;
- `rollback_plan_ready`.

## Live Mutation Boundary Required Evidence

`promoter live-mutation boundary` requires:

- Covenant live-mutation authority with status `approved`;
- Foundry live-mutation request packet with status `ready`;
- Forge live-mutation dry-run plan with status `ready`;
- AO2 live-mutation dry-run packet with status `ready`;
- Sentinel live-mutation hold verdict with status `clear` and no hold;
- rollback rehearsal with status `ready`;
- AO Command live-mutation readback with status `ready` and armed kill-switch.

The boundary output uses `ao.promoter.live-mutation-boundary.v0.1`. It remains
dry-run only and must not mutate repositories, schedule work, execute work,
approve work, call providers, release, or publish.

## Evidence Reference Required Fields

- `role`;
- `path`;
- `schema_version`;
- `sha256`;
- `status`;
- `candidate_id`;
- `created_at_utc`;
- `expires_at_utc`;
- `authority`;

Allowed statuses:

- `passed`;
- `ready`;
- `allowed`;
- `verified`;
- `blocked`;
- `failed`.

## Valid Fixtures

- `examples/candidates/valid/ao-foundry-candidate.json`
- `examples/packets/valid/ao-promoter-v0.1.json`
- `examples/active/valid/current-active-stack.json`
- `examples/evidence/valid/arena-promotion-gate.json`
- `examples/evidence/valid/crucible-hardening-gate.json`
- `examples/evidence/valid/covenant-policy-decision.json`
- `examples/evidence/valid/foundry-goal-readiness.json`
- `examples/evidence/valid/forge-packet-summary.json`
- `examples/evidence/valid/ao2-run-summary.json`
- `examples/evidence/valid/public-safety-scan.json`
- `examples/live-mutation/valid/live-mutation-boundary.passed.json`
- `examples/live-mutation/valid/covenant-authority.approved.json`
- `examples/live-mutation/valid/foundry-request.ready.json`
- `examples/live-mutation/valid/forge-plan.ready.json`
- `examples/live-mutation/valid/ao2-packet.ready.json`
- `examples/live-mutation/valid/sentinel-hold.clear.json`
- `examples/live-mutation/valid/rollback-rehearsal.ready.json`
- `examples/live-mutation/valid/command-status.ready.json`

## Invalid Fixtures

- `examples/packets/invalid/missing-crucible-gate.json`
- `examples/packets/invalid/stale-arena-gate.json`
- `examples/packets/invalid/digest-mismatch.json`
- `examples/packets/invalid/candidate-id-mismatch.json`
- `examples/packets/invalid/live-apply-default.json`
- `examples/evidence/invalid/failed-crucible-gate.json`
- `examples/evidence/invalid/unsafe-public-scan.json`
- `examples/candidates/invalid/unknown-target-slot.json`
- `examples/live-mutation/invalid/live-mutation-boundary.failed.json`
- `examples/live-mutation/invalid/sentinel-hold.required.json`
- `examples/live-mutation/invalid/ao2-packet.forbidden-authority.json`

## Validation Rules

- Reject unknown schema versions.
- Reject evidence references without SHA-256 digests.
- Reject evidence whose digest does not match the referenced file.
- Reject stale evidence based on `expires_at_utc`.
- Reject missing required gate roles.
- Reject candidate ID mismatches between candidate and evidence.
- Reject `dry_run_only: false` in v0.1 default fixtures.
- Reject promotion when rollback is required but no rollback plan can be built.
- Reject local absolute paths and secret-like values in durable examples.
- Reject live-mutation activation when Sentinel holds, Command kill-switch is
  not armed, rollback evidence is missing, or any input expands mutation,
  scheduling, execution, approval, provider, release, or publication authority.
