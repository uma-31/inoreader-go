#!/usr/bin/env bash
#MISE description="Run the E2E tests against the mock server, which they start themselves"
#USAGE arg "[go_test_args]" var=#true help="Extra arguments for go test, such as -run"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)/e2e"
  go test -race "$@" ./...
}

main "$@"
