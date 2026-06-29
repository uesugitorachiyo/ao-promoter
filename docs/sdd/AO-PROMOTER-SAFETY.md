# AO Promoter Safety

## Default Posture

AO Promoter v0.1 is dry-run and fail-closed. It can create local scratch
artifacts under `tmp/`, but it cannot mutate live state or sibling repositories.

## Forbidden Actions

Default paths must not:

- push to any remote;
- create or delete tags;
- publish releases or packages;
- upload artifacts outside local scratch output;
- deploy services;
- mutate sibling AO repositories;
- write live control-plane state;
- store credentials;
- print secret-like values;
- write local absolute paths to durable public artifacts.

## Approval Rules

Free-form approval text is not authority. A future live apply requires a
machine-readable approval artifact with:

- approval schema version;
- operator identity;
- candidate ID;
- target stack ID;
- exact action scope;
- expiration;
- signature or trusted local authority marker.

v0.1 fixtures keep live apply disabled.

The first docs-only live mutation boundary is narrower than general live apply.
It requires exact-scope Covenant approval, Foundry approval-gate evidence,
Forge guard evidence, an AO2 docs-only patch packet, Sentinel clear verdict,
rollback execution rehearsal, and AO Command readback. A passing boundary is
eligibility evidence for that exact docs-only PR rehearsal scope only; it is not
authority to execute broad live mutation or approve future scopes.

## Public Safety Scan

The scanner blocks:

- bearer-token-like strings;
- private key markers;
- GitHub token-like strings;
- cloud access-key-like strings;
- password assignment patterns;
- local absolute paths;
- forbidden action command text.

Findings report detector, file, line, and summary without printing the matched
secret-like value.

## Fail-Closed Rules

Promotion fails when:

- a required evidence file is missing;
- a digest mismatches;
- evidence is stale;
- candidate IDs disagree;
- public-safety scan fails;
- rollback planning fails;
- dry-run guard is disabled;
- live mutation is requested in v0.1;
- first-docs-only live mutation evidence is missing exact approval, rollback,
  public-safety, verification, Sentinel, or Command readback proof;
- schema version is unknown.
