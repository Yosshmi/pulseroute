# Kubernetes manifests provided; no live Kubernetes deployment claimed

Use an existing cluster only. These commands can create workloads; do not provision
a paid cluster for this demo. PostgreSQL and Redis are external dependencies.

1. Build the image and load it into your local cluster, or replace `pulseroute:local`
   in the three workload definitions with your registry image and immutable digest.
2. Create the secret from an untracked file containing `DATABASE_URL`, `REDIS_URL`,
   and a 64-character random hex `ENCRYPTION_KEY`:

   `kubectl create secret generic pulseroute-secrets --from-env-file=.env.kubernetes`

3. `kubectl apply -f deployments/kubernetes/config.yaml`
4. `kubectl apply -f deployments/kubernetes/migrate.yaml`
5. `kubectl wait --for=condition=complete job/pulseroute-migrate --timeout=120s`
6. `kubectl apply -f deployments/kubernetes/workloads.yaml`
7. Terminate HTTPS at your existing ingress and route to `pulseroute-api:80`.

The API image includes the dashboard. Use external TLS for PostgreSQL and Redis.
Never commit the secret file; base64 in a Kubernetes Secret is not encryption.
Configure cluster secret encryption, access controls, backups and egress policy
before any real production use. Liveness only checks the process; readiness checks
dependencies so a database outage does not cause a restart storm. API and workers
may scale separately; each process has a 20-connection database pool.
