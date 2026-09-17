# Durable idempotency with content conflict detection

Status: accepted. Date: 2026-09-17.

## Decision

Use UNIQUE(project_id,event_id) and canonical envelope SHA-256 comparison. Return 200 for identical duplicates and 409 for changed content.

## Rationale

A cache-only check permits duplicate races and loses protection after Redis eviction. Database arbitration survives restarts.

## Consequences and alternatives

Producer IDs must be stable and distinct per logical event. Numeric lexical variants can conflict; idempotency is scoped to ingestion, not receiver side effects.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
