#!/usr/bin/env bash
#MISE description="Format the Go code and apply lint fixes"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"

  echo "running golangci-lint fmt..."
  golangci-lint fmt

  echo "running golangci-lint run --fix..."
  golangci-lint run --fix

  echo "running go mod tidy..."
  go mod tidy
}

main "$@"
