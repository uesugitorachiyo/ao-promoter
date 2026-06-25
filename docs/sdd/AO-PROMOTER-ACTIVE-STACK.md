# AO Promoter Active Stack

## Active Stack Manifest

The active stack manifest records the AO components currently trusted by the
framework. AO Promoter v0.1 renders a next manifest in dry-run mode instead of
mutating live state.

Required active-stack fields:

- `schema_version`: `ao.promoter.active-stack.v0.1`;
- `stack_id`;
- `created_at_utc`;
- `slots`;
- `previous_stack_ref`;
- `promotion_history`;
- `trust_boundary`.

## Slots

Allowed slots:

- `factory`;
- `orchestrator`;
- `benchmark`;
- `hardening`;
- `policy`;
- `command_surface`;
- `control_plane`;
- `release_gate`;

Each slot contains:

- `slot`;
- `component_id`;
- `version`;
- `source_ref`;
- `activated_by`;
- `activation_gate`;
- `rollback_ref`.

## Activation Plan

`promoter plan activate` writes:

- `schema_version`: `ao.promoter.activation-plan.v0.1`;
- `plan_id`;
- `candidate_id`;
- `target_stack_id`;
- `target_slot`;
- `current_component`;
- `next_component`;
- `required_gate_ref`;
- `rollback_plan_ref`;
- `actions`;
- `dry_run_only`;
- `mutates_live_state`.

In v0.1:

- `dry_run_only` must be true;
- `mutates_live_state` must be false;
- actions are descriptions, not live commands;
- output path must be under `tmp/`.

## Rollback Plan

`promoter rollback plan` writes:

- `schema_version`: `ao.promoter.rollback-plan.v0.1`;
- `rollback_id`;
- `candidate_id`;
- `target_stack_id`;
- `previous_component`;
- `restore_actions`;
- `verification_commands`;
- `dry_run_only`;
- `mutates_live_state`.

Promotion cannot pass unless rollback planning succeeds.

## Apply Dry Run

`promoter apply --dry-run` writes:

- `schema_version`: `ao.promoter.apply-result.v0.1`;
- `status`: `dry_run_complete`;
- `actions_simulated`;
- `mutates_live_state`: false;
- `active_stack_written`: false;
- `operator_approval_required_for_live_apply`: true.

Any apply command without `--dry-run` fails in v0.1.
