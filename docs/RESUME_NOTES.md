# Resume notes and claim defence

Repository: https://github.com/Yosshmi/pulseroute

Describe this as a portfolio project, not employment or production infrastructure.
The work was AI-assisted; claim understanding and ownership only to the extent you
can explain and demonstrate it. No invented adoption, uptime or scale metrics.

## Three concise bullets

1. Built a Go webhook delivery platform with transactional PostgreSQL event fan-out,
   idempotent ingestion, bounded background workers, retries and dead-letter replay.
2. Implemented Redis-backed rate limits, HMAC-SHA256 webhook verification, encrypted
   endpoint secrets and project-scoped authorization with a React/TypeScript dashboard.
3. Added real PostgreSQL/Redis integration tests, browser E2E coverage, Docker Compose
   and GitHub Actions checks; documented measured worker-pool and SQL index experiments.

These bullets intentionally do not say production, millions of requests, exactly
once, or a fabricated percentage improvement. Kubernetes manifests can be mentioned
as "provided Kubernetes deployment manifests," not "operated production Kubernetes."

## Claim 1: reliable asynchronous delivery

**Evidence:** Ingestion writes event and jobs in one transaction; a unique business
key resolves duplicate races; worker capacity is bounded; leases recover abandoned
jobs; completion verifies a fencing token; retry budgets lead to dead letters and
replay preserves history through generations.

**Files:** `internal/api/ingest.go`, `migrations/001_initial.sql`,
`internal/delivery/{worker,queue,retry}.go`, `internal/api/inspection.go`,
`tests/integration_test.go`.

**Demonstrate:** Send the same event concurrently, show one durable event/delivery;
use receiver `?failures=2`, inspect three attempts and eventual success; use
`?status=400`, fix the endpoint, replay and show both generations.

**Likely follow-ups:** Why PostgreSQL instead of Kafka? What if the process crashes
after HTTP success? What does fencing prevent? Who closes the channel? Why is
idempotent ingestion different from exactly-once receiver effects?

## Claim 2: security and operational visibility

**Evidence:** Redis Lua token buckets use a server clock; HMAC includes timestamp
and exact body; comparison is constant time; endpoint secrets are AES-GCM encrypted;
API/session tokens are hashed; management queries enforce project ownership.
The dashboard displays persisted attempts and scoped aggregate metrics.

**Files:** `internal/redisx/redis.go`, `internal/security/{security,transport}.go`,
`internal/api/{auth,projects,endpoints,inspection}.go`, `frontend/src/App.tsx`.

**Demonstrate:** Tamper with a signature, request another account's delivery,
exceed a token bucket, revoke an API key, inspect a failed attempt. Explain that
dashboard totals can lag five seconds and local private-network mode is not public-safe.

**Likely follow-ups:** Why encrypt rather than hash signing secrets? How is DNS
rebinding addressed? What happens when Redis disappears? What prevents CSRF?
What is missing for production authentication and credential rotation?

## Claim 3: tested and reproducible engineering

**Evidence:** Actual-service integration checks cover concurrent idempotency,
recovery, exhaustion, timeout, replay, authorization and stale leases. Playwright
drives a complete browser workflow. CI builds/tests the packaged Compose system.
Performance and query-plan outputs describe exactly the measured environment.

**Files:** `tests/integration_test.go`, `tests/performance_test.go`,
`frontend/e2e/workflow.spec.ts`, `.github/workflows/ci.yml`, `compose.yaml`,
`docs/performance.md`, `docs/query-plans.txt`, `docs/verification.md`.

**Demonstrate:** Open the verified GitHub workflow run, run local setup/seed, show
failure artifacts from an earlier run and the subsequent fix, then explain why
60-event loopback measurements do not establish sustained production throughput.

**Likely follow-ups:** What was mocked? What does race detection miss? Which Docker
failure could unit tests miss? What can be reproduced from a clean checkout?
How did you choose free hosting without accidentally enabling paid resources?

## Claims to avoid or qualify

- "Exactly-once webhooks": false; the receiver can see duplicates.
- "Scales to millions": unmeasured; explain design options instead.
- "Reduced latency by X%": do not derive a resume improvement claim from one tiny run.
- "Production-deployed": no public deployment until independently verified.
- "Kubernetes experience": specify authored manifests versus operated a cluster.
- "Microservices": qualify independent API/worker processes sharing one schema.
- "All tests pass": cite the exact successful run and tested commit from verification.

## Your next ownership step

Study the handbook, reproduce the demo, and implement a small change yourself—such
as Retry-After handling with a capped delay and regression tests. Explain your
decision and commit it normally. That gives you a concrete personal extension to
discuss alongside the AI-assisted foundation.
