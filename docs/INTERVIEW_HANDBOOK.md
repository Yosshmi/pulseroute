# PulseRoute interview handbook

Read this with the repository open. The goal is to explain decisions and failure
windows, not memorize function names. First run the demo; then trace one event;
then deliberately cause a failure and explain the stored evidence. All examples
use synthetic data. Be open about AI assistance and distinguish code you have
studied from changes you have personally made.

## 1. The project in simple English

A webhook is an HTTP message one application sends when something happens. An
order service might send `order.created` to a warehouse and an analytics service.
If it sends both synchronously, a slow warehouse makes the order request slow. If
it sends once and forgets, an outage loses the notification. If it retries without
deduplication, the warehouse may ship twice.

PulseRoute accepts the event quickly after durable storage. It creates one delivery
job per active destination. Background workers send those jobs, remember completed
attempts and schedule retries. An operator can see why a delivery failed and replay
it after fixing the problem. The dashboard is a window into stored state, not a
separate source of truth.

The API is the reception desk: it checks identity, validates input, manages projects
and records work. PostgreSQL is the ledger and waiting tray. Redis is the short-term
rate counter and dashboard cache. Workers are the couriers. A webhook receiver is
the destination, responsible for its own business transaction and deduplication.
React renders the ledger, including failed attempts instead of hiding them.

Try this: seed 30 events. Each goes to three destinations: immediate success,
two failures then success, and permanent 400. Open a delivery timeline. The three
outcomes belong to the same source event but evolve independently.

## 2. Trace one complete request

Envelope: `{"event_id":"order-1001","type":"order.created","data":{"value":1499}}`.
The `data` object is generic; affiliate conversions are one possible scenario.
Paths below are relative to repository root. Read the complete functions around
these abbreviated excerpts to see error paths and context propagation.

| Step | File and function | Important expression | Why it exists |
|---|---|---|---|
| Start processes | `cmd/api/main.go`, `app.Run` | `app.Run("api")` | Thin executable selects the runtime mode. |
| Load constraints | `internal/config/config.go`, `Load` | `LeaseDuration < 3*HTTPTimeout` | Rejects unsafe time budgets at startup. |
| Connect | `internal/database/database.go`, `Open` | `pgxpool.NewWithConfig(ctx,cfg)` | Shares a bounded connection pool rather than connecting per request. |
| HTTP routing | `internal/api/server.go`, `Handler` | `m.HandleFunc("POST /v1/events",s.ingest)` | Dispatches this method/path to ingestion. |
| Middleware | same, `middleware` | `context.WithTimeout(...,15*time.Second)` | Assigns request ID, deadline, security headers, origin checks and log fields. |
| Key authentication | `internal/api/ingest.go`, `ingest` | `security.Hash(key)` | Queries a hash of the bearer key, requiring `revoked_at IS NULL`. |
| Rate check | `internal/redisx/redis.go`, `Allow` | `bucket.Run(ctx,r,...).Int()` | Atomic Redis-clock bucket stops concurrent replicas overspending tokens. |
| Parse | `internal/api/server.go`, `decode` | `http.MaxBytesReader(...,1<<20)` | Bounds memory and rejects unknown fields/trailing JSON. |
| Validate | `internal/events/events.go`, `Input.Validate` | `identifier.MatchString(v.EventID)` | IDs/types are bounded and valid; data must be JSON. |
| Canonicalize | same, `Input.Canonical` | `decoder.UseNumber()` | Stable object ordering/whitespace; preserves large integer precision. |
| Begin transaction | `internal/api/ingest.go`, `ingest` | `s.DB.Begin(r.Context())` | Event and all delivery rows commit or roll back together. |
| Idempotency arbitration | same | `ON CONFLICT(project_id,event_id) DO NOTHING` | The database unique key resolves concurrent requests safely. |
| Duplicate branch | same | `if oldHash != hash` | 200 for identical retry, 409 for accidental ID reuse with changed content. |
| Fan-out | same | `INSERT INTO deliveries ... SELECT ... FROM endpoints` | Creates one job per active destination inside the same transaction. |
| Acknowledge | same | `tx.Commit(r.Context())` then `write(...,202,...)` | Acknowledges only persisted work; no separate broker write can be lost. |
| Reserve worker capacity | `internal/delivery/worker.go`, `Run` | `case slots <- struct{}{}` | Bounds all claimed/in-flight jobs to the configured worker count. |
| Claim job | `internal/delivery/queue.go`, `Claim` | `FOR UPDATE SKIP LOCKED` | Multiple processes claim distinct available rows without waiting on each other. |
| Pass to goroutine | `internal/delivery/worker.go`, `Run` | `jobs <- j`; `for j := range jobs` | Dispatcher hands work through a bounded channel. |
| Recheck endpoint and rate | same, `process` | `if !j.Active`; `redisx.Allow(...)` | Cancel inactive destinations or defer throttled work without an HTTP attempt. |
| Decrypt secret | `internal/security/security.go`, `Decrypt` | `g.Open(...)` | Obtains signing material only when needed; authenticates ciphertext. |
| Timeout | `internal/delivery/worker.go`, `process` | `context.WithTimeout(ctx,HTTPTimeout)` | A slow destination cannot hold a worker indefinitely. |
| Sign | `internal/security/security.go`, `Sign` | `timestamp+"."` then exact body | Binds timestamp and payload under HMAC-SHA256. |
| Send | `internal/security/transport.go`, `HTTPClient`; worker `process` | `w.Client.Do(req)` | Resolves/checks addresses at dial time, disables redirects, enforces deadlines. |
| Receive | `internal/receiver/receiver.go`, `ServeHTTP` | `security.Verify(...)` | Demo destination verifies authenticity before applying response behavior. |
| Record | `internal/delivery/queue.go`, `Finish` | `WHERE id=$1 AND lease_token=$2 ... FOR UPDATE` | Rejects a stale worker before storing attempt/status together. |
| Retry decision | `internal/delivery/retry.go`, `Retryable`, `Backoff` | `Retryable(statusCode) && Attempt < MaxAttempts` | Separates transient failures from permanent or exhausted work. |
| Dead-letter | `Queue.Finish` | `INSERT INTO dead_letters` | Retains operator-visible reason in the same completion transaction. |
| Dashboard | `internal/api/inspection.go`, `dashboard`, `delivery` | aggregates and ordered `json_agg` | Returns actual counters and attempt history, owner-scoped. |
| Render | `frontend/src/App.tsx`, `Dashboard`, `DeliveryDetails` | `useLoad(...)`, `data.reload()` | Fetches current state; refresh is explicit and metrics can lag five seconds. |

The key subtlety: ingestion idempotency stops duplicate **jobs**, but a worker may
send the same job twice after a partial failure. Explain both rather than saying
"we guarantee exactly once." The external effect and the local database commit
are not one transaction.

## 3. Go crash course using this repository

Each entry gives a generic example, the real use, a reason, and a practice question.

### Packages and modules

`package orders` groups related code; `import "myapp/orders"` uses it. `go.mod`
defines the module path and dependency versions. PulseRoute's module is `pulseroute`;
`internal/delivery` groups queue/worker/retry logic. The `internal` directory prevents
unrelated modules importing implementation packages. Ask: why is a package boundary
useful even when there is only one repository? It organizes responsibility without
requiring a service or interface for every file.

### Structs and methods

`type Job struct { ID string }` creates a record; `func (j Job) Valid() bool` attaches
behavior. `Queue` holds a pool and lease duration; its methods implement durable
operations. `Worker` holds dependencies needed to execute work. This avoids global
mutable configuration. Ask: why a method rather than a standalone function? The
receiver supplies the relevant state and makes call sites explain responsibility.

### Interfaces

`type Reader interface { Read([]byte) (int,error) }` describes behavior. Go satisfies
interfaces implicitly. PulseRoute uses `http.Handler`, `io.Reader` and
`http.ResponseWriter`; it does not wrap every database operation in an invented
interface. `httptest.NewServer` accepts the real handler. Ask: when add an interface?
At a genuine alternative implementation or useful testing boundary, not by default.

### Pointers and values

`p := &Job{ID:"x"}` stores an address; `p.ID` accesses its field. A `*pgxpool.Pool`
must be shared, not copied into a new independent connection manager. `*bool` in
endpoint input distinguishes omitted `active` from an explicit false value. Ask:
how does a PATCH request preserve false? A pointer provides presence information;
nil and false are different states.

### Slices

`items := []string{}; items = append(items,"one")` manages a variable-size sequence.
The API's `rows` helper collects result objects in a slice and returns an empty
array instead of null. Worker payloads are `[]byte` so signing uses exact bytes.
Ask: what happens when appending exceeds capacity? Go may allocate a new backing
array; other slices can still refer to the old one. Do not share mutable slices
between goroutines without an ownership or synchronization rule.

### Maps

`counts := map[string]int{}; counts["x"]++` indexes by key. The demo receiver counts
attempts per delivery in a map protected by a mutex. API response maps are local
to a request. Ask: are Go maps safe for simultaneous writes? No; use a mutex or
single ownership. The receiver also bounds its demo map at 10,000 entries.

### Errors and wrapping

`if err != nil { return fmt.Errorf("open database: %w",err) }` adds context without
losing the underlying error. `errors.Is(err,pgx.ErrNoRows)` distinguishes an empty
queue from a broken database. Infrastructure failures are logged; client responses
avoid exposing database internals. Ask: `%w` versus `%v`? `%w` supports unwrapping.
Normal failures return errors; only unavailable OS entropy triggers a panic.

### defer

`defer file.Close()` schedules cleanup for function exit. Migrations and queue
completion `defer tx.Rollback(ctx)` immediately after beginning a transaction. A
later successful Commit makes the deferred rollback harmless. This covers early
returns. Ask: does defer run immediately? The arguments are evaluated immediately,
but the call happens at return; deferred calls execute in reverse order.

### Goroutines

`go work()` schedules concurrent execution. `Worker.Run` starts exactly N worker
goroutines and an HTTP server runs independently in `app.Run`. Goroutines are
lightweight, not free. Ask: why not one goroutine per event? An unbounded burst
can exhaust memory, sockets, database connections and destination capacity.

### Channels and buffered channels

`jobs := make(chan Job,5); jobs <- job; j := <-jobs` transfers a value. PulseRoute
uses a buffered jobs channel and a buffered `struct{}` channel as capacity tokens.
Buffering permits limited decoupling; a full channel blocks its sender. Ask: why
have slots if jobs is already bounded? To bound jobs already claimed from SQL as
well as jobs running, so prefetched leases do not expire waiting in memory.

### select

`select { case <-ctx.Done(): return; case jobs <- j: }` waits for one ready
operation. The worker uses it for cancellation, slots, dispatch and idle polling.
Ask: does select prioritize cancellation? No; if both cases are ready, either can
win. The worker also checks context in `process`, and any abandoned claim recovers
by lease expiration. Do not assume scheduling order proves a strict guarantee.

### context.Context

`ctx,cancel := context.WithTimeout(parent,time.Second); defer cancel()` creates a
deadline under a parent. Request context flows into SQL and Redis; worker context
flows into HTTP. Cancellation propagates downward. Ask: should context be stored
as business state? Generally pass it as the first parameter; it controls a call's
lifetime, not a job's durable identity. Durable scheduling lives in PostgreSQL.

### Timeouts and cancellation

`http.NewRequestWithContext(ctx,...)` lets cancellation abort I/O. Both the worker's
request context and HTTP client enforce limits. The server sets header/read/write
timeouts too. Ask: does a timeout prove the destination did nothing? No. It only
means the caller did not obtain a timely response; duplicate effects remain possible.

### WaitGroup, mutex and atomics

`wg.Add(1); go func(){ defer wg.Done(); ... }(); wg.Wait()` joins workers before
closing pools. `mu.Lock()` protects the receiver map and load-test latency slice.
`atomic.Int64.Add(1)` counts load-test errors without a separate mutex. Ask: when
use which? A WaitGroup waits for completion; a mutex protects a multi-step invariant;
an atomic is useful for one independent numeric counter.

### HTTP server and middleware

`mux.HandleFunc("GET /health",handler)` registers a route. Middleware wraps the
handler: `func(next http.Handler) http.Handler`. PulseRoute assigns request IDs,
rejects cross-origin mutations and logs status/duration. A response recorder wraps
`ResponseWriter` to capture status. Ask: why set timeouts on the server? Clients
should not hold resources forever by slowly sending headers or bodies.

### JSON

`json.NewDecoder(r.Body).Decode(&input)` decodes into a struct; tags select field
names. PulseRoute disallows unknown fields and trailing values. `json.RawMessage`
preserves nested payload bytes until canonicalization, which uses `UseNumber`.
Ask: why not decode every number into float64? Integers above 2^53 can collapse,
breaking payload fidelity and idempotency comparisons. There is a regression test.

### Database access

`pool.QueryRow(ctx,"SELECT ... WHERE id=$1",id).Scan(&value)` binds parameters instead
of concatenating SQL. `Begin`, `Commit` and `Rollback` express transaction boundaries.
`defer rows.Close()` returns resources to the pool. Ask: what is a connection pool?
A bounded manager of reusable database connections, not a cache of query results.

### Graceful shutdown

`signal.NotifyContext(...,os.Interrupt,syscall.SIGTERM)` connects OS shutdown to
cancellation. The server stops accepting requests, active HTTP work is bounded,
the producer closes the jobs channel, workers exit, WaitGroup completes, pools
close. Ask: who closes a channel? The producer/owner that knows no more values will
be sent. Consumers closing it can cause send-on-closed-channel panics.

## 4. Concurrency and overload, deeply

One worker serializes all destination latency. If each request takes 100 ms, its
rough upper bound before database overhead is 10 requests/s. N workers overlap I/O;
this is concurrency, not a promise of N CPU cores or linear throughput. The actual
small 10 ms receiver experiment is in `performance.md` and shows diminishing returns.

Unlimited goroutines merely move a durable queue into memory and push overload onto
the database/destinations. PulseRoute reserves a slot before each SQL claim; at
most N jobs per process have leases active in memory. Jobs beyond that remain
durable. This is backpressure. The channel buffer is N, but the slot cap includes
buffered and executing jobs, so it does not allow an extra N prefetched leases.

The dispatcher alone closes the jobs channel after it stops sending. Consumers
release slots after processing. WaitGroup waits for all consumers before resources
close. The queue claim uses a database row lock; it is not a Go mutex, so it works
across processes. Fencing checks lease ownership at completion. Redis limits are
also cross-process; a local mutex could not enforce a shared endpoint rate.

| Scenario | Actual behavior | What you should inspect |
|---|---|---|
| 100 simultaneous events | API-key burst may accept 100; unique IDs create durable fan-out. Only N deliveries per worker process execute at once. | Accepted events versus deliveries, queue age, endpoint limits. |
| 10,000 arrivals | API-key limit rejects excess with 429; accepted work stays in SQL. A producer should retry with stable IDs/backoff. | 429 rate, DB latency, backlog growth, memory. |
| Destination takes 30s | Default HTTP timeout cancels at 10s; retry may follow. Receiver might still finish a side effect. | Status code 0, timeout reason, duration and receiver logs. |
| Worker crashes | Claimed rows remain processing until lease expiry; another worker reclaims. Duplicate HTTP is possible. | lease_until, worker restarts, attempt gaps. |
| API crashes before commit | Transaction rolls back; producer retries. | Producer response and unique constraint behavior. |
| API crashes after commit, before response | Durable jobs exist; producer retries same ID and gets duplicate result. | Stored event and corresponding deliveries. |
| Redis unavailable | Ingestion/auth fail closed; worker reschedules without consuming attempts; metrics query SQL. | Readiness and limiter logs; do not delete queued jobs. |
| PostgreSQL unavailable | New events cannot be accepted; claims/completions fail. HTTP success may need re-delivery after recovery. | DB health/pool waits and lease recovery. |

Exercise: stop the worker, ingest an event, verify pending state, restart worker,
and see it deliver. Next kill it while a slow destination is active. Explain why
the absence of an attempt row does not prove no HTTP request was sent.

## 5. Database concepts and the actual schema

| Table | Meaning and relationships |
|---|---|
| users | Unique normalized email, bcrypt hash. Owns projects and sessions. |
| sessions | Hashed random token, user reference, absolute expiry. Logout deletes it. |
| projects | Owner authorization boundary. Groups keys, endpoints and events. |
| api_keys | Project, name, prefix, hash, revocation timestamp. Raw key shown once. |
| endpoints | URL, encrypted secret, active/deleted flags and retry/rate policy. Soft deletion preserves historical references. |
| events | Generic JSON envelope, external event ID, type, canonical hash, project. Unique project/external ID. |
| deliveries | One event/endpoint pair, status, schedule, attempts, generation and lease. Also the durable queue. |
| delivery_attempts | Completed result with generation, attempt number, status, latency and safe error category. |
| dead_letters | One current terminal failure reason per delivery; removed on replay. |

A primary key identifies a row. A foreign key prevents a delivery referencing a
nonexistent event. A unique constraint prevents duplicate business identity even
when two requests race. A CHECK keeps impossible retry settings out of SQL.
Application validation improves errors; database constraints protect invariants.

A transaction makes changes atomic. Ingestion event+fan-out and completion
attempt+state+dead-letter are separate transactions. Network HTTP is intentionally
outside a SQL transaction: holding row locks across slow destinations would exhaust
connections and serialize unrelated work. That separation creates the unavoidable
external-effect/acknowledgment gap.

READ COMMITTED is sufficient for ingestion because the unique constraint arbitrates
writes and subsequent statements take fresh snapshots. It does not make every
multi-statement workflow serializable. Replay locks the delivery row; claims use
SKIP LOCKED to avoid selecting a locked candidate. A lock is released at transaction
end, unlike a lease, which is a stored deadline surviving a crashed process.

Indexes are ordered access paths. `events_project_time` serves recent project events;
`events_type_time` adds type equality before the ordered timestamp/ID suffix.
`deliveries_due` contains pending/retrying rows only, and `deliveries_expired` contains
processing leases only. Fewer indexed rows makes queue lookups cheaper, but every
state update can alter index membership and produce vacuum work. Indexes cost disk
and write I/O; do not add one for every column without a query reason.

`EXPLAIN` shows the planned path; `EXPLAIN ANALYZE` actually runs it and reports
timings/rows. The included 20,000-event experiment uses a transaction and rollback.
Read `docs/query-plans.txt`: with the type/time index it returned 25 matching rows
directly; without it the project/time scan removed 2,475 rows by filter. Those are
real observations for that dataset, not a universal speedup. Buffer hits, row
estimates, planning time and cache state matter as well as execution time.

N+1 means fetching N rows and then making N extra queries for their related data.
Inspection uses joins and an aggregate for attempt history instead. List queries
bound results; nevertheless deep offsets and lifetime metric aggregates will cost
more as history grows. Connections are capped at 20 per process: four API replicas
plus four workers could request 160 connections. Scaling replicas without budgeting
database capacity can reduce reliability.

## 6. Redis: useful ephemeral state

Redis stores fast mutable counters and short-lived cache values. PostgreSQL stores
the record we cannot lose. A cache value can be recomputed; an accepted event cannot.
This distinction decides what belongs where.

The Lua bucket script reads Redis TIME, refills tokens according to elapsed time,
caps at capacity, optionally spends one token, writes the new state, and assigns
TTL. The operation is atomic relative to other commands. Expiry is refill duration
plus 60 seconds: an idle bucket would already be full when it disappears. Different
API hosts do not rely on their local clocks for this calculation.

An endpoint bucket has rate and capacity equal to its configured requests/second.
Ingestion uses rate 50, capacity 100. A short burst can exceed 50/s and still respect
the algorithm; sustained behavior matters. Worker rate rejection does not count as
an HTTP attempt. It schedules another chance one second later.

Metrics cache keys include owner/project, preventing cross-tenant reuse. Values
expire after five seconds; that TTL is the invalidation policy. UI Refresh does not
promise a cache bypass. Cache read, parse or write failures leave SQL as the fallback.
If Redis disappears, events/jobs do not disappear. Acceptance and sending pause to
preserve shared rate limits; a recovered empty Redis permits a new burst. This is
a conscious availability-versus-protection trade-off, not a silent fail-open path.

## 7. Distributed systems without slogans

At-most-once would mean never retrying an uncertain request; that can lose events.
At-least-once attempts permit duplicates while giving transient failures another
chance. Exactly-once business effects require cooperation from the receiver's own
database transaction or a stronger end-to-end protocol; HMAC and queue locks alone
cannot provide it.

Idempotency means repeating the same logical operation does not create a new result.
Here the producer's stable event ID is the ingestion key. It must persist across
producer retries. A random new ID on every retry defeats this protection.

Eventual consistency appears between acceptance and the dashboard's delivered
state, and again in the five-second metrics cache. Returning 202 means accepted,
not delivered. A dead letter is an explicit end to automatic retries, not data loss.
Replay is an operator decision with preserved history, not erasing failure evidence.

Partial failure is the central problem: receiver succeeds, response gets lost;
Redis fails while PostgreSQL is healthy; worker records no result because SQL is
down after a successful HTTP response. Timeouts bound waiting but cannot tell which
side of the network completed a side effect. Backpressure controls intake/work
capacity, retries need jitter, and destinations must defend against duplicate effects.

## 8. How the system would scale

**100 events/day:** the current Compose topology is more than enough for a learning
demo. Prefer correctness, backups, simple logs and predictable costs. A sleeping
free host means delayed delivery and must be described honestly.

**100,000/day:** average is about 1.16 events/s, but fan-out and bursts matter more
than the daily average. Ten destinations means ten times the delivery work. Measure
queue age and p95 latency; scale API/worker independently behind a load balancer.
Current SQL locks and Redis buckets coordinate replicas, but database pool capacity,
indexes, retention and destination fairness need attention. Read replicas can serve
some stale analytics reads; they should not arbitrate claims or read-your-write auth.

**10 million/day:** average is about 116 events/s before fan-out and bursts. Do not
claim the project has demonstrated this. Add aggregate metrics, partition/retention
plans, load tests with realistic payloads and destination latency, and queue-age
alerts before choosing new technology. An outbox relay to Kafka/PubSub or another
durable broker may separate ingestion storage from scheduling, but adds publish
deduplication, consumer offsets, operations and replay semantics. Partition by a
stable ordering/fairness key when justified. Sharding is a late response to measured
single-database limits, not an automatic first step.

Implemented: separate API/worker processes, bounded concurrency, shared SQL claims,
Redis limits, indexes and health checks. Proposed: autoscaling, broker, partitions,
replicas, sharding, multi-region routing and SLO alerting. Kubernetes YAML alone does
not implement a working production autoscaling system.

## 9. Testing and confidence

Unit tests exercise signature tampering/staleness, authenticated encryption, password
verification, private address rejection, retry classification/jitter, config limits,
JSON validation/precision and API errors/pagination. These need no running database.

`tests/integration_test.go` runs the actual API and worker against real PostgreSQL
and Redis. `httptest` supplies real local HTTP servers, not mocked successful return
values. Twelve simultaneous submissions test idempotency arbitration. The suite
checks recovery after two 500s, 400 termination, 503 exhaustion, timeout, replay
history, another owner's 404, token buckets and stale lease completion rejection.
If service variables are absent, the test explicitly skips. A skipped integration
test is not evidence of an integrated pass.

`frontend/src/api.test.ts` mocks fetch to check the browser client's JSON/error/cookie
contract. `frontend/e2e/workflow.spec.ts` uses a real browser and application: register,
create project/endpoint/key, ingest, inspect successful delivery, check mobile width.
It also saves screenshots. CI runs this against built Compose images, not the Vite
development server. Failed traces and Compose logs are uploaded as artifacts.

The Go race detector reports unsynchronized memory accesses that execute during the
test; it cannot prove absence of all races or distributed consistency bugs. SQL
unique constraints and fenced lease tests protect different invariants. Windows
without a C compiler cannot run `-race`; Linux CI runs it. Performance experiments
are opt-in because environment timing should not decide an ordinary correctness gate.

## 10. Docker, CI/CD and Kubernetes

A Dockerfile is a build recipe. An image is the result; a container is a running
instance. The backend image uses Node to compile the dashboard, Go to compile
commands, and a small runtime with CA certificates and a non-root user. Build stages
do not ship compilers into the final runtime. CA certificates are needed for HTTPS.

Compose defines the local network, dependencies, health checks and PostgreSQL volume.
Service names such as `receiver` resolve inside that network, not in the browser's
host DNS. The frontend Nginx image proxies `/api` and `/v1` to the API using the
original Host so origin checks remain valid. The volume survives `down`; deleting
it deliberately deletes local database data. A healthy process is not necessarily
ready to accept work, hence separate `/health` and `/ready` checks.

GitHub Actions CI checks formatting, vet, race-enabled tests with real services,
frontend lint/tests/build and all commands. A separate job builds/starts Compose,
seeds data and runs browser E2E. Each step can fail. CD means taking a verified
artifact to an environment; Render can redeploy connected Git commits but account
setup and a verified public rollout are separate from CI. Do not claim either a
green run or successful deployment unless its actual result is recorded.

Kubernetes mapping: a Pod runs containers; a Deployment maintains desired replicas;
a Service gives stable internal access; ConfigMap holds non-secrets; Secret holds
credentials (base64 is not encryption); probes distinguish ready/live; requests
reserve scheduling capacity and limits cap consumption. In `deployments/kubernetes`,
API/worker Deployments have probes, limits and restricted containers. A migration
Job runs before rollouts. PostgreSQL/Redis and HTTPS ingress are external concerns.
Manifests are provided, not evidence of a live production cluster.

## 11. Debugging playbook

Always establish scope and timing first. Is one project affected, one endpoint,
all deliveries, or only the displayed metrics? Use request ID for API logs and
delivery/event/endpoint IDs for worker logs. Do not paste credentials into logs.

| Symptom | Investigation | Likely action |
|---|---|---|
| High delivery latency | Compare HTTP attempt duration with queue age; inspect retries and worker count; look at database pool waits and endpoint bucket rate. | Fix slow receiver or bottleneck before adding workers. More workers cannot override rate policy. |
| Duplicate business effects | Compare producer IDs and delivery IDs; check timeout/crash window and receiver transaction. | Keep producer IDs stable and add receiver-side unique business key. Do not remove retries blindly. |
| High 5xx rate | Check destination status and upstream changes; inspect signature/URL config and retry timing. | Restore dependency, then replay dead letters; avoid retry storms. |
| PostgreSQL slowdown | Check active queries, locks, pool saturation, EXPLAIN ANALYZE, index usage and vacuum. | Bound traffic; optimize measured hot query; don't add replicas to fix write contention. |
| Redis outage | Readiness fails, auth/ingestion 503, workers defer without HTTP attempts. | Restore Redis; expect a fresh token burst; verify PostgreSQL backlog retained. |
| Worker crash | Inspect process logs, memory and restart cause; compare lease_until to current time. | Restart, let leases expire; don't manually mark unverified requests successful. |
| Stuck retries | Check next_attempt_at, current clocks, endpoint enabled state, worker health and Redis availability. | Correct config/dependency; preserve history; replay only a dead delivery after diagnosis. |
| Memory increase | Separate DB history growth from process heap; inspect goroutine count, payload sizes, response limits and local receiver map. | Profile before changing concurrency. Buffers/limits are bounded; DB retention remains operational work. |
| UI appears stale | Refresh; inspect network errors, selected filters, tenant and cache TTL. | Wait at most cache TTL before diagnosing durable state; never replace metrics with made-up values. |

Practice aloud: "I would first distinguish time spent waiting in the queue from
time spent in the outbound request. Their fixes are different." This is stronger
than immediately suggesting more goroutines.


## 12. Fifty-two interview questions and defensible answers

Use these as practice prompts. Answer from the code, then demonstrate the behavior.

### Go

**1. Why use Go here?**

Strong answer: Its standard HTTP stack, explicit errors and cancellation fit an I/O-heavy worker. Goroutines overlap outbound waits while a channel bounds work.

Simple explanation: Go makes concurrent network work straightforward, but limits still matter.

Reference: `cmd/api/main.go; internal/delivery/worker.go`.

**2. Why avoid interfaces for every package?**

Strong answer: The concrete pgx/Redis clients have one production implementation. Real-service tests exercise their semantics; interfaces are reserved for meaningful behavioral boundaries such as http.Handler.

Simple explanation: An interface should solve a problem, not decorate every struct.

Reference: `internal/database/database.go; internal/api/server.go`.

**3. Why use a pointer to bool in PATCH input?**

Strong answer: Omitted active must retain the old value; explicit false must pause the endpoint. A bool alone cannot distinguish those states after decoding.

Simple explanation: Missing and false are different instructions.

Reference: `internal/api/endpoints.go: endpointInput, updateEndpoint`.

**4. How are errors propagated?**

Strong answer: Helpers return errors, database setup wraps causes with %w, and handlers map not-found versus internal failures. Internal details go to structured logs, not client responses.

Simple explanation: Keep the original cause for debugging while keeping public messages safe.

Reference: `internal/database/database.go: Open; internal/api/server.go: dbError`.

### Concurrency

**5. Why use both slots and a jobs channel?**

Strong answer: Slots are reserved before SQL claims and released after processing. They bound claimed plus active work, while the jobs channel transports jobs to fixed consumers.

Simple explanation: Do not lease more work than the process can promptly handle.

Reference: `internal/delivery/worker.go: Run`.

**6. Can closing the channel race with a send?**

Strong answer: The dispatcher is the sole sender/closer and closes only after its dispatch loop returns. Workers only receive. A WaitGroup joins consumers before resources close.

Simple explanation: One owner decides when no more jobs will arrive.

Reference: `internal/delivery/worker.go: Run`.

**7. What prevents two workers taking the same due job?**

Strong answer: The claim statement locks a candidate with FOR UPDATE SKIP LOCKED and updates its lease atomically. Other transactions skip locked candidates.

Simple explanation: Database coordination works across processes; a Go mutex would not.

Reference: `internal/delivery/queue.go: Claim`.

**8. What happens on SIGTERM?**

Strong answer: The signal context cancels claims and HTTP. The producer stops, closes jobs and waits for consumers. Unfinished leases later expire rather than being silently lost.

Simple explanation: Stop work safely; leave recoverable durable state.

Reference: `internal/app/app.go: Run; internal/delivery/worker.go: Run`.

### REST APIs

**9. Why return 202 for ingestion?**

Strong answer: It acknowledges committed event/jobs, not completed HTTP delivery. Consumers use inspection endpoints to follow asynchronous state.

Simple explanation: Accepted is different from delivered.

Reference: `internal/api/ingest.go: ingest`.

**10. When is a duplicate 200 versus 409?**

Strong answer: The same project/event ID with identical canonical content returns its existing internal ID and duplicate=true. Changed content returns an idempotency conflict.

Simple explanation: A retry is okay; reusing identity for another event is not.

Reference: `internal/api/ingest.go; internal/events/events.go`.

**11. How are lists bounded?**

Strong answer: Limit defaults to 25 and is capped at 100; offset is bounded and sorting uses timestamp plus ID. Filtering is parameterized. Deep offsets remain a known cost.

Simple explanation: Clients cannot request an unlimited result in one call.

Reference: `internal/api/server.go: page; internal/api/inspection.go`.

**12. How can an operator correlate an API error?**

Strong answer: The middleware generates an X-Request-ID, places it in the error envelope and structured request log. Worker logs use event/delivery IDs for the asynchronous continuation.

Simple explanation: Follow request identity into the stored event, then delivery identity.

Reference: `internal/api/server.go: middleware, fail; internal/delivery/worker.go`.

### PostgreSQL

**13. Why store the queue in PostgreSQL?**

Strong answer: The ingestion transaction inserts the event and all jobs atomically. This avoids a database/broker dual-write gap without implementing an outbox relay.

Simple explanation: One commit covers accepting the event and scheduling its work.

Reference: `internal/api/ingest.go; docs/decisions/002-postgresql-queue.md`.

**14. Is READ COMMITTED enough for idempotency?**

Strong answer: Yes for this invariant: the unique constraint arbitrates concurrent insertion, and the next statement sees a fresh committed snapshot. It does not generally eliminate every application race.

Simple explanation: The unique constraint does the hard arbitration.

Reference: `migrations/001_initial.sql; internal/api/ingest.go`.

**15. Why partial queue indexes?**

Strong answer: Pending/retrying and processing rows serve different due/expiry scans. Partial indexes retain only relevant rows, at the cost of index updates as states change.

Simple explanation: Index the active work, not every historical success.

Reference: `migrations/001_initial.sql: deliveries_due, deliveries_expired`.

**16. What did EXPLAIN ANALYZE actually show?**

Strong answer: On the included synthetic dataset the composite type/time index avoided filtering 2,475 project rows. Captured timings and buffers are in the report; one run is not a general speedup claim.

Simple explanation: Read the actual plan, including filtering and planning overhead.

Reference: `scripts/explain.sql; docs/query-plans.txt`.

### Redis

**17. Why Redis if PostgreSQL already exists?**

Strong answer: Atomic, expiring token buckets coordinate API and worker replicas; a five-second cache reduces repeat metric aggregation. Durable business records stay in PostgreSQL.

Simple explanation: Use Redis for short-lived coordination, not irreplaceable events.

Reference: `internal/redisx/redis.go`.

**18. Why Lua rather than GET then SET?**

Strong answer: Separate read/modify/write commands race across clients. The script refills and spends tokens atomically using Redis's clock.

Simple explanation: Only one combined operation decides whether a token is available.

Reference: `internal/redisx/redis.go: bucket`.

**19. What happens if Redis loses all data?**

Strong answer: Events and jobs survive. Cache entries rebuild; token buckets restart full and permit a fresh burst. During outage acceptance/sending are paused rather than failing open.

Simple explanation: Lost counters are tolerable; lost accepted events would not be.

Reference: `internal/redisx/redis.go; internal/api/ingest.go; internal/delivery/worker.go`.

**20. How is cached data invalidated?**

Strong answer: Metrics keys are owner/project scoped and expire in five seconds. Mutations do not synchronously invalidate them, so totals can briefly lag. Cache failure falls back to SQL.

Simple explanation: Short-lived staleness is an explicit dashboard trade-off.

Reference: `internal/api/inspection.go: dashboard; internal/redisx/redis.go`.

### Service boundaries

**21. Are API and worker microservices?**

Strong answer: They are independently runnable service-oriented processes with separate runtime responsibilities, but share a database/schema and codebase. Do not claim independent domain data ownership.

Simple explanation: Separate deployment does not automatically mean fully autonomous services.

Reference: `cmd/api; cmd/worker; docs/decisions/008-service-boundaries.md`.

**22. Why not deliver inside the API handler?**

Strong answer: Slow or failing destinations would delay ingestion and tie acceptance to their availability. Durable jobs decouple acceptance from network delivery.

Simple explanation: Store work first; do slow work in the background.

Reference: `internal/api/ingest.go; internal/delivery/worker.go`.

**23. Why a combined demo command?**

Strong answer: Free web hosting may lack a free background-worker type. The demo reuses the same API and worker lifecycle in one instance, with idle-suspension limitations.

Simple explanation: A hosting compromise, not a different reliability algorithm.

Reference: `cmd/demo/main.go; internal/app/app.go`.

**24. What happens when replicas increase?**

Strong answer: SQL row locks and Redis buckets coordinate shared work, but every replica has its own connection pool. Total connections and queue-update contention must be budgeted.

Simple explanation: More processes can overload a shared dependency.

Reference: `internal/database/database.go; deployments/kubernetes/workloads.yaml`.

### Distributed systems

**25. Is delivery exactly once?**

Strong answer: No. A receiver may commit before the worker records success; a crash then causes a retry. Fencing protects database updates, not already performed HTTP effects.

Simple explanation: The receiver must deduplicate its own side effects.

Reference: `internal/delivery/queue.go: Finish; docs/architecture.md`.

**26. What does the lease token prevent?**

Strong answer: After a job is reclaimed, an old worker's token no longer matches. Its late completion is rejected before an attempt/status update.

Simple explanation: Old owners cannot overwrite new owners' database state.

Reference: `internal/delivery/queue.go: Claim, Finish; tests/integration_test.go`.

**27. Why exponential jitter?**

Strong answer: Repeated failures progressively back off while randomizing retry times, reducing synchronized bursts. A cap and finite budget keep behavior bounded.

Simple explanation: Wait longer after repeated trouble, and avoid everyone retrying together.

Reference: `internal/delivery/retry.go: Backoff`.

**28. What is eventual consistency here?**

Strong answer: A committed event exists before its deliveries finish, and dashboard aggregates can be cached for five seconds. Detail reads and metrics can briefly differ.

Simple explanation: The system's views catch up as background work and cache expiry progress.

Reference: `internal/api/inspection.go; frontend/src/App.tsx`.

### System design

**29. How would you scale to ten million events/day?**

Strong answer: First measure fan-out, burst rate, destination latency, queue age and database pressure. Then add retention/rollups/partitioning and consider an outbox plus broker. Current tests do not establish that scale.

Simple explanation: Daily count alone is not a sizing model.

Reference: `docs/performance.md; docs/INTERVIEW_HANDBOOK.md section 8`.

**30. Why not Kafka now?**

Strong answer: The current workload benefits from atomic SQL scheduling and simple operational setup. Kafka would add partition/offset/replay concerns and still require solving database publication consistency.

Simple explanation: Choose a broker when measured pressure justifies its cost.

Reference: `docs/decisions/002-postgresql-queue.md`.

**31. What would trigger autoscaling?**

Strong answer: Sustained oldest-due-job age and backlog relative to completion rate are more informative than CPU alone for I/O-bound workers. Respect endpoint limits and DB connections.

Simple explanation: Scale to clear legitimate backlog, not to hammer a failing receiver.

Reference: `internal/delivery/worker.go; docs/architecture.md`.

**32. What is the first large-history optimization?**

Strong answer: Measure queries, then replace deep offsets with keyset pagination, use metric rollups and introduce retention/partitioning. Do not shard before showing a database bottleneck.

Simple explanation: Reduce unnecessary work before splitting databases.

Reference: `internal/api/inspection.go; migrations/001_initial.sql`.

### Docker and Kubernetes

**33. Why multi-stage Docker builds?**

Strong answer: Node compiles static assets and Go compiles executables in build stages; only artifacts and CA certificates reach the non-root runtime.

Simple explanation: Smaller runtime with fewer build tools exposed.

Reference: `deployments/docker/Dockerfile`.

**34. Why does receiver resolve in Compose but not on the host?**

Strong answer: Compose provides DNS on its network for service names. Native processes use localhost and published ports instead. Browser requests go to the frontend/API origin.

Simple explanation: Inside-container and host addresses are different.

Reference: `compose.yaml; README.md`.

**35. What is readiness versus liveness?**

Strong answer: Liveness checks the process; readiness checks PostgreSQL and Redis too. A dependency outage should remove readiness without provoking a restart loop.

Simple explanation: Alive does not always mean able to serve useful work.

Reference: `internal/api/server.go: ready; deployments/kubernetes/workloads.yaml`.

**36. Are Secrets encrypted by base64?**

Strong answer: No. Kubernetes Secret encoding is not encryption. Use cluster encryption/access control and an external secret workflow; never commit actual credentials.

Simple explanation: Base64 hides nothing from someone who can read it.

Reference: `deployments/kubernetes/README.md`.

### CI/CD

**37. What does CI execute?**

Strong answer: Formatting, vet, race-enabled tests with real services, frontend lint/unit/build, all command builds, shell checks, Docker builds and Compose/browser smoke tests.

Simple explanation: The repository is checked at code, service and packaged-system levels.

Reference: `.github/workflows/ci.yml`.

**38. How can a green unit suite miss a failure?**

Strong answer: Unit tests cannot detect a wrong container URL, runtime environment, migration startup order or browser layout difference. The container workflow covers those paths.

Simple explanation: Test the packaged system as well as isolated functions.

Reference: `.github/workflows/ci.yml: containers`.

**39. Is CI the same as deployment?**

Strong answer: No. Passing checks produces confidence, while CD rolls an artifact to an environment. The repository includes a Render recipe but no unverified live deployment claim.

Simple explanation: Build success does not create a verified public service.

Reference: `render.yaml; docs/deployment.md`.

**40. What real defects did validation expose?**

Strong answer: Tests caught a route-pattern conflict, a load-generator timer variable compile error, a browser selector issue and Linux mobile table overflow. Their normal fix commits preserve the history.

Simple explanation: Use genuine debugging evidence, not invented war stories.

Reference: `git log; frontend/e2e/workflow.spec.ts; internal/api/server.go`.

### Testing

**41. What is mocked in the integration test?**

Strong answer: The receiver behavior is synthetic but it is a real HTTP server. PostgreSQL, Redis, API handlers and worker execution are real. Frontend unit tests separately mock fetch.

Simple explanation: Simulate the remote business system while testing actual network/storage behavior.

Reference: `tests/integration_test.go; frontend/src/api.test.ts`.

**42. How is concurrent idempotency tested?**

Strong answer: Twelve goroutines submit the same event. Exactly one must return 202, others 200, and the database must have one delivery for the endpoint. Changed content must return 409.

Simple explanation: Force the race instead of testing two sequential calls only.

Reference: `tests/integration_test.go: TestIntegrationWorkflow`.

**43. What does the race detector prove?**

Strong answer: It detects conflicting unsynchronized memory accesses exercised by that run. It does not prove all schedules safe or check SQL isolation and external effects.

Simple explanation: It catches a class of runtime bugs, not every concurrency bug.

Reference: `scripts/test.sh; tests/integration_test.go`.

**44. Why not claim the benchmark as capacity?**

Strong answer: Only 60 events per pool size were tested with a warm loopback 10 ms receiver. Token-bucket burst capacity also inflates short ingestion rates above sustainable policy.

Simple explanation: Report the exact experiment and its limits.

Reference: `tests/performance_test.go; docs/performance.md`.

### Production debugging

**45. Where would you start with rising latency?**

Strong answer: Separate queue wait from attempt duration, compare endpoint distribution, inspect worker saturation, bucket limits and SQL/pool waits. More workers may not help a throttled endpoint.

Simple explanation: Find where time is spent before changing capacity.

Reference: `delivery_attempts schema; internal/delivery/worker.go`.

**46. Why is a delivery stuck in processing?**

Strong answer: Its worker may have crashed or lost database access. Check lease_until, process health and whether another worker is polling. Recovery happens after expiry, not immediately.

Simple explanation: A processing row can be waiting for crash recovery.

Reference: `internal/delivery/queue.go: Claim`.

**47. Why does the dashboard differ from a delivery detail?**

Strong answer: The aggregate cache can lag for five seconds; filters and ownership scopes can also differ. Verify the durable row and network response before assuming data corruption.

Simple explanation: Cached totals are approximate views of live work.

Reference: `internal/api/inspection.go: dashboard`.

**48. How should dead-letter replay be used?**

Strong answer: Inspect the error and fix the endpoint or receiver first. Replay starts a new generation with preserved history; blindly replaying a permanent failure wastes capacity.

Simple explanation: Repair the cause before resubmitting the work.

Reference: `internal/api/inspection.go: replay; frontend/src/App.tsx: DeliveryDetails`.

### Security

**49. Why hash API keys but encrypt webhook secrets?**

Strong answer: High-entropy API keys are only compared, so storing a hash is enough. Signing requires recovering the webhook secret, so authenticated encryption is necessary.

Simple explanation: Compare-only credentials and recoverable signing keys have different needs.

Reference: `internal/security/security.go; internal/api/projects.go`.

**50. Why sign timestamp plus payload?**

Strong answer: It binds both freshness metadata and exact body bytes to the secret. The receiver rejects stale timestamps and compares decoded MACs in constant time.

Simple explanation: Tampering with the message or time breaks verification.

Reference: `internal/security/security.go: Sign, Verify`.

**51. How is SSRF reduced?**

Strong answer: Require HTTPS outside demo mode, reject userinfo and unsafe addresses, resolve/check all IPs at dial time, dial the validated IP, disable redirects and environment proxies.

Simple explanation: An attacker cannot simply point a webhook at local metadata or rebind DNS between checks.

Reference: `internal/security/transport.go`.

**52. How is tenant isolation enforced?**

Strong answer: Session identity scopes project ownership in SQL. Inspection joins through projects; endpoint/key mutations check the same owner boundary. A second account's access is tested as 404.

Simple explanation: Knowing another resource ID is not authorization.

Reference: `internal/api/projects.go; internal/api/inspection.go; tests/integration_test.go`.
