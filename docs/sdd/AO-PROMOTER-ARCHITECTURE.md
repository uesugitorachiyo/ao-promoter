# AO Promoter Architecture

## Product Shape

AO Promoter v0.1 is a local-first Go CLI named `promoter`. It reads candidate
profiles and promotion packets from JSON files, evaluates gate evidence, emits a
promotion decision, creates an activation plan, renders the next active-stack
manifest, creates a rollback plan, and renders a public-safe report.

Default mode is fixture and dry-run only. Every write command requires an output
path under `tmp/`. Durable examples under `docs`, `examples`, `cmd`, and
`internal` must remain public-safe.

## Planned Commands

| Command | Purpose |
| --- | --- |
| `promoter candidate validate --candidate <path>` | Validate a candidate profile. |
| `promoter packet validate --packet <path>` | Validate a promotion packet. |
| `promoter gates evaluate --packet <path> --out <json>` | Evaluate all required gates and write promotion decision. |
| `promoter plan activate --packet <path> --out <json>` | Write deterministic activation plan. |
| `promoter active render --plan <path> --out <json>` | Render next active-stack manifest from activation plan. |
| `promoter rollback plan --active <path> --candidate <path> --out <json>` | Write rollback plan before promotion. |
| `promoter report render --gate <path> --plan <path> --out <markdown>` | Render public-safe operator report. |
| `promoter apply --plan <path> --dry-run --out <json>` | Simulate activation without mutating live state. |
| `promoter evidence inspect --packet <path> --out <json>` | List evidence references, digests, freshness, and gate roles. |
| `promoter safety scan --path <path> --out <json>` | Scan durable artifacts for private data and forbidden actions. |

## Planned Packages

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Command parsing, usage text, deterministic exit codes. |
| `internal/contracts` | JSON loading, semantic validation, schema version checks. |
| `internal/gates` | Gate matrix evaluation and blocker reasons. |
| `internal/evidence` | Evidence references, SHA-256 checks, freshness rules. |
| `internal/active` | Active-stack manifest rendering and slot validation. |
| `internal/rollback` | Rollback plan generation and validation. |
| `internal/report` | Markdown reports derived from JSON decisions. |
| `internal/safety` | Secret, local-path, and forbidden-action scanning. |

## Data Flow

1. Operator validates a candidate profile.
2. Operator validates a promotion packet that references current evidence.
3. Gate evaluator loads each evidence reference and verifies schema, digest,
   freshness, candidate ID, and gate status.
4. Gate evaluator writes a promotion decision.
5. Activation planner writes a dry-run activation plan from the same packet.
6. Active renderer writes the next active-stack manifest from the plan.
7. Rollback planner writes the rollback path from current active stack to
   previous stack state.
8. Report renderer combines gate and plan JSON into a public-safe Markdown
   report.
9. Dry-run apply writes the actions that would be performed and records that no
   mutation occurred.

## Storage Layout

Durable files:

- `docs/contracts/promoter-*.schema.json`
- `examples/candidates/valid/ao-foundry-candidate.json`
- `examples/packets/valid/ao-promoter-v0.1.json`
- `examples/active/valid/current-active-stack.json`
- `examples/evidence/valid/*.json`
- `examples/evidence/invalid/*.json`
- `examples/reports/valid/ao-promoter-v0.1.report.md`

Generated scratch files:

- `tmp/promotion-gate.json`
- `tmp/activation-plan.json`
- `tmp/active-stack.next.json`
- `tmp/rollback-plan.json`
- `tmp/promotion-report.md`
- `tmp/apply-dry-run.json`
- `tmp/*-scan.json`

## Error Handling

All validation and promotion commands fail closed. Error messages identify the
contract family, field, gate, and reason without printing secret-like values.
Unknown schema versions, missing evidence, stale evidence, digest mismatches,
candidate mismatches, failed safety scans, and missing rollback plans all return
non-zero exit codes.
