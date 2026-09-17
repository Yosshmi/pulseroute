#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
bash scripts/setup.sh
docker compose up --build -d --wait
echo 'Dashboard: http://localhost:3000'
