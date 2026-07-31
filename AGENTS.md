# AO Promoter Agent Instructions

## Status And Role

AO Promoter is the active deterministic promotion-verdict component. It evaluates a candidate's policy, execution, benchmark, adversarial, monitoring, readiness, and readback evidence, then produces a decision, activation preview, rollback plan, active-stack preview, and operator report.

Promoter v0.1 does not activate, mutate, deploy, publish, or release a candidate. A positive verdict is a bounded recommendation package for an operator, not execution authority.

## Sources Of Truth

- [docs/sdd/AO-PROMOTER-PRD.md](docs/sdd/AO-PROMOTER-PRD.md) and [docs/sdd/AO-PROMOTER-ARCHITECTURE.md](docs/sdd/AO-PROMOTER-ARCHITECTURE.md) define scope, integrations, and non-goals.
- [docs/sdd/AO-PROMOTER-CONTRACTS.md](docs/sdd/AO-PROMOTER-CONTRACTS.md) and [docs/sdd/AO-PROMOTER-GATES.md](docs/sdd/AO-PROMOTER-GATES.md) own evidence and blocker semantics.
- [docs/sdd/AO-PROMOTER-ACTIVE-STACK.md](docs/sdd/AO-PROMOTER-ACTIVE-STACK.md) and [docs/sdd/AO-PROMOTER-SAFETY.md](docs/sdd/AO-PROMOTER-SAFETY.md) define preview, rollback, dry-run, and fail-closed boundaries.
- `docs/contracts/`, `internal/cli/`, and their tests are authoritative for implemented behavior. [`.github/workflows/ci.yml`](.github/workflows/ci.yml) defines the broad gate.

## Ownership And Boundaries

- Bind each verdict to the exact candidate, source head, policy decision, AO2 evidence, Arena comparison, Crucible assessment, Foundry and Forge readiness, Sentinel verdict, Command readback, artifact digests, and freshness requirements.
- Block on missing, stale, digest-mismatched, invalid, held, regressed, or contradictory evidence. Do not infer a passing gate from absence.
- Preserve upstream ownership: Covenant decides policy, AO2 owns execution evidence, Arena and Crucible own evaluation outputs, Sentinel owns holds, and Command supplies read-only views.
- Treat `examples/` as contract fixtures and keep valid and invalid cases separate. Never edit scores, verdicts, holds, evidence, baselines, or rollback fields to make a candidate promotable.
- Keep generated gates, plans, active-stack previews, rollback plans, reports, scans, and binaries under ignored `tmp/` or `target/`. Default every apply or live-mutation surface to non-mutating dry run.
- Do not record secrets, credentials, private paths, account identifiers, or unredacted provider output. Activation, release, deployment, publication, live mutation, credentialed operation, permission changes, and direct-main changes require separate explicit operator authority and executable gates.

## Working Method

- Change the smallest verdict or preview surface while preserving deterministic evaluation, provenance, blocker priority, rollback completeness, public safety, and fail-closed parsing.
- Add negative tests for missing gates, stale evidence, digest drift, Sentinel holds, failed evaluation, unsafe content, incomplete rollback, and over-authority inputs.
- Update this file in the same pull request when durable commands, architecture, ownership, or authority boundaries change.

## Verification

- Promoter logic and contracts: `go test ./internal/cli -count=1`.
- Format relevant Go source with `gofmt -d` over `cmd/` and `internal/`; run `go test ./... -count=1`, `go vet ./...`, and `go build -o tmp/bin/promoter ./cmd/promoter`.
- Run the product-gate validation, verdict, plan, rollback, report, dry-run apply, live-boundary, and safety-scan commands listed in [README.md](README.md) when their surfaces change. Never substitute a live apply.
- For instruction changes run `python3 ../ao-architecture/scripts/verify_agent_instruction_layout.py --workspace-root .. --repository ao-promoter`. Always run `git diff --check`.

## Evidence And Completion

- Record source heads, commands and exits, upstream evidence digests and freshness, every blocked or passed gate, rollback evidence, and the final verdict digest. Report skipped, unavailable, or failed checks explicitly.
- Completion requires focused and broad gates, green pull-request CI, clean synchronized `main`, and task-branch cleanup. A promotion verdict never widens task authority.
