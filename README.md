# AO Promoter

AO Promoter evaluates a candidate against policy, execution, benchmark,
adversarial, and monitoring results. It produces a deterministic decision,
activation plan preview, rollback plan, active-stack manifest preview, and
operator report, but it does not apply the change. Use it when collected AO
evidence must be evaluated as one decision package.

## How it fits in AO

- **Primary responsibility:** Candidate evaluation without activation execution.
- **Inputs:** AO Arena comparisons, AO Crucible assessments, AO Covenant decisions, AO Foundry and AO Forge readiness, AO2 evidence, AO Sentinel verdicts, and AO Command readbacks.
- **Outputs:** A decision, activation plan preview, rollback plan, active-stack manifest preview, and operator report.
- **Upstream:** AO Arena, AO Crucible, AO Covenant, AO Foundry, AO Forge, AO2, AO Sentinel, and AO Command.
- **Downstream:** AO Command and operators.

See the
[AO Architecture guide](https://github.com/uesugitorachiyo/ao-architecture)
and the
[AO Promoter component page](https://github.com/uesugitorachiyo/ao-architecture/blob/main/components/ao-promoter.md)
for the cross-repository flow.

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
PATH="$PWD/tmp/bin:$PATH" promoter mission rollup-summary --no-promotion examples/evidence/valid/ao-mission-gateway-no-promotion.json --out tmp/ao-mission-rollup-summary.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path README.md --out tmp/readme-scan.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path docs --out tmp/docs-scan.json
PATH="$PWD/tmp/bin:$PATH" promoter safety scan --path examples --out tmp/examples-scan.json
git diff --check
```

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

## Implementation Rule

Implement slice by slice. Keep v0.1 dry-run by default. A real active-stack
mutation requires a future live profile, explicit operator approval, valid
rollback plan, clean public-safety scan, and non-default command flag.

<!-- Legacy documentation-test compatibility tokens (not rendered):
AO Mission Gateway No-Promotion Readback
gateway readbacks are no-promotion evidence
Telegram and A2A intents cannot promote classes
timeline compaction is readback only
promotion_allowed=false
-->

## License

AO Promoter is licensed under `Apache-2.0`. See `LICENSE`.
