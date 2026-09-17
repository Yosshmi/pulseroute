#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
: "${API_KEY:?Export an ingestion API_KEY}"
go run ./cmd/loadtest "$@"
