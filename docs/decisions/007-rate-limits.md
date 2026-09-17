# Redis-clock token buckets

Status: accepted. Date: 2026-09-17.

## Decision

Use one atomic Lua script with Redis TIME, refill rate and capacity, plus expiring state.

## Rationale

A token bucket allows short bursts while limiting sustained rates; the server-side clock avoids different application clocks disagreeing.

## Consequences and alternatives

A hot shared endpoint is one Redis key; multi-region coordination and weighted project fairness are not implemented. Endpoint throttling reschedules without consuming an attempt.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
