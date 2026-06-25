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
