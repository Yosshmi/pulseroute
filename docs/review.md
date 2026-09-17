# Final engineering review

Reviewed against the attached implementation brief on 2026-09-17. The code is a
portfolio implementation with executable evidence, not a claim of production use.

| Area | Conclusion and evidence | Remaining boundary |
|---|---|---|
| Architecture | API/worker share concrete packages and a schema. No unused broker or interface hierarchy. | Coupled schema migrations; combined demo is a hosting compromise. |
| Acceptance consistency | Unique project/event ID plus transactional event/job insertion. Concurrent test accepts once. | Canonical numeric spellings can conflict; destinations snapshot at acceptance. |
| Queue ownership | SKIP LOCKED claim, expiry and token-checked completion. Stale-owner test passes. | Fencing cannot undo an HTTP side effect. |
| Retry transactions | Attempt, status and dead-letter update commit together. Replay keeps generations. | No Retry-After, circuit breaker or delivery ordering. |
| Concurrency | Fixed consumers; capacity reserved before claim; dispatcher owns close; WaitGroup joins. | Horizontal scale still shares SQL/Redis capacity. |
| Context/resources | HTTP/SQL/Redis receive contexts; HTTP timeouts; bounded body reads; rows/bodies/pools close. | Abrupt process death leaves leases until expiry. |
| Redis failure | Acceptance fails closed; worker reschedules; dashboard cache falls back. | Reset permits a fresh burst; public free-tier quotas matter. |
| Credentials | Password hashes, token hashes, AES-GCM secrets; no actual credentials tracked. | No managed key rotation, password recovery or email verification. |
| Destination security | HTTPS default, dial-time IP checks, no redirects or env proxies. | Add network egress policy in production; local mode intentionally permits private destinations. |
| Authorization | Owner-scoped SQL and negative integration checks for another account. | No team roles; production registration needs abuse controls. |
| UI accuracy | Metrics come from SQL/cache; errors show unavailable values instead of zero. Browser verifies actual delivery. | Explicit refresh and five-second aggregate cache; large lists use offsets. |
| Observability | Structured IDs, status and duration; no payload/response-body logging. | No distributed tracing or SLO alerting backend. |
| Testing | Real-service workflow, unit logic, Linux race detector, browser, packaged stack and measured experiments. | Coverage is purposeful, not exhaustive formal verification. |
| Documentation | Commands, failure windows and measurements are distinguished from proposed scaling. | Hosting accounts still required; no verified public app URL. |

## Defects discovered and fixed during implementation

- HTTP ServeMux route patterns conflicted on startup; integration tests exposed it.
- JSON float conversion could lose large integer precision; canonicalization now
  uses `UseNumber`, with a regression test.
- The load utility referenced the wrong timer variable; all-command CI caught it.
- Vitest initially collected Playwright tests; its include scope now targets units.
- A test-runner dependency advisory was resolved; npm audit returned zero findings.
- A browser selector relied on an ambiguous wrapping label; it now selects by role.
- Linux mobile fonts exposed table/header overflow; scroll containment and shrinkable
  heading text preserve the viewport without hiding the page's overflow.
- Test cleanup originally ran after the pool closed; it now executes before closure
  and reports cleanup errors.
- Configuration tests inherited local insecure-mode variables; tests now explicitly
  exercise secure defaults independent of the developer environment.

## Repository hygiene

The source scan found no unfinished TODO/FIXME items, sample `example.com` URLs,
debug `console.log` statements or private-key/token patterns. HTML `placeholder`
attributes are legitimate input hints, not unfinished features. Generated binaries,
dependencies, local credentials and test artifacts are ignored. The personal
handbook and cheat sheet are ignored in Git and Docker context. Earlier handbook
versions remain in historical commits; no history rewrite or falsified dates was done.

Kubernetes manifests include resources, probes, non-root security settings and a
secret reference. They have not been applied to a Kubernetes cluster. The actual
container validation uses Docker Compose in GitHub Actions. See
[verification](verification.md) for precise results and links.
