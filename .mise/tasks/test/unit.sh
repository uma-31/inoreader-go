#!/usr/bin/env bash
#MISE description="Run the unit tests"
#USAGE arg "[go_test_args]" var=#true help="Extra arguments for go test, such as -run"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"
  go test -race "$@" ./...
}

main "$@"
