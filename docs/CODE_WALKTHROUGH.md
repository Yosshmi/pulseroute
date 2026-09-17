# Code walkthrough and study order

Read the handbook's plain-English introduction first, then run seed and inspect
one successful/retried/dead delivery. This order follows executable control flow.
Links open the actual source. For each row, explain the invariant without looking
before moving on. Not every statement needs memorization.

| Order / file | What to understand and functions to trace | Concepts / interview prompt |
|---|---|---|
| 1. [go.mod](../go.mod) | Module `pulseroute`; direct pgx, Redis and bcrypt dependencies. `go.sum` authenticates module contents. | Why these dependencies and no large framework? |
| 2. [cmd/api/main.go](../cmd/api/main.go), [cmd/worker/main.go](../cmd/worker/main.go), [cmd/demo/main.go](../cmd/demo/main.go) | Each selects `app.Run` mode. API/worker are separate executables; demo shares lifecycle. | Process boundaries versus code/package boundaries. |
| 3. [internal/config/config.go](../internal/config/config.go) | `Load`, environment defaults, encryption key validation, worker count and lease/timeout relation. | Fail early; distinguish secure defaults from local overrides. |
| 4. [internal/app/app.go](../internal/app/app.go) | `Run`: signal context, pool lifecycle, server timeouts, error channel, worker join and shutdown. | Who owns cancellation and resource closure? |
| 5. [internal/database/database.go](../internal/database/database.go) | `Open`, ParseConfig, MaxConns, connection lifetime, Ping and wrapped errors. | Why can more worker replicas exhaust database connections? |
| 6. [migrations/migrate.go](../migrations/migrate.go), [cmd/migrate/main.go](../cmd/migrate/main.go) | `Apply`, embedded files, migration table and advisory transaction lock. | What if two deploys run migrations simultaneously? |
| 7. [migrations/001_initial.sql](../migrations/001_initial.sql) | Draw tables, unique constraints, FKs, state CHECK and partial indexes. | Which invariant is protected by which constraint? |
| 8. [internal/api/server.go](../internal/api/server.go) | `Handler`, `middleware`, `decode`, `fail`, `rows`, `one`, `page`, `ready`. | Body limits, error shape, cancellation, pagination and logging. |
| 9. [internal/api/auth.go](../internal/api/auth.go) | `register`, `login`, `session`, `auth`, `logout`, `authLimit`. | Random sessions versus JWT; revocation; Secure cookies; proxy-IP trade-off. |
| 10. [internal/security/security.go](../internal/security/security.go) | `Password`, `CheckPassword`, `Token`, `Hash`, `Encrypt`, `Decrypt`, `Sign`, `Verify`. | Why hash one kind of secret and encrypt another? |
| 11. [internal/api/projects.go](../internal/api/projects.go) | Owner-scoped lists, `createKey`, one-time raw key response, `deleteKey` revocation. | Why is a resource ID insufficient authorization? |
| 12. [internal/api/endpoints.go](../internal/api/endpoints.go) | Partial updates, `*bool`, policy validation, generated/encrypted secret, soft deletion. | What happens to old delivery history when an endpoint is deleted? |
| 13. [internal/events/events.go](../internal/events/events.go) | `Validate`, `Canonical`, `UseNumber`, SHA-256 of normalized envelope. | Why can float64 conversion break idempotency? |
| 14. [internal/api/ingest.go](../internal/api/ingest.go) | `ingest`: API-key lookup, bucket, transaction, insert conflict, content check, fan-out, commit. | Name every crash window and the producer's safe retry behavior. |
| 15. [internal/redisx/redis.go](../internal/redisx/redis.go) | `Open`, `Allow`, Lua TIME/HMGET/HSET/EXPIRE, `Cached`, `Cache`. | Atomicity, TTL, cache invalidation and failure policy. |
| 16. [internal/delivery/queue.go](../internal/delivery/queue.go) | `Job`, `Claim`, `Defer`, `Finish`, `ErrStale`. Trace locks, lease token and completion transaction. | Difference between row lock and durable lease; what fencing cannot prevent. |
| 17. [internal/delivery/worker.go](../internal/delivery/worker.go) | `Run`, `process`, `deferJob`. Draw slots/channel/goroutines; follow all exits. | Why no unbounded goroutines? Who closes the channel? |
| 18. [internal/security/transport.go](../internal/security/transport.go) | `ValidateURL`, `PublicIP`, `HTTPClient`, custom DialContext and redirect policy. | SSRF, DNS rebinding, timeout and connection reuse. |
| 19. [internal/delivery/retry.go](../internal/delivery/retry.go) | `Retryable`, `Backoff`, exponential cap and equal jitter. | Why does 400 differ from 429? Why jitter? |
| 20. [internal/api/inspection.go](../internal/api/inspection.go) | `events`, `deliveryList`, `delivery`, `replay`, `dashboard`, filters and date validation. | Replay generations, history, SQL ownership and cached aggregates. |
| 21. [internal/receiver/receiver.go](../internal/receiver/receiver.go), [cmd/receiver/main.go](../cmd/receiver/main.go) | `ServeHTTP`: verify raw body, synchronized counter, configurable failure/sleep, shutdown. | Synthetic dependency versus mocking; response loss and duplicate effects. |
| 22. [frontend/src/api.ts](../frontend/src/api.ts) | Types, `APIError`, `api`, `post`, cookie credentials and non-JSON errors. | How does the browser distinguish expired session from a server failure? |
| 23. [frontend/src/App.tsx](../frontend/src/App.tsx) | `useLoad`, `App`, `Dashboard`, `EndpointManager`, `Records`, `DeliveryDetails`. Follow state and refetch paths. | Loading/error/empty states, request cancellation and trustworthy metrics. |
| 24. [frontend/src/styles.css](../frontend/src/styles.css), [frontend/src/main.tsx](../frontend/src/main.tsx) | Responsive layout, scroll containers, hash router and React root. | Why hash routing works with a static file server. |
| 25. [security tests](../internal/security/security_test.go), [retry tests](../internal/delivery/retry_test.go), [event tests](../internal/events/events_test.go), [config tests](../internal/config/config_test.go), [API tests](../internal/api/server_test.go) | Identify meaningful invariant, negative case and expected result in each test. | Which tests need infrastructure and which should not? |
| 26. [tests/integration_test.go](../tests/integration_test.go) | `TestIntegrationWorkflow`: real services, concurrent duplicate requests, recovery, replay, ownership, Redis and fencing. | What would this catch that a repository mock would miss? |
| 27. [frontend unit tests](../frontend/src/api.test.ts), [browser workflow](../frontend/e2e/workflow.spec.ts) | Fetch contract versus complete browser/server path; inspect trace/screenshots on failure. | Why does Linux font layout matter for mobile verification? |
| 28. [cmd/seed/main.go](../cmd/seed/main.go), [internal/democlient/client.go](../internal/democlient/client.go) | `Bootstrap`, synthetic account/project/endpoints, generic event types. | How do you reproduce a useful demo without real customer data? |
| 29. [cmd/loadtest/main.go](../cmd/loadtest/main.go), [tests/performance_test.go](../tests/performance_test.go) | Bounded concurrent producers, atomic error count, latency samples and worker comparison. | Burst throughput versus sustainable rate, p95 versus average. |
| 30. [scripts/explain.sql](../scripts/explain.sql) | Transactional fixture, ANALYZE, indexed/unindexed plan, rollback. | How do you compare an access path without inventing a speedup? |
| 31. [scripts](../scripts/) | Setup secrets, migration, seed, dev, test, load. All Bash scripts fail on errors. | Why does setup refuse to replace an existing .env? |
| 32. [Dockerfile](../deployments/docker/Dockerfile), [frontend Dockerfile](../deployments/docker/frontend.Dockerfile), [compose.yaml](../compose.yaml) | Multi-stage/non-root images, service DNS, migration dependency, volume and health checks. | How can a healthy process still be unready? |
| 33. [CI](../.github/workflows/ci.yml) | Two jobs: source/service checks and packaged container/browser checks. | What is verified by each job? Where are failure artifacts? |
| 34. [Kubernetes](../deployments/kubernetes/), [render.yaml](../render.yaml) | Resource boundaries, Secret reference, migration order and cost-driven combined demo. | Provided manifests versus verified live deployment. |

## Active learning exercises

1. Draw the ingest transaction and identify where the unique key arbitrates a race.
2. Explain why a new endpoint does not receive an old event on duplicate ingestion.
3. Follow a 503 through retry to dead-letter, then replay without deleting history.
4. Explain why the stale token test rejects old completion but cannot undo an HTTP send.
5. Change only a local endpoint retry policy, predict the timeline, then observe it.
6. Run the query-plan experiment and explain rows removed by filter and buffer hits.
7. Read one CI failure/fix in Git history and reproduce the reasoning behind the fix.
8. Before using a resume bullet, demonstrate its behavior without reading this guide.
