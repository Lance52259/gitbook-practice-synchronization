#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"
MODE="${1:-run}"
shift || true
go build -o bin/gitbook-practice-synchronization ./cmd/gitbook-practice-synchronization
exec ./bin/gitbook-practice-synchronization "${MODE}" "$@"
