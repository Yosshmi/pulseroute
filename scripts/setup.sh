#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ -e .env ]]; then echo '.env already exists; leaving it unchanged.'; exit 0; fi
command -v openssl >/dev/null || { echo 'Install openssl to generate local secrets.' >&2; exit 1; }
umask 077
password="$(openssl rand -hex 24)"
cat > .env <<EOF
POSTGRES_PASSWORD=$password
DATABASE_URL=postgres://pulseroute:$password@localhost:5432/pulseroute?sslmode=disable
REDIS_URL=redis://localhost:6379/0
ENCRYPTION_KEY=$(openssl rand -hex 32)
RECEIVER_SECRET=$(openssl rand -hex 32)
COOKIE_SECURE=false
ALLOW_PRIVATE_ENDPOINTS=true
WORKERS=5
EOF
echo 'Created .env with generated local credentials. Run docker compose up --build -d.'
