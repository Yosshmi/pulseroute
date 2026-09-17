# PostgreSQL is both the record and queue

Status: accepted. Date: 2026-09-17.

## Decision

Insert events and delivery rows in one transaction; claim with FOR UPDATE SKIP LOCKED and expiring tokens.

## Rationale

A separate broker would create a dual-write window or require an outbox relay. One database makes the acceptance guarantee directly testable.

## Consequences and alternatives

Polling and queue updates compete with inspection queries. At higher measured volumes, an outbox plus external broker may be justified; Kafka is not implemented.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
