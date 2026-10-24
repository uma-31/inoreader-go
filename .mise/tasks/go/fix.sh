#!/usr/bin/env bash
#MISE description="Format the Go code and apply lint fixes"
set -euo pipefail

# shellcheck source=./lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

function main() {
  cd "$(git rev-parse --show-toplevel)"

  local module
  while read -r module; do
    echo "fixing the module in $module..."
    (
      cd "$module"
      golangci-lint fmt
      golangci-lint run --fix
      go mod tidy
    )
  done < <(ls_modules)
}

main "$@"
