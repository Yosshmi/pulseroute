# Implementation plan

The attached Pasted text.txt is the implementation brief. Work proceeds in independently
reviewable commits with actual timestamps.

1. Establish configuration, schema, security helpers, and database lifecycle.
2. Implement authentication, owner-scoped projects, endpoints, API keys, ingestion,
   inspection, metrics, and replay.
3. Implement PostgreSQL lease claims, fenced completion, bounded concurrency,
   signed delivery, retry policy, and a configurable receiver.
4. Build the React dashboard and meaningful unit, integration, and E2E coverage.
5. Package Compose, scripts, CI, Kubernetes, and a free demo deployment option.
6. Measure what the environment permits, review failure modes, then teach the final
   implementation through the handbook, walkthrough, and evidence-backed resume notes.

## Decisions made before implementation

- Standard library HTTP routing; pgx for PostgreSQL, go-redis for Redis.
- PostgreSQL stores the durable queue in the delivery table. Ingestion and fan-out
  commit together; no database/queue dual-write window.
- Delivery uses leases and fencing tokens, with at-least-once external effects.
- Redis rate limiting fails closed; dashboard cache failures fall back to SQL.
- Single-owner projects provide the authorization boundary; team RBAC is outside scope.
- API and worker are independent processes; a combined demo command can share one
  free web instance when hosting does not provide free background workers.
