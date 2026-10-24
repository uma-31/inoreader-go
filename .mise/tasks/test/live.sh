#!/usr/bin/env bash
#MISE description="Run the live tests against the real Inoreader API"
#USAGE arg "[go_test_args]" var=#true help="Extra arguments for go test, such as -run"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"
  # -count=1 because a cached pass says nothing about the API today.
  # Only ./live: the tests of live/authorize need no API and run in test:unit.
  go test -tags live -count=1 -v "$@" ./live
}

main "$@"
