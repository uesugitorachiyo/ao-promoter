# AO Promoter Implementation Slices

These slices are written for a junior engineer implementing the future
`../ao-promoter` Go repository. Each slice is independently testable.

## Slice 01: Go CLI Foundation

Create:

- `go.mod`
- `cmd/promoter/main.go`
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `README.md`

Acceptance:

- `promoter --help` lists `candidate`, `packet`, `gates`, `plan`, `active`,
  `rollback`, `report`, `apply`, `evidence`, and `safety`;
- unknown commands fail with non-zero exit code;
- `go test ./...` passes.

## Slice 02: Contracts And Fixtures

Create:

- `docs/contracts/promoter-*.schema.json`
- all valid and invalid fixtures named in `AO-PROMOTER-CONTRACTS.md`

Acceptance:

- every JSON fixture parses;
- contract docs and fixture filenames match;
- invalid fixtures are covered by Go tests.

## Slice 03: Candidate And Packet Validation

Implement:

- `promoter candidate validate --candidate <path>`
- `promoter packet validate --packet <path>`

Acceptance:

- valid candidate passes;
- valid packet passes;
- unknown target slot fails;
- missing required gate fails;
- live apply default fixture fails.

## Slice 04: Evidence Digest And Freshness

Implement:

- evidence reference loading;
- SHA-256 verification;
- freshness validation;
- candidate ID cross-check.

Acceptance:

- digest mismatch fixture fails;
- stale evidence fixture fails;
- candidate mismatch fixture fails;
- valid packet evidence passes.

## Slice 05: Gate Evaluation

Implement:

- `promoter gates evaluate --packet <path> --out <json>`

Acceptance:

- canonical packet emits `status=passed`;
- failed Crucible gate blocks promotion;
- failed safety scan blocks promotion;
- blockers are structured and stable.

## Slice 06: Activation And Active Stack Rendering

Implement:

- `promoter plan activate --packet <path> --out <json>`
- `promoter active render --plan <path> --out <json>`

Acceptance:

- activation plan is dry-run-only;
- active manifest updates exactly one target slot;
- live mutation fields remain false;
- output outside `tmp/` fails.

## Slice 07: Rollback Planning

Implement:

- `promoter rollback plan --active <path> --candidate <path> --out <json>`

Acceptance:

- rollback plan contains previous component and verification commands;
- missing current active slot fails;
- promotion gate refuses packets without rollback readiness.

## Slice 08: Report And Dry-Run Apply

Implement:

- `promoter report render --gate <path> --plan <path> --out <markdown>`
- `promoter apply --plan <path> --dry-run --out <json>`

Acceptance:

- Markdown is derived from gate and plan JSON;
- dry-run apply reports `mutates_live_state=false`;
- apply without `--dry-run` fails.

## Slice 09: Safety Scan

Implement:

- `promoter safety scan --path <path> --out <json>`

Acceptance:

- public README/docs/examples pass;
- unsafe fixture fails without printing matched secret-like value;
- local absolute path fixture fails;
- forbidden action fixture fails.

## Slice 10: Public Demo And Clean-Clone Gate

Create:

- `docs/demo/AO-PROMOTER-GATED-PROMOTION.md`
- `examples/reports/valid/ao-promoter-v0.1.report.md`

Acceptance:

- clean clone runs the full dry-run promotion gate;
- public demo explains candidate to active stack path;
- no live credentials or sibling repositories are required.

## Final Verification

```sh
go test ./...
go vet ./...
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
