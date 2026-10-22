#!/usr/bin/env bash
#MISE description="Lint the OpenAPI description of the Inoreader API"
set -euo pipefail

function main() {
  local root
  root="$(git rev-parse --show-toplevel)"

  cd "$root"
  redocly lint api/openapi.yaml
}

main "$@"
