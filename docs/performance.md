# Measured performance, not capacity claims

## Local worker comparison — 2026-09-17

Executed `RUN_PERFORMANCE=1 go test -run TestPerformance -v ./tests` against actual
PostgreSQL 18.4 and Redis 5.0.14.1 (Windows community build), using Go 1.27.1 on
Windows amd64. Linux CI uses Redis 7. Each subtest ingested 60 synthetic events
sequentially over HTTP, then started a fresh worker pool to drain those jobs.
One signed loopback receiver sleeps 10 ms per request. Endpoint limit: 1,000/s.
No other project workload was deliberately generated during measurement.

| Workers | Ingestion requests/s | Completed deliveries/s | Drain seconds | Mean attempt ms | P95 attempt ms | Ingestion errors |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 850.09 | 72.98 | 0.8222 | 10.22 | 11 | 0/60 |
| 5 | 808.65 | 249.85 | 0.2401 | 10.07 | 11 | 0/60 |
| 10 | 877.69 | 598.67 | 0.1002 | 10.13 | 11 | 0/60 |
| 25 | 889.57 | 649.91 | 0.0923 | 10.03 | 10 | 0/60 |

All 240 deliveries succeeded. The initial API-key token bucket burst is 100, so
these short ingestion bursts can exceed the sustained 50 requests/s rate. This
does **not** demonstrate sustainable ingestion at 850 requests/s. Delivery latency
above measures the HTTP attempt, excluding queue wait; drain timing includes
polling/observation overhead. One small run has no confidence interval. Loopback,
warm caches, Windows scheduling and the synthetic receiver make these unsuitable
for production sizing. More workers reduced this small backlog's drain time, with
diminishing returns between 10 and 25 workers.

## Reproduction and a larger experiment

Use dedicated test services and export `TEST_DATABASE_URL` and `TEST_REDIS_URL`.
Then run the command above (PowerShell: `$env:RUN_PERFORMANCE='1'`). Repeat multiple
times and retain full outputs; do not compare runs from different hardware as an
index or code improvement. Test slow destinations, multiple endpoints, Redis
failure, and database pool saturation before making deployment sizing claims.

`API_KEY=... bash scripts/load-test.sh --events 100 --concurrency 5` reports measured
HTTP throughput, P50/P95 request latency, and error rate. It intentionally counts
429s as failures instead of hiding them through retries. For sustained testing,
pace clients below the configured limit. The Go performance test also measures
delivery throughput with worker counts 1, 5, 10 and 25.

## Index experiment

Run `scripts/explain.sql` using psql against a disposable database after migrations.
It creates 20,000 synthetic events inside a transaction, compares the same query
with and without the `events_type_time` index, and rolls back all changes.
The original path can scan many project events and filter/sort by type and time.
The composite index starts with equality fields and ends with the ordering fields,
allowing an ordered limited scan. Its costs are storage, write amplification, and
vacuum/index maintenance. The planner may prefer a sequential scan for small or
low-selectivity datasets; forcing an index is not a performance fix. No index
speedup is claimed without the captured EXPLAIN ANALYZE output.
