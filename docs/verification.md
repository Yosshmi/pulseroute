# Verification report

Verified on 2026-09-17. Application revision:
`ee42d2a205857b57f458d269a1b78e59eaec71c0`.
Subsequent documentation-only commits do not change the tested implementation.

**Complete Linux CI passed:**
[quality run 35233340019](https://github.com/Yosshmi/pulseroute/actions/runs/35233340019).
The `verify` job passed in 53 seconds and `containers` passed in 2 minutes 4 seconds.
An earlier full successful run is
[35232829852](https://github.com/Yosshmi/pulseroute/actions/runs/35232829852).

## Executed quality gates

| Gate | Result and exact scope |
|---|---|
| gofmt | No unformatted files in cmd, internal, migrations or tests. |
| go vet | Passed for `./cmd/... ./internal/... ./migrations/... ./tests/...`. |
| Go unit tests | 10 top-level test functions passed across API, configuration, retry, events and security packages. |
| Real-service integration | `TestIntegrationWorkflow` passed; named `redis_atomic_bucket` and `lease_fencing` subtests also passed. Last local workflow elapsed 4.13s; test package 4.387s. |
| Race detector | Linux CI `go test -race -count=1` passed with PostgreSQL 18 and Redis 7 services. |
| Backend builds | All seven commands passed: api, worker, demo, receiver, migrate, seed and loadtest. |
| Frontend lint | ESLint passed. |
| Frontend unit tests | Vitest 4.1.11: 1 file, 3 tests passed, 0 failed. |
| Frontend build | TypeScript and Vite production build passed (42 modules). |
| Dependency audit | `npm audit --audit-level=moderate`: 0 vulnerabilities at check time. This is not a claim of a full independent security audit. |
| Local browser E2E | 1 Playwright workflow passed, including desktop/mobile screenshots and viewport overflow assertion; last local run 8.2s total. |
| Docker builds | Backend and frontend multi-stage images built successfully in Linux CI. |
| Compose smoke | Empty-volume PostgreSQL startup, migrations, Redis, API, worker, receiver, Nginx frontend, health checks and seed all passed in CI. |
| Packaged browser E2E | 1 workflow passed against the built Compose application, not a mocked API. |
| Shell | Bash syntax checks passed for all scripts in CI; setup and seed actually executed in the Compose job. |
| Performance | Separate opt-in experiment passed all four pool sizes: 1/5/10/25; 240 successful deliveries, zero ingestion errors. See measured table and caveats. |
| SQL index experiment | EXPLAIN ANALYZE executed on 20,000 synthetic rows; both plans captured; fixture and index change rolled back. |
| Hygiene | No inappropriate TODO/FIXME, debug logging, sample example.com URL, or private-key/token-pattern findings. Only legitimate HTML input hints matched `placeholder`. |

The ordinary Go test run deliberately skips `TestPerformance` unless
`RUN_PERFORMANCE=1`. It was separately executed, not silently counted as passed in
the ordinary run. Packages without test files are not counted as test functions.
There are 13 passed named Go test events when the two integration subtests and
their parent are counted; this does not mean 13 independent unit tests.

## What the integration workflow establishes

- Empty database migrations apply and re-apply successfully.
- Twelve concurrent identical requests produce one accepted event/fan-out; changed
  content under the same identity returns 409.
- Signed receiver fails twice then succeeds, leaving three attempt records.
- Permanent 400 ends after one attempt; 503 exhausts its configured budget.
- A slow receiver reaches timeout and dead-letter state.
- Replay preserves old attempts and increments generation.
- Another account cannot inspect/mutate the first account's project resources.
- Real Redis buckets reject excess tokens.
- Expired leases can be reclaimed; old-token completion is rejected.

## Environment limits and honest deployment status

Local checks ran on Windows amd64 with Go 1.27.1, Node 24.18.1, PostgreSQL 18.4
through portable test tooling and Redis 5.0.14.1's Windows community build on
loopback only. Linux CI independently exercises official PostgreSQL 18 and Redis 7
containers. Local Docker's engine could not start; Docker/Compose validation was
therefore performed on the GitHub runner. Local `-race` was unavailable without a
C compiler; the Linux race result above supplies that check.

Kubernetes manifests were authored/reviewed but not applied to a cluster. No
Kubernetes runtime success is claimed. No verified public application deployment
exists, and no cloud resources or paid tiers were created. Hosting account setup
and credentials are the only remaining deployment actions; follow
[deployment.md](deployment.md). Provider free-tier limitations are recorded there.

The final CI run emits a non-failing warning that some actions declare the older
Node 20 runtime while GitHub runs them under Node 24. All jobs completed successfully.
The warning is retained here rather than represented as a failure or hidden.

## Local-only personal material

`docs/INTERVIEW_HANDBOOK.md` and `docs/CHEAT_SHEET.md` exist in the owner's local
workspace and are excluded from both Git and Docker context. The current repository
tree does not contain them. Earlier handbook versions remain in old commits because
history was not rewritten. Code walkthrough and resume evidence remain public as
requested in the original brief.

## Reproduce

```bash
bash scripts/setup.sh
docker compose up --build -d --wait
bash scripts/seed.sh --events 30
```

For the full source/service checks, use dedicated test PostgreSQL/Redis, export
`TEST_DATABASE_URL` and `TEST_REDIS_URL`, then `bash scripts/test.sh`. Never use real
business data in integration/performance experiments. Detailed commands and failure
boundaries are in the README, architecture, performance and final review documents.
