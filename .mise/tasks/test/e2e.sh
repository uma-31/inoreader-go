#!/usr/bin/env bash
#MISE description="Run the E2E tests against a mock server started for them"
#USAGE flag "--port <port>" default="4011" help="Port of the mock server"
#USAGE arg "[go_test_args]" var=#true help="Extra arguments for go test, such as -run"
set -euo pipefail

prism_pid=""

function stop_mock() {
  if [ -n "$prism_pid" ]; then
    kill "$prism_pid" 2>/dev/null || true
    wait "$prism_pid" 2>/dev/null || true
  fi
}

function wait_for_mock() {
  local url=$1
  local i
  for i in $(seq 1 60); do
    if curl --silent --output /dev/null "$url"; then
      return 0
    fi
    if ! kill -0 "$prism_pid" 2>/dev/null; then
      echo "the mock server exited; see $log" >&2
      return 1
    fi
    sleep 0.5
  done
  echo "the mock server did not start in 30s; see $log" >&2
  return 1
}

function main() {
  local root
  root="$(git rev-parse --show-toplevel)"
  cd "$root"

  local port=${usage_port:?}
  local url="http://127.0.0.1:$port"
  log="$(mktemp -t prism-XXXXXX.log)"

  trap stop_mock EXIT

  echo "starting the mock server at $url (log: $log)..."
  # --errors also rejects responses that violate the description, which
  # catches examples that drifted from their schemas.
  prism mock --errors --host 127.0.0.1 --port "$port" api/openapi.yaml >"$log" 2>&1 &
  prism_pid=$!
  wait_for_mock "$url"

  echo "running the E2E tests..."
  INOREADER_E2E_BASE_URL="$url" go test -tags e2e -count=1 "$@" ./e2e/...
}

main "$@"
