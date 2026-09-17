# Go and standard library HTTP

Status: accepted. Date: 2026-09-17.

## Decision

Use Go with net/http method/path routing, contexts, and slog. pgx and go-redis are the infrastructure clients.

## Rationale

Concurrency, cancellation and profiling are explicit and interviewable without a framework lifecycle. Standard routing is enough for this API.

## Consequences and alternatives

Handlers still need careful validation and middleware. A framework would reduce some wiring but would not solve idempotency or authorization.

See [architecture](../architecture.md), the linked code in the
[walkthrough](../CODE_WALKTHROUGH.md), and [measured performance](../performance.md).
