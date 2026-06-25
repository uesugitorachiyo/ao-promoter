# AO Promoter Gated Promotion Demo

AO Promoter is the final promotion boundary for the AO orchestration framework.
It turns a candidate component into a proposed active-stack update only after
all required evidence gates pass.

## Demo Flow

1. Validate the candidate profile.
2. Validate the promotion packet and evidence references.
3. Evaluate the required gate matrix.
4. Create a dry-run activation plan.
5. Render the next active-stack manifest without mutating live state.
6. Create a rollback plan for the previous active component.
7. Render a public-safe promotion report.
8. Simulate apply in dry-run mode only.

## Commands

```sh
go test ./...
go build -o tmp/bin/promoter ./cmd/promoter
PATH="$PWD/tmp/bin:$PATH" promoter packet validate --packet examples/packets/valid/ao-promoter-v0.1.json
PATH="$PWD/tmp/bin:$PATH" promoter gates evaluate --packet examples/packets/valid/ao-promoter-v0.1.json --out tmp/promotion-gate.json
PATH="$PWD/tmp/bin:$PATH" promoter plan activate --packet examples/packets/valid/ao-promoter-v0.1.json --out tmp/activation-plan.json
PATH="$PWD/tmp/bin:$PATH" promoter active render --plan tmp/activation-plan.json --out tmp/active-stack.next.json
PATH="$PWD/tmp/bin:$PATH" promoter rollback plan --active examples/active/valid/current-active-stack.json --candidate examples/candidates/valid/ao-foundry-candidate.json --out tmp/rollback-plan.json
PATH="$PWD/tmp/bin:$PATH" promoter report render --gate tmp/promotion-gate.json --plan tmp/activation-plan.json --out tmp/promotion-report.md
PATH="$PWD/tmp/bin:$PATH" promoter apply --plan tmp/activation-plan.json --dry-run --out tmp/apply-dry-run.json
```

## Expected Result

The canonical AO Foundry candidate passes the gate matrix and produces:

- `tmp/promotion-gate.json` with `status=passed`;
- `tmp/activation-plan.json` with `dry_run_only=true`;
- `tmp/active-stack.next.json` with the `factory` slot updated;
- `tmp/rollback-plan.json` with the previous `factory` component preserved;
- `tmp/promotion-report.md` suitable for public review;
- `tmp/apply-dry-run.json` with `mutates_live_state=false`.

AO Promoter v0.1 does not contact live providers, mutate sibling repositories,
write control-plane state, publish releases, or apply active-stack changes.
