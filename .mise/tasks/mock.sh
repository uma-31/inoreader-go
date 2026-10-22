#!/usr/bin/env bash
#MISE description="Start the mock Inoreader API server for E2E tests"
#USAGE flag "--host <host>" default="127.0.0.1" help="Address to listen on"
#USAGE flag "--port <port>" default="4010" help="Port to listen on"
set -euo pipefail

function main() {
  local root
  root="$(git rev-parse --show-toplevel)"

  exec prism mock \
    --host "${usage_host:?}" \
    --port "${usage_port:?}" \
    "$root/api/openapi.yaml"
}

main "$@"
