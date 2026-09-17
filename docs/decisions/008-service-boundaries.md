# Separate runtime services, shared implementation

Status: accepted. Date: 2026-09-17.

## Decision

Run API and worker separately in Compose/Kubernetes. A demo command starts both with one lifecycle and serves static React assets.

## Rationale

API latency and worker throughput can be scaled independently; a single free web instance cannot host a separate free background service.

## Consequences and alternatives

Shared schema creates coordinated migration requirements. The combined command is a hosting compromise; idle hosting suspends delivery until the instance wakes.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
