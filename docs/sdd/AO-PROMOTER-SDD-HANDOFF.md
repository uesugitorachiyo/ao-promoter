# AO Promoter SDD Handoff

Use this prompt after reviewing the SDD pack.

```text
You are implementing AO Promoter v0.1 from the approved SDD documents.

Repository to create:
./ao-promoter

Goal:
Build AO Promoter as the gated promotion path from candidate to active AO stack.
The v0.1 product validates candidates and promotion packets, verifies evidence
from AO Arena, AO Crucible, AO Covenant, AO Foundry, AO Forge, and AO2, emits a
promotion gate, creates a dry-run activation plan, renders the next active-stack
manifest, creates a rollback plan, renders a public-safe promotion report, and
supports dry-run apply only.

Required constraints:
- Use Go for the CLI.
- Support Ubuntu, macOS, and Windows.
- Keep v0.1 dry-run-only by default.
- Do not run live providers.
- Do not push, tag, release, upload, deploy, mutate sibling repositories, or
  write live control-plane state.
- Do not store secrets, local absolute paths, private prompts, or unredacted
  evidence in durable artifacts.
- Implement slice by slice from AO-PROMOTER-IMPLEMENTATION-SLICES.md.
- Add failing tests before implementation code.
- Stop when AO-PROMOTER-ACCEPTANCE-GATES.md product 100/100 gate passes.

Final response must include:
- slices completed;
- files changed;
- verification commands and results;
- current production-readiness score;
- promotion gate result;
- dry-run apply result;
- remaining blocking next actions, if any.
```

## Implementation Readiness Verdict

The plan is ready to implement when:

- `target/ao-promoter-plan.json` validates with AO2 SDD validation;
- SDD docs contain concrete requirements rather than placeholders;
- acceptance gates define exact commands;
- handoff prompt needs no additional context.
