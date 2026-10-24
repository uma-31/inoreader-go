#!/usr/bin/env bash
#MISE description="Verify formatting, lint and vet of the Go code without changing any file"
set -euo pipefail

# shellcheck source=./lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

function main() {
  cd "$(git rev-parse --show-toplevel)"

  local module
  while read -r module; do
    echo "checking the module in $module..."
    (
      cd "$module"
      golangci-lint fmt --diff
      golangci-lint run
      go mod tidy -diff
    )
  done < <(ls_modules)
}

main "$@"
