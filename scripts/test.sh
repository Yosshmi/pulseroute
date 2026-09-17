#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test -z "$(gofmt -l cmd internal migrations tests)"
go vet ./cmd/... ./internal/... ./migrations/... ./tests/...
go test -race -count=1 ./cmd/... ./internal/... ./migrations/... ./tests/...
npm --prefix frontend ci
npm --prefix frontend run lint
npm --prefix frontend test
npm --prefix frontend run build
