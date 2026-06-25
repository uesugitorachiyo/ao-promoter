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
- schema version is unknown.
