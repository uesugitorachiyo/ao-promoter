# AO Promoter

AO Promoter is the gated promotion path from an AO candidate to the active AO
stack. It consumes evidence from AO Arena, AO Crucible, AO Covenant, AO Foundry,
AO Forge, and AO2, then emits a deterministic promotion decision, activation
plan, active-stack manifest update, rollback plan, and public-safe operator
report.

The v0.1 product is a local-first Go CLI. Default execution is fixture and
dry-run only. AO Promoter does not push, tag, release, upload, deploy, mutate
sibling repositories, or write live control-plane state in v0.1 default paths.

## Run

```sh
go test ./...
go vet ./...
go run ./cmd/promoter --help
```

Product gate commands:

```sh
go build -o tmp/bin/promoter ./cmd/promoter
PATH="$PWD/tmp/bin:$PATH" promoter packet validate --packet examples/packets/valid/ao-promoter-v0.1.json
PATH="$PWD/tmp/bin:$PATH" promoter candidate validate --candidate examples/candidates/valid/ao-foundry-candidate.json
PATH="$PWD/tmp/bin:$PATH" promoter gates evaluate --packet examples/packets/valid/ao-promoter-v0.1.json --out tmp/promotion-gate.json
PATH="$PWD/tmp/bin:$PATH" promoter plan activate --packet examples/packets/valid/ao-promoter-v0.1.json --out tmp/activation-plan.json
PATH="$PWD/tmp/bin:$PATH" promoter active render --plan tmp/activation-plan.json --out tmp/active-stack.next.json
PATH="$PWD/tmp/bin:$PATH" promoter rollback plan --active examples/active/valid/current-active-stack.json --candidate examples/candidates/valid/ao-foundry-candidate.json --out tmp/rollback-plan.json
PATH="$PWD/tmp/bin:$PATH" promoter report render --gate tmp/promotion-gate.json --plan tmp/activation-plan.json --out tmp/promotion-report.md
PATH="$PWD/tmp/bin:$PATH" promoter apply --plan tmp/activation-plan.json --dry-run --out tmp/apply-dry-run.json
PATH="$PWD/tmp/bin:$PATH" promoter live-mutation boundary --authority examples/live-mutation/valid/covenant-authority.approved.json --foundry-request examples/live-mutation/valid/foundry-request.ready.json --forge-plan examples/live-mutation/valid/forge-plan.ready.json --ao2-packet examples/live-mutation/valid/ao2-packet.ready.json --sentinel-hold examples/live-mutation/valid/sentinel-hold.clear.json --rollback examples/live-mutation/valid/rollback-rehearsal.ready.json --command-status examples/live-mutation/valid/command-status.ready.json --out tmp/live-mutation-boundary.json
PATH="$PWD/tmp/bin:$PATH" promoter live-mutation docs-boundary --approval-ticket examples/live-docs-mutation/valid/approval-ticket.approved.json --foundry-gate examples/live-docs-mutation/valid/foundry-approval-gate.ready.json --forge-guard examples/live-docs-mutation/valid/forge-guard.ready.json --ao2-packet examples/live-docs-mutation/valid/ao2-docs-packet.ready.json --sentinel-verdict examples/live-docs-mutation/valid/sentinel-verdict.clear.json --rollback examples/live-docs-mutation/valid/rollback-execution.ready.json --command-readback examples/live-docs-mutation/valid/command-readback.ready.json --out tmp/live-docs-boundary.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path README.md --out tmp/readme-scan.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path docs --out tmp/docs-scan.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path examples --out tmp/examples-scan.json
git diff --check
```

## Governed Live-Mutation Boundary

`promoter live-mutation boundary` is a dry-run activation boundary for governed
live-mutation class promotion. It requires Covenant authority, Foundry request
evidence, Forge dry-run plan, AO2 dry-run packet, Sentinel hold verdict,
rollback rehearsal, and AO Command readback. The output includes
`current_mutation_class`, `next_mutation_class`, and
`class_promotion_readiness`. Promotion from one class to the next is ready only
after a completed live rehearsal for the current class, class-bound rollback
proof, clean `main` CI, and no active Sentinel or Promoter holds. It fails
closed when any upstream artifact is missing, not ready, on hold, not
digest-bound, claims scheduling, execution, approval, provider, release, or
repository mutation authority, or skips the current live rehearsal successor.
The live rehearsal successor path currently advances from
`docs_only_single_file` to `docs_only_multi_file` to `test_only`; config-only
remains a defined class but is not promoted live until a later slice adds
evidence for that boundary.
For the `test_only` to `low_risk_code` boundary, Promoter also emits explicit
`promotion_prerequisites`: successful `test_only` live evidence, a `test_only`
rollback fixture, low-risk Sentinel clear verdict, clean `main` CI, exact
`low_risk_code` Covenant class ticket, and read-only Command readback. A
wrong-class Covenant ticket fails the boundary even when the rest of the dry-run
evidence is ready.
For the `low_risk_code` to `multi_repo_low_risk` boundary, Promoter reports
`highest_proven_live_class`, `current_class_live_evidence_status`,
`next_denied_class`, `next_denied_reason`, and prerequisites for ordered merge
plan, per-repo rollback, per-repo CI, fresh repo state, and armed kill switch.
If `low_risk_code` has only dry-run/readback evidence, older fixtures keep the
highest proven live class at `test_only` and deny `multi_repo_low_risk` until
completed low-risk live rehearsal evidence is recorded. Later merged evidence
kept `fully_unsupervised_complex_mutation` proven for the governed 26-node first
non-planning rehearsal boundary. The current highest proven live class is
`bounded_rsi_self_improvement_application`, proven only for the exact private
readback/eval rubric rehearsal; the next denied class is `broad_RSI`.

Passing this boundary does not perform live mutation and does not grant ungated
authority. It reports whether the next class can be promoted by policy; it does
not widen promotion into broad RSI, hidden instruction mutation, unrestricted
self-modification, or policy/auth/secret/provider/deploy/release/config/
dependency expansion.

The final bounded RSI self-improvement application verdict accepts only the
exact private readback/eval rubric rehearsal. That means
`bounded_rsi_self_improvement_application` is proven only for that exact
private readback/eval rubric rehearsal. `broad_RSI` remains denied,
unrestricted self-modification remains denied, hidden instruction mutation
remains denied, and policy/auth/secret/provider/deploy/release/config/
dependency expansion remains denied. The Promoter verdict keeps the highest
proven live class at `bounded_rsi_self_improvement_application` and the next
denied class at `broad_RSI`; it does not claim broad RSI or policy-changing
autonomy.

`promoter live-mutation docs-boundary` is the narrower dry-run promotion
boundary for the first tiny docs-only live class. It requires an approved
Covenant docs-only approval ticket, Foundry approval gate, Forge execution
guard, AO2 docs-only patch packet, Sentinel clear verdict, Foundry rollback
execution rehearsal, and AO Command readback. It still does not mutate
repositories, execute work, approve work, call providers, release, upload, or
publish, and it does not claim broad or fully unsupervised live mutation.
The boundary output may support `safe_to_execute=true` only for the exact
approved docs-only PR rehearsal scope. That value means every upstream gate has
reported ready evidence; it is not a command to apply a patch, create a branch,
merge a PR, or widen authority beyond the approved docs-only class.

## SDD Files

| File | Purpose |
| --- | --- |
| `docs/sdd/AO-PROMOTER-PRD.md` | Product scope, users, non-goals, and readiness definition. |
| `docs/sdd/AO-PROMOTER-ARCHITECTURE.md` | Planned CLI, packages, data flow, storage layout, and integrations. |
| `docs/sdd/AO-PROMOTER-CONTRACTS.md` | JSON contract families, required fields, fixtures, and validation rules. |
| `docs/sdd/AO-PROMOTER-GATES.md` | Promotion gate matrix and blocker semantics. |
| `docs/sdd/AO-PROMOTER-ACTIVE-STACK.md` | Active-stack manifest, activation, and rollback semantics. |
| `docs/sdd/AO-PROMOTER-SAFETY.md` | Public-safety, dry-run, approval, and fail-closed rules. |
| `docs/sdd/AO-PROMOTER-IMPLEMENTATION-SLICES.md` | Implementation slices in dependency order. |
| `docs/sdd/AO-PROMOTER-ACCEPTANCE-GATES.md` | SDD and product 100/100 readiness gates. |
| `docs/sdd/AO-PROMOTER-SDD-HANDOFF.md` | Handoff prompt for AO Forge, AO Foundry, or Codex. |

## Local Planner Artifacts

AO2 SDD planner artifacts can be written under `target/` during local
automation runs. The directory is ignored because runspecs may include local
machine paths.

## Implementation Rule

Implement slice by slice. Keep v0.1 dry-run by default. A real active-stack
mutation requires a future live profile, explicit operator approval, valid
rollback plan, clean public-safety scan, and non-default command flag.

## License

AO Promoter is licensed under `Apache-2.0`. See `LICENSE`.
