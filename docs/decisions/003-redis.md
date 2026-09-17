# Redis for ephemeral coordination

Status: accepted. Date: 2026-09-17.

## Decision

Use Redis for Lua token buckets and a five-second dashboard cache, never as the authoritative event store.

## Rationale

Atomic operations coordinate rates across workers/API replicas; an expiring cache avoids repeated aggregate queries.

## Consequences and alternatives

Rate limits fail closed on outage, reducing availability to protect destinations. Cache failures fall back to PostgreSQL. A reset allows a fresh burst.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
