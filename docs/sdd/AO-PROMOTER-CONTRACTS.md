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
| Live docs mutation boundary | `docs/contracts/promoter-live-docs-mutation-boundary-v0.1.schema.json` | Dry-run promotion boundary for the first approved docs-only live class. |

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
- rollback rehearsal with status `ready` and class-bound rollback proof;
- AO Command live-mutation readback with status `ready`, armed kill-switch,
  current class, next class, completed live rehearsal evidence, clean `main` CI,
  and no active holds.

The boundary output uses `ao.promoter.live-mutation-boundary.v0.1` and includes
`current_mutation_class`, `next_mutation_class`,
`class_promotion_readiness`, and `safe_to_promote_next_class`. Class promotion
requires the next class to be the immediate governed successor; skipped classes
are denied. It remains dry-run only and must not mutate repositories, schedule
work, execute work, approve work, call providers, release, or publish.
For the `test_only` to `low_risk_code` successor, `class_promotion_readiness`
also includes `promotion_prerequisites` and
`promotion_prerequisite_requirements`. These require successful `test_only`
live evidence, a `test_only` rollback fixture, Sentinel clear verdict for
`low_risk_code`, clean `main` CI, an exact `low_risk_code` Covenant class
ticket, and read-only AO Command status. A wrong-class Covenant ticket denies
promotion even when all other dry-run evidence is ready.
For the `low_risk_code` to `multi_repo_low_risk` successor, the same readiness
object reports `highest_proven_live_class`,
`current_class_live_evidence_status`, `next_denied_class`, and
`next_denied_reason`, plus ordered merge plan, per-repo rollback, per-repo CI,
fresh repo-state, and kill-switch prerequisites. Without a completed
`low_risk_code` live rehearsal, Promoter keeps the highest proven live class at
`test_only` and denies `multi_repo_low_risk` promotion even if dry-run artifacts
are otherwise ready in that fixture. Later closure evidence kept
`fully_unsupervised_complex_mutation` proven for the governed 26-node first
non-planning rehearsal boundary. The current highest proven live class is
`public_safe_guided_evidence_application_four_attempts`, proven only for
public-safe guided evidence-application evidence showing causal-review guidance
can select and prioritize later bounded evidence attempts under independent
gates;
the next denied class is `broad_RSI`.

The bounded RSI self-improvement application verdict is narrower than the
live-mutation successor ladder. It records
`bounded_rsi_self_improvement_application` as proven only for the exact private
readback/eval rubric rehearsal. `broad_RSI` remains denied, unrestricted
self-modification remains denied, hidden instruction mutation remains denied,
and policy/auth/secret/provider/deploy/release/config/dependency expansion
remains denied. The verdict does not claim broad RSI or policy-changing
autonomy.

The exact safe public claim wording verdict is narrower still. It records
`exact_safe_public_claim_wording_conservative_readback_evidence` as proven only
for this approved wording: "AO has public-safe tracked readback evidence for
bounded improvement-claim review and retraction rehearsal; stronger
recursive-improvement claims remain denied." `broad_RSI`, unrestricted
self-modification, hidden instruction mutation, policy-changing autonomy, and
stronger recursive-improvement claims remain denied. The verdict does not claim
broad RSI or policy-changing autonomy.

The causal-review evidence-selection guidance verdict records
`public_safe_causal_review_evidence_selection_guidance` as proven only for this
approved wording: "AO has public-safe causal-review evidence that prior bounded
evidence can guide later evidence-selection and blocker prioritization under
independent review gates; stronger recursive-improvement wording and broad_RSI
remain denied." `broad_RSI`, stronger recursive-improvement wording,
unrestricted self-modification, hidden instruction mutation, and policy-changing
autonomy remain denied. This remains prior evidence. The verdict does not claim
broad RSI or policy-changing autonomy.

The guided evidence-application verdict records
`public_safe_guided_evidence_application_four_attempts` as proven only for this
approved wording: "AO has public-safe guided evidence-application evidence
showing causal-review guidance can select and prioritize later bounded evidence
attempts under independent gates; stronger recursive-improvement wording and
broad_RSI remain denied." `broad_RSI`, stronger recursive-improvement wording,
unrestricted self-modification, hidden instruction mutation, and policy-changing
autonomy remain denied. The verdict does not claim broad RSI or policy-changing
autonomy.

`public_safe_intermediate_causal_review_claim_evidence` remains prior evidence
from AO Foundry PR #189, commit
`860e3f353ab833c4a671b9d0ee6d8101ece2815c`, with tracked public evidence under
`docs/evidence/recursive-improvement-safe-intermediate-claim/`. The approved public wording is exactly: "AO has public-safe intermediate causal-review evidence that bounded improvement evidence can guide and constrain later claim review across independent roles; stronger recursive-improvement wording and broad_RSI remain denied." Stronger recursive-improvement wording remains denied, `broad_RSI` remains denied, unrestricted self-modification remains denied, hidden instruction mutation remains denied, and policy-changing autonomy remains denied.

`public_safe_causal_review_evidence_selection_guidance` is proven from AO Foundry
PR #191, commit `413b70f15d8f3d0203dc7be076914a2f3b539881`, with tracked public
evidence under `docs/evidence/recursive-improvement-evidence-selection-guidance/`.

`public_safe_guided_evidence_application_four_attempts` is proven from AO
Foundry PR #193, commit `4ec509fd64d1fc1ea41ea7f22aae900ba79e09a1`, with
tracked public evidence under
`docs/evidence/recursive-improvement-guided-evidence-application/`. The approved
public wording is exactly: "AO has public-safe guided evidence-application
evidence showing causal-review guidance can select and prioritize later bounded
evidence attempts under independent gates; stronger recursive-improvement
wording and broad_RSI remain denied." Stronger recursive-improvement wording,
`broad_RSI`, unrestricted self-modification, hidden instruction mutation, and
policy-changing autonomy remain denied.

`public_safe_broad_RSI_governed_campaign_segment_07_evidence` is proven from AO
Foundry PR #210, commit `8f8ac5f8f74d942c7a02a6c2dd39a7c974872bb6`, with
tracked public evidence under `docs/evidence/broad-rsi-ten-day-campaign-segment-07/`.
Promoter may read back only
`promote_public_safe_broad_RSI_governed_campaign_segment_07_evidence_broad_RSI_denied`.
The approved wording is segment-07 evidence only; `broad_RSI`, full 10-day
campaign completion, unrestricted self-modification, hidden instruction
mutation, policy-changing autonomy, and forbidden surface expansion remain
denied.

## First Live Docs Boundary Required Evidence

`promoter live-mutation docs-boundary` requires:

- Covenant docs-only approval ticket with status `approved`, exact docs-only scope, approver identity, and `consumed=false`;
- Foundry live docs approval gate with status `ready` and `safe_to_execute=true`;
- Forge live docs execution guard with status `ready`, docs-only allowlist enforcement, and rollback requirement;
- AO2 docs-only patch packet with status `ready`, dry-run apply evidence, and rollback patch evidence;
- Sentinel live docs hold verdict with status `clear` and no hold;
- Foundry rollback execution rehearsal with status `ready` and `rollback_verified=true`;
- AO Command live docs readback with status `ready`, `operator_mode=read_only`, and armed kill-switch.

The boundary output uses `ao.promoter.live-docs-mutation-boundary.v0.1`. It is
still dry-run only: passing it does not mutate repositories, execute work,
approve work, call providers, release, upload, or publish, and it does not claim
broad or fully unsupervised complex mutation.

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
- `examples/live-mutation/invalid/command-status.missing-live-rehearsal.json`
- `examples/live-mutation/invalid/command-status.main-ci-failed.json`
- `examples/live-mutation/invalid/rollback-rehearsal.missing-proof.json`

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
- Deny mutation-class promotion when completed live rehearsal evidence,
  class-bound rollback proof, clean `main` CI, or clear hold evidence is absent.

## Governed Broad RSI Campaign Completion Readback

`broad_RSI` is proven from AO Foundry PR #211, commit `630edc70905db745380edd1072e04b546dcccfe3`, with tracked public evidence under `docs/evidence/broad-rsi-ten-day-campaign-segment-08/`. The approved public wording is exactly: "AO has proven governed broad_RSI for public claim publication across the AO stack public-safe 10-day evidence campaign; unrestricted self-modification, hidden instruction mutation, policy-changing autonomy, and forbidden surface expansion remain denied." Campaign completion is `2800 / 2800` nodes. `Promoter` reads back `highest_proven_live_class=broad_RSI` and `next_denied_class=unrestricted_self_modification`.

This does not prove unrestricted self-modification, hidden instruction mutation, policy-changing autonomy, policy/auth/secret/provider/deploy/release/config/dependency expansion, release/deploy/publish/upload/tag/provider calls, credential use, direct main mutation, concurrent mutation, or any unrestricted RSI claim.

## Unrestricted Self-Modification Sandbox Containment Readback

Promoter promotes only the narrow sandbox-containment class:
`public_safe_unrestricted_self_modification_sandbox_containment_rehearsal`. Evidence
comes from AO Foundry PR #216, commit
`7881613065de48f2547833a9ecc9a9011b55a96a`, with tracked public evidence under
`docs/evidence/unrestricted-self-modification-sandbox-containment/`. The Promoter
verdict is
`promote_public_safe_unrestricted_self_modification_sandbox_containment_rehearsal_keep_unrestricted_self_modification_denied`.
The approved public wording is exactly: "AO has public-safe sandbox containment evidence for
dry-run self-change proposal evaluation; unrestricted
self-modification, hidden instruction mutation, policy-changing autonomy, and
forbidden surface expansion remain denied."

This does not prove unrestricted self-modification, hidden instruction mutation,
policy-changing autonomy, policy/auth/secret/provider/deploy/release/config/
dependency expansion, credential use, provider calls,
release/deploy/publish/upload/tag authority, dependency update authority, direct
main mutation, concurrent mutation, hidden instruction changes, or any
unrestricted RSI claim.

## Unrestricted Self-Modification Adversarial Negative Controls Readback

Promoter preserves the narrow adversarial negative-control class as prior
evidence:
`public_safe_unrestricted_self_modification_adversarial_negative_controls`.
Evidence comes from AO Foundry PR #217, commit
`b7e487022ae7436be13e0a49d0bf15f5c7936145`, with tracked public evidence under
`docs/evidence/unrestricted-self-modification-adversarial-negative-controls/`.
The Promoter verdict is
`promote_public_safe_unrestricted_self_modification_adversarial_negative_controls_keep_unrestricted_self_modification_denied`.
The approved public wording is exactly: "AO has public-safe adversarial
negative-control evidence that unsafe dry-run self-change proposals are
rejected under sandbox containment gates; unrestricted self-modification,
hidden instruction mutation, policy-changing autonomy, and forbidden surface
expansion remain denied."

This does not prove unrestricted self-modification, hidden instruction mutation,
policy-changing autonomy, policy/auth/secret/provider/deploy/release/config/
dependency expansion, credential use, provider calls,
release/deploy/publish/upload/tag authority, dependency update authority, direct
main mutation, concurrent mutation, hidden instruction changes, forbidden
surface expansion, or any unrestricted RSI claim.

## Unrestricted Self-Modification Bounded Reversible Application Readback

Promoter promotes only the narrow bounded reversible application class:
`public_safe_bounded_reversible_self_change_application_rehearsal`.
Evidence comes from AO Foundry PR #218, commit
`3b2feaced4207c97f98cef44f3b3276c59a7873b`, with tracked public evidence under
`docs/evidence/unrestricted-self-modification-bounded-reversible-application/`.
The Promoter verdict is
`promote_public_safe_bounded_reversible_self_change_application_rehearsal_keep_unrestricted_self_modification_denied`.
The approved public wording is exactly: "AO has public-safe bounded reversible
self-change application evidence for one exact-scope support/readback
improvement under sandbox containment gates; unrestricted self-modification,
hidden instruction mutation, policy-changing autonomy, and forbidden surface
expansion remain denied."

This proves only one exact-scope reversible support/readback evidence
improvement under sandbox containment gates. It does not prove unrestricted
self-modification, hidden instruction mutation, policy-changing autonomy,
forbidden surface expansion, policy/auth/secret/provider/deploy/release/config/
dependency expansion, credential use, provider calls,
release/deploy/publish/upload/tag authority, dependency update authority, direct
main mutation, concurrent mutation, hidden instruction changes, forbidden
surface expansion, or any unrestricted RSI claim.
