# Bounded capacity before lease claim

Status: accepted. Date: 2026-09-17.

## Decision

Start N worker goroutines, a buffered channel and an N-slot capacity semaphore; reserve capacity before claiming one row.

## Rationale

A large in-memory prefetched backlog could expire leases before requests start. Bounded claims also cap memory and destination concurrency.

## Consequences and alternatives

One dispatcher performs sequential claims per process; multiple processes may scale this. Worker count and database connection pools must be tuned together.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
