#!/usr/bin/env bash
#MISE description="Start the mock Inoreader API server for manual checks"
#USAGE flag "--addr <addr>" default="127.0.0.1:4010" help="Address to listen on"
set -euo pipefail

function main() {
  local root
  root="$(git rev-parse --show-toplevel)"

  go -C "$root/e2e" run ./cmd/mock --addr "${usage_addr:?}" --spec "$root/api/openapi.yaml"
}

main "$@"
