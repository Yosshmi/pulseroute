# Finite retries and explicit replay generations

Status: accepted. Date: 2026-09-17.

## Decision

Retry network failures, 408, 429 and 5xx with capped exponential equal jitter. Preserve historical attempts when replay starts a new generation.

## Rationale

Jitter spreads synchronized failures; finite budgets prevent endless hammering. Ordinary 4xx usually requires a configuration or payload fix.

## Consequences and alternatives

Retry-After is not parsed. Changing endpoint policy affects later claims. At-least-once effects remain possible even with fenced database updates.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
