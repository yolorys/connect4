#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
compose=(docker compose -p connect4-tests -f compose.test.yaml)
cleanup() { "${compose[@]}" down --volumes; }
trap cleanup EXIT
"${compose[@]}" up --build --abort-on-container-exit --exit-code-from tests
