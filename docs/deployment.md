# Free deployment decision

Provider documentation checked on 2026-09-17. No cloud resources were created and
no money was spent. The GitHub repository is public; a public application URL is
not claimed until its register → ingest → delivery workflow is verified.

| Option checked | Relevant constraint | Decision |
|---|---|---|
| [Render Free](https://render.com/docs/free) | Free web service sleeps after 15 minutes idle; free PostgreSQL expires after 30 days. Free background worker is not offered. | Use one web service running the combined demo command; store durable data externally. |
| [Koyeb Free](https://www.koyeb.com/docs/reference/instances) | One free instance, no worker service; sleeps after one idle hour. | Viable alternative, not selected. |
| [Neon Free](https://neon.com/pricing) | Free PostgreSQL with usage and storage limits; account required. | Select the Free plan only, use TLS connection string. Confirm current allowances in console. |
| [Upstash Free](https://upstash.com/pricing/redis) | 256 MB, 500,000 commands/month at the time checked. | Use Free Redis with TLS; never enable pay-as-you-go. |

These are demo tiers, not an always-on delivery guarantee. While compute sleeps,
jobs remain in PostgreSQL and will resume after a request wakes the service.
Polling every five seconds reduces idle database traffic but can still consume
free database compute allowances. Readiness traffic also consumes Redis commands.
Keep the demo small, monitor quotas, and accept suspension rather than upgrading.
Render can charge overages if a payment method is attached; use an account without
a payment method for this zero-spend deployment. If signup requires a card or the
console does not offer the stated Free plan, stop and keep the local demo.

## Minimal manual deployment

1. Sign in to Neon and create a **Free** PostgreSQL project. Copy its pooled TLS
   connection string as `DATABASE_URL`. Do not select a paid plan.
2. Sign in to Upstash and create a **Free** Redis database. Copy the **Redis protocol**
   TLS URL (`rediss://...`), not the HTTP REST URL, as `REDIS_URL`.
3. Generate an encryption key locally: `openssl rand -hex 32`. Keep it in the
   provider's secret settings. Changing this key without re-encrypting stored
   endpoint secrets makes old destinations undeliverable.
4. In Render, create a **Free Web Service** from
   `https://github.com/Yosshmi/pulseroute`. Choose Docker, Dockerfile
   `deployments/docker/Dockerfile`, command `/app/bin/demo`; or use `render.yaml`.
   Enter the three secrets above. Preserve `COOKIE_SECURE=true`,
   `ALLOW_PRIVATE_ENDPOINTS=false`, `WORKERS=2`, `POLL_INTERVAL=5s`.
5. After the deploy completes, open its HTTPS URL, register, create a project and
   an HTTPS endpoint you control, create an API key, and submit the README example.
   Check `/ready` and verify the delivery's recorded 2xx attempt in the dashboard.

The combined command applies migrations under an advisory transaction lock, serves
the built dashboard, and runs the same worker implementation as the independent
worker process. It is a cost-driven demo topology, not a third delivery design.
Do not use self-pings or artificial traffic to defeat provider idle limits.

For a public retry demonstration, deploy the included receiver as another **Free**
web service using the same Dockerfile, command `/app/bin/receiver`, and a generated
`RECEIVER_SECRET`. Set the identical secret on its endpoint in PulseRoute. Receiver
sleep can itself cause timeouts; that is a real demonstration of the retry path.
The receiver responds to `/health` without a signature and verifies webhook POSTs.

GitHub Actions performs CI and container smoke tests. Render's Git integration can
redeploy `main` after connection, but no live CD success is claimed here. Prefer
deploying only commits whose quality workflow has passed. Kubernetes manifests are
provided separately; they are not the selected free demo environment.
