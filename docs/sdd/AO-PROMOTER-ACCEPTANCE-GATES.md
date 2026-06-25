# AO Promoter Acceptance Gates

## SDD Readiness Gate

The SDD pack is 100/100 implementation-ready only when:

- PRD defines users, goals, non-goals, success metrics, and production
  readiness;
- architecture defines commands, packages, data flow, storage, and errors;
- contracts define schema families, required fields, valid fixtures, invalid
  fixtures, and validation rules;
- gate matrix defines required evidence and blocker semantics;
- active-stack document defines activation, manifest, rollback, and dry-run
  apply behavior;
- safety document defines forbidden actions, approvals, scans, and fail-closed
  rules;
- implementation slices define exact future files, commands, tests, and final
  verification;
- handoff prompt needs no additional context;
- `target/ao-promoter-plan.json` validates with AO2 SDD validation;
- placeholder scan finds no incomplete planning markers.

## Product Readiness Gate

The implemented AO Promoter v0.1 scores 100/100 only when these pass:

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

## Competitive Gate

AO Promoter is competitive only when it provides:

- deterministic dry-run promotion decisions;
- strict evidence digest and freshness checks;
- explicit gate matrix with blocker reasons;
- active-stack manifest rendering;
- rollback plan requirement before promotion;
- public-safe promotion report;
- clean-clone reproducibility;
- no live mutation in default paths.

## Exit Condition

An autonomous implementation run stops when:

- every implementation slice is complete;
- product readiness gate passes from a clean clone;
- promotion gate emits `passed`;
- dry-run apply reports no mutation;
- public-safety scans pass with zero findings;
- final response lists verification commands and remaining non-blocking future
  work.
