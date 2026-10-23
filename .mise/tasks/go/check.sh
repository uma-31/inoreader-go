#!/usr/bin/env bash
#MISE description="Verify formatting, lint and vet of the Go code without changing any file"
set -euo pipefail

function main() {
  cd "$(git rev-parse --show-toplevel)"

  echo "running golangci-lint fmt --diff..."
  golangci-lint fmt --diff

  echo "running golangci-lint run..."
  golangci-lint run

  echo "verifying go.mod and go.sum are tidy..."
  go mod tidy -diff
}

main "$@"
